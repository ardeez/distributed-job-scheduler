package model

import "time"

type Worker struct {
	ID                 string
	Hostname           string
	Status             WorkerStatus
	MaxConcurrentJobs  int
	CurrentRunningJobs int
	LastHeartbeat      time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type WorkerStatus string

const (
	WorkerStatusIdle     WorkerStatus = "IDLE"
	WorkerStatusBusy     WorkerStatus = "BUSY"
	WorkerStatusDraining WorkerStatus = "DRAINING"
	WorkerStatusDead     WorkerStatus = "DEAD"
)
