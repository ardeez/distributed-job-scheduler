# Herschel — Distributed Job Scheduler

> A production-grade distributed job scheduler built in Go, designed for reliability, fault tolerance, and horizontal scalability.

Herschel schedules and executes recurring jobs across a cluster of worker nodes with automatic failover, retry mechanisms, and dead letter queue support. Named after [Caroline Herschel](https://en.wikipedia.org/wiki/Caroline_Herschel), the astronomer who cataloged and scheduled the stars.

---

## Why This Project?

Most cron-based schedulers are single-node — if the node dies, all scheduled jobs stop. Herschel solves this by distributing the scheduling responsibility across multiple nodes with leader election, so there's **no single point of failure**.

### Problems Herschel Solves

- **Single point of failure** — Traditional cron runs on one machine. If it crashes, jobs stop silently. Herschel runs multiple scheduler nodes; if the leader dies, a new one takes over within seconds.
- **No retry or error handling** — Cron doesn't know if a job failed. Herschel tracks job state, retries with exponential backoff, and moves permanently failed jobs to a dead letter queue for investigation.
- **No visibility** — Cron gives you nothing beyond log files. Herschel exposes Prometheus metrics, structured logs, and health endpoints for full observability.
- **Manual scaling** — Adding capacity to cron means SSH-ing into a new server. Herschel workers register automatically and jobs are distributed across available capacity.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      Client (CLI / API)                      │
│              Submit jobs, query status, manage DLQ            │
└──────────────────────────┬──────────────────────────────────┘
                           │ gRPC
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                    Scheduler Cluster                         │
│                                                              │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐                │
│  │ Leader   │   │ Follower │   │ Follower │                 │
│  │ (active) │   │(standby) │   │(standby) │                │
│  └────┬─────┘   └──────────┘   └──────────┘                │
│       │              ▲              ▲                         │
│       │         etcd leader election                         │
│       │                                                      │
│       │  ┌─────────────────┐  ┌──────────────┐              │
│       ├──│ Cron Engine     │  │ Job Dispatcher│              │
│       │  │ Parse & schedule│  │ Assign to     │              │
│       │  │ next run times  │  │ workers       │              │
│       │  └─────────────────┘  └──────┬───────┘              │
│       │                              │                       │
└───────┼──────────────────────────────┼──────────────────────┘
        │                              │
        ▼                              ▼ gRPC streaming
┌──────────────┐            ┌─────────────────────────────────┐
│  PostgreSQL  │            │         Worker Pool              │
│              │            │                                   │
│ • Jobs       │            │  ┌────────┐ ┌────────┐ ┌──────┐ │
│ • Executions │            │  │Worker 1│ │Worker 2│ │  ... │ │
│ • Audit log  │            │  └───┬────┘ └───┬────┘ └──┬───┘ │
│              │            │      │          │         │      │
└──────────────┘            │   Execute    Execute   Execute   │
                            │   (goroutine + timeout + retry)  │
┌──────────────┐            └─────────────────────────────────┘
│    Redis     │                       │
│              │◄──────────────────────┘
│ • Job queue  │            report results
│   (Streams)  │
│ • DLQ        │
│ • Pub/Sub    │
└──────────────┘
```

---

## How It Works

### 1. Job Submission

A client submits a job definition via gRPC (or CLI). The job includes a cron expression, payload, timeout, and retry policy. The scheduler persists it to PostgreSQL and calculates the next run time.

```
Client → gRPC → Scheduler → PostgreSQL (persist job)
                          → Redis Stream (enqueue next run)
```

### 2. Scheduling Loop

The **leader node** runs a scheduling loop that ticks every second. It checks for jobs whose `next_run_at` has passed, transitions them to `DISPATCHED` state, and pushes them to the Redis Streams job queue.

Only the leader runs this loop. Followers are on hot standby — if the leader fails, etcd triggers a new election and the new leader picks up the loop immediately.

### 3. Job Dispatch & Execution

Workers consume from the Redis Stream using consumer groups (`XREADGROUP`). Each worker:

1. Claims a job from the stream
2. Executes it in a dedicated goroutine with `context.WithTimeout`
3. Reports success/failure back via gRPC
4. Acknowledges the message (`XACK`)

Concurrency per worker is controlled by a **semaphore** (buffered channel), preventing resource exhaustion.

### 4. Retry & Dead Letter Queue

When a job fails:

```
FAILED → check retry count
       → if retries < MaxRetries: exponential backoff → re-enqueue
       → if retries >= MaxRetries: move to Dead Letter Queue
```

Backoff formula: `wait = baseDelay * 2^attempt` (1s → 2s → 4s → 8s ...)

The DLQ is a separate Redis Stream (`dlq:jobs`). Failed jobs sit there for manual inspection and can be retried via CLI.

### 5. Leader Election & Failover

Herschel uses **etcd** for distributed consensus:

- On startup, each scheduler node campaigns to become leader
- The winner runs the scheduling loop; losers enter standby
- Each leader maintains a **lease** with etcd (heartbeat)
- If the leader crashes, the lease expires and etcd triggers a new election
- A **fencing token** (monotonically increasing) prevents stale leaders from dispatching jobs

Failover target: **< 10 seconds** from leader death to new leader active.

### 6. Worker Health Monitoring

Workers send periodic heartbeats to the scheduler via gRPC server streaming. If a worker misses **3 consecutive heartbeats**:

1. The scheduler marks the worker as `DEAD`
2. Any in-flight jobs assigned to that worker are reclaimed
3. Reclaimed jobs re-enter the dispatch queue

---

## Job State Machine

```
  ┌─────────┐
  │ PENDING │  ← job submitted, waiting for next cron tick
  └────┬────┘
       │ cron tick matches
       ▼
  ┌──────────┐
  │SCHEDULED │  ← next run time reached, queued for dispatch
  └────┬─────┘
       │ pushed to Redis Stream
       ▼
 ┌────────────┐
 │ DISPATCHED │  ← assigned to a worker, waiting for pickup
 └─────┬──────┘
       │ worker claims job
       ▼
  ┌─────────┐
  │ RUNNING │  ← worker is executing
  └────┬────┘
       │
  ┌────┴─────┐
  ▼          ▼
┌─────────┐ ┌────────┐
│COMPLETED│ │ FAILED │
└─────────┘ └───┬────┘
                │
         ┌──────┴──────┐
         ▼             ▼
    ┌─────────┐   ┌────────┐
    │  RETRY  │   │  DEAD  │  ← moved to DLQ after max retries
    └────┬────┘   └────────┘
         │
         ▼
   (back to DISPATCHED)
```

---

## Project Structure

```
distributed-job-scheduler/
├── cmd/
│   ├── scheduler/         # Scheduler node entry point
│   │   └── main.go
│   ├── worker/            # Worker node entry point
│   │   └── main.go
│   └── cli/               # CLI client for job management
│       └── main.go
├── internal/
│   ├── scheduler/         # Scheduling engine (cron, dispatch loop)
│   ├── worker/            # Job execution, health reporting
│   ├── leader/            # etcd-based leader election
│   ├── store/             # PostgreSQL repository (GORM)
│   ├── broker/            # Redis Streams abstraction
│   ├── transport/         # gRPC server & client implementations
│   ├── retry/             # Retry logic, backoff, DLQ
│   └── model/             # Shared domain models
├── proto/                 # Protobuf service definitions
│   ├── job.proto
│   ├── worker.proto
│   └── scheduler.proto
├── migrations/            # SQL migration files
├── deployments/
│   ├── docker/            # Dockerfiles, docker-compose.yml
│   └── terraform/         # AWS infrastructure as code
├── .github/
│   └── workflows/
│       ├── ci.yml         # Lint, test, build on every push
│       └── cd.yml         # Build, push, deploy on tag
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

---

## Tech Stack

| Component | Technology | Why |
|-----------|-----------|-----|
| Language | **Go** | First-class concurrency (goroutines, channels), fast compilation, single binary deployment |
| Inter-service communication | **gRPC** | Bi-directional streaming for heartbeats & job dispatch, strong typing via protobuf |
| Job queue | **Redis Streams** | Built-in consumer groups for competing consumers, pending entry tracking, lightweight |
| Data store | **PostgreSQL** | ACID transactions for job state, audit trail, familiar with GORM |
| Consensus | **etcd** | Industry-standard leader election, battle-tested (used by Kubernetes) |
| Containers | **Docker** | Reproducible builds, multi-node local development via Compose |
| CI/CD | **GitHub Actions** | Automated lint → test → build → deploy pipeline |
| Infrastructure | **Terraform** | Declarative AWS provisioning (VPC, ECS, RDS, ElastiCache) |
| Deployment | **AWS ECS Fargate** | Serverless containers, no EC2 management, built-in scaling |
| Monitoring | **Prometheus + Grafana** | Metrics collection, dashboards, alerting |
| Logging | **slog** (stdlib) | Structured JSON logging, zero dependencies |

---

## Getting Started

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- `protoc` compiler with Go plugins
- Make

### Local Development

```bash
# Clone the repository
git clone https://github.com/ardista-sk/distributed-job-scheduler.git
cd distributed-job-scheduler

# Start infrastructure (PostgreSQL, Redis, etcd)
make docker-up

# Generate protobuf code
make proto-gen

# Run database migrations
make migrate

# Start a scheduler node
make run-scheduler

# In another terminal, start a worker node
make run-worker

# Submit a test job via CLI
make run-cli -- submit \
  --name "test-job" \
  --cron "*/5 * * * *" \
  --payload '{"task": "hello-world"}'
```

### Multi-node Setup (Docker Compose)

```bash
# Start full cluster: 3 schedulers + 3 workers + infra + monitoring
docker compose -f deployments/docker/docker-compose.yml up

# View Grafana dashboard
open http://localhost:3000

# Submit a job
docker compose exec cli ./cli submit --name "test" --cron "* * * * *"

# Simulate leader failure
docker compose stop scheduler-1

# Watch logs — a new leader should be elected within 10 seconds
docker compose logs -f scheduler-2 scheduler-3
```

---

## API Overview

### gRPC Services

**JobService** — Job lifecycle management
| RPC | Description |
|-----|-------------|
| `SubmitJob` | Create a new scheduled job |
| `CancelJob` | Cancel a pending/scheduled job |
| `GetJobStatus` | Get current status and execution history |
| `ListJobs` | List all jobs with filtering and pagination |
| `RetryDeadJob` | Re-enqueue a job from the DLQ |
| `ListDeadJobs` | List all jobs in the Dead Letter Queue |

**WorkerService** — Worker ↔ Scheduler communication
| RPC | Description |
|-----|-------------|
| `RegisterWorker` | Register a new worker node |
| `Heartbeat` | Server streaming — periodic health check |
| `DispatchJob` | Stream jobs to workers for execution |
| `ReportResult` | Worker reports job completion/failure |

---

## Monitoring

### Prometheus Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `herschel_jobs_scheduled_total` | Counter | Total jobs scheduled |
| `herschel_jobs_completed_total` | Counter | Completed jobs (by status label) |
| `herschel_jobs_in_flight` | Gauge | Currently executing jobs |
| `herschel_job_duration_seconds` | Histogram | Job execution duration |
| `herschel_worker_count` | Gauge | Active registered workers |
| `herschel_leader_elections_total` | Counter | Leader election events |
| `herschel_dlq_size` | Gauge | Jobs in Dead Letter Queue |
| `herschel_heartbeat_latency_seconds` | Histogram | Worker heartbeat round-trip |

### Health Endpoints

| Endpoint | Purpose |
|----------|---------|
| `/healthz` | Liveness — is the process alive? |
| `/readyz` | Readiness — can accept traffic? |

---

## Deployment

### AWS Architecture

```
                    ┌─────────────┐
                    │   Route 53  │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │     ALB     │  (public subnet)
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │      Private Subnets     │
              │                          │
              │  ┌────────────────────┐  │
              │  │   ECS Fargate      │  │
              │  │                    │  │
              │  │ Scheduler (×3)    │  │
              │  │ Worker (auto-scale)│  │
              │  └────────────────────┘  │
              │           │  │           │
              │  ┌────────┘  └────────┐  │
              │  ▼                    ▼  │
              │ ┌──────┐      ┌───────┐ │
              │ │ RDS  │      │ElastiC│ │
              │ │(PgSQL)│      │(Redis)│ │
              │ └──────┘      └───────┘ │
              └──────────────────────────┘
```

### Deploy Commands

```bash
# Provision infrastructure
cd deployments/terraform/environments/staging
terraform init
terraform apply

# Deploy via CI/CD (recommended)
git tag v1.0.0
git push origin v1.0.0
# → GitHub Actions: build → push to ECR → deploy to ECS

# Manual deploy (if needed)
make deploy-staging
```

---

## Future Features

Features planned for future releases, roughly in priority order:

### v1.1 — Enhanced Scheduling
- **Job dependencies** — DAG-based execution (Job B runs only after Job A completes)
- **Priority queues** — High-priority jobs skip ahead in the queue
- **Rate limiting** — Throttle job execution per tenant or job type
- **Delayed jobs** — One-time jobs that execute at a specific future time (not cron-based)

### v1.2 — Observability & Debugging
- **OpenTelemetry tracing** — Distributed traces across scheduler → worker → job execution
- **Web UI dashboard** — Real-time view of jobs, workers, and cluster health (React frontend)
- **Job execution logs** — Capture stdout/stderr from job runs, viewable via API
- **Alerting rules** — Pre-configured Grafana alerts for common failure patterns

### v1.3 — Multi-tenancy & Security
- **Namespace isolation** — Separate job queues and workers per tenant/team
- **RBAC** — Role-based access control for job submission and management
- **mTLS** — Mutual TLS between scheduler and workers
- **Secrets injection** — Securely pass credentials to jobs at runtime

### v1.4 — Advanced Scaling
- **Kubernetes deployment** — Helm chart + operator for K8s-native scheduling
- **Multi-region** — Cross-region scheduler replication for disaster recovery
- **Spot instance awareness** — Graceful job migration when Fargate Spot gets reclaimed
- **Auto-tuning** — Dynamic concurrency limits based on worker resource utilization

### v1.5 — Ecosystem
- **Plugin system** — Custom job executors (HTTP webhook, gRPC call, shell command, Docker container)
- **Event hooks** — Webhooks on job state transitions (for Slack notifications, etc.)
- **SDK** — Go and Python client libraries for programmatic job management
- **Terraform provider** — Manage job definitions as infrastructure code

---

## Contributing

This is currently a personal learning project. Feel free to open issues for bugs or suggestions.

---

## License

MIT