package model

import "testing"

func TestTransitionTo_Valid(t *testing.T) {
	tests := []struct {
		name string
		from JobStatus
		to   JobStatus
	}{
		{"pending to scheduled", JobStatusPending, JobStatusScheduled},
		{"scheduled to dispatched", JobStatusScheduled, JobStatusDispatched},
		{"dispatched to running", JobStatusDispatched, JobStatusRunning},
		{"running to completed", JobStatusRunning, JobStatusCompleted},
		{"running to failed", JobStatusRunning, JobStatusFailed},
		{"failed to retry", JobStatusFailed, JobStatusRetry},
		{"failed to dead", JobStatusFailed, JobStatusDead},
		{"retry to dispatched", JobStatusRetry, JobStatusDispatched},
		{"retry to failed", JobStatusRetry, JobStatusFailed},
		{"pending to cancelled", JobStatusPending, JobStatusCancelled},
		{"scheduled to cancelled", JobStatusScheduled, JobStatusCancelled},
		{"scheduled to failed", JobStatusScheduled, JobStatusFailed},
		{"dispatched to failed", JobStatusDispatched, JobStatusFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Job{Status: tt.from}
			err := job.TransitionTo(tt.to)
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestTransitionTo_Invalid(t *testing.T) {
	tests := []struct {
		name string
		from JobStatus
		to   JobStatus
	}{
		{name: "completed to pending", from: JobStatusCompleted, to: JobStatusPending},
		{name: "dead to retry", from: JobStatusDead, to: JobStatusRetry},
		{name: "cancelled to pending", from: JobStatusCancelled, to: JobStatusPending},
		{name: "pending to running", from: JobStatusPending, to: JobStatusRunning},
		{name: "running to dispatched", from: JobStatusRunning, to: JobStatusDispatched},
		{name: "pending to pending", from: JobStatusPending, to: JobStatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Job{Status: tt.from}
			err := job.TransitionTo(tt.to)
			if job.Status != tt.from {
				t.Errorf("status should remain %s,got %s", tt.from, job.Status)
			}
			if err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}

}

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		status   JobStatus
		expected bool
	}{
		{name: "completed is terminal", status: JobStatusCompleted, expected: true},
		{name: "dead is terminal", status: JobStatusDead, expected: true},
		{name: "cancelled is terminal", status: JobStatusCancelled, expected: true},
		{name: "pending is not terminal", status: JobStatusPending, expected: false},
		{name: "scheduled is not terminal", status: JobStatusScheduled, expected: false},
		{name: "dispatched is not terminal", status: JobStatusDispatched, expected: false},
		{name: "running is not terminal", status: JobStatusRunning, expected: false},
		{name: "retry is not terminal", status: JobStatusRetry, expected: false},
		{name: "failed is not terminal", status: JobStatusFailed, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Job{Status: tt.status}
			if job.IsTerminal() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, job.IsTerminal())
			}
		})
	}
}


func TestCanRetry(t *testing.T) {
	tests := []struct {
		name string
		maxRetries int
		retryCount int
		expected   bool
	}{
		{name: "retry count less than max retries", maxRetries: 3, retryCount: 2, expected: true},
		{name: "retry count equal to max retries", maxRetries: 3, retryCount: 3, expected: false},
		{name: "retry count greater than max retries", maxRetries: 3, retryCount: 4, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Job{MaxRetries: tt.maxRetries, RetryCount: tt.retryCount}
			if job.CanRetry() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, job.CanRetry())
			}
		})
	}
}