package model

import "time"

type JobExecution struct {
	ID           string
	JobID        string
	WorkerID     string
	Attempt      int
	Status       JobStatus
	ErrorMessage string
	StartedAt    time.Time
	FinishedAt   *time.Time
	CreatedAt    time.Time
}
