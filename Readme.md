# Distributed Job Scheduler

## Overview

Distributed Job Scheduler is a backend system designed to execute scheduled jobs based on time (cron-based), with a strong focus on **concurrency, reliability, and scalability**.

This project is not just about building a simple scheduler. It is an exploration of how real-world scheduling systems behave under constraints such as timing accuracy, failures, and distributed execution.

---

## Objectives

The main goals of this project:

* Build a scheduler capable of:

  * executing jobs based on time (cron expressions)
  * handling multiple jobs efficiently
  * avoiding duplicate executions
* Understand real-world engineering challenges:

  * race conditions
  * time-based scheduling accuracy
  * concurrency control
  * failure handling and retries

---

## Development Approach

This project follows a deliberate engineering approach:

* **Problem-driven development**
  Solutions are introduced only after encountering real limitations

* **Iterative design**
  Start simple, evolve based on observed issues

* **Test-Driven Development (TDD)**
  Applied to core domain logic such as scheduling and retry behavior

---

## Current Scope (Phase 1)

The current implementation is intentionally minimal:

* In-memory job scheduler
* Cron-based scheduling
* Single process execution
* Basic scheduling loop

> The purpose of this phase is to surface real problems before introducing complexity.

---

## Planned Architecture

```text
Client/API
   │
   ▼
PostgreSQL (source of truth)
   │
   ▼
Scheduler Service
   │
   ▼
Queue (Redis)
   │
   ▼
Worker Service
```

---

## Future Components

### Scheduler Service

* Determines which jobs are due
* Dispatches jobs to the queue

### Worker Service

* Consumes jobs from the queue
* Executes jobs
* Handles retries and failures

### Database (PostgreSQL)

* Stores job definitions
* Tracks execution state and history

### Queue (Redis)

* Decouples scheduler and worker
* Buffers job execution

---

## Key Challenges

This project intentionally explores:

* Time-based scheduling accuracy
* Distributed coordination
* Job deduplication
* Failure recovery and retry strategies
* Scaling execution across multiple workers

---

## Non-Goals

Out of scope for now:

* UI / frontend
* Multi-region deployment
* Advanced orchestration (e.g., Kubernetes)

---

## Tech Stack (Planned)

* Go — core system and concurrency
* PostgreSQL — persistence layer
* Redis — queue and coordination
* Docker — containerization

---

## Status

🚧 Early stage — basic in-memory scheduler

---

## Notes

This project starts with a simple implementation by design.

The goal is to:

* expose real system limitations
* understand failure modes
* evolve the architecture based on actual problems

Refactoring and improvements will be driven by observed issues, not assumptions.
