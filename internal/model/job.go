package model

import (
	"time"
)

type Job struct {
	ID             string
	Name           string
	CronExpr       string
	Payload        []byte
	MaxRetries     int
	TimeoutSeconds int
	RetryCount     int
	Status         JobStatus
	LastRunAt      *time.Time
	NextRunAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type JobStatus string

const (
	JobStatusPending    JobStatus = "PENDING"
	JobStatusScheduled  JobStatus = "SCHEDULED"
	JobStatusDispatched JobStatus = "DISPATCHED"
	JobStatusRunning    JobStatus = "RUNNING"
	JobStatusCompleted  JobStatus = "COMPLETED"
	JobStatusFailed     JobStatus = "FAILED"
	JobStatusRetry      JobStatus = "RETRY"
	JobStatusDead       JobStatus = "DEAD"
	JobStatusCancelled  JobStatus = "CANCELLED"
)
