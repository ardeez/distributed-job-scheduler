package model

import "fmt"

func (j *Job) IsTerminal() bool {
	return j.Status == JobStatusCompleted || j.Status == JobStatusDead || j.Status == JobStatusCancelled
}

func (j *Job) CanRetry() bool {
	return j.RetryCount < j.MaxRetries
}

var validTransitions = map[JobStatus][]JobStatus{
	JobStatusPending:    {JobStatusScheduled, JobStatusCancelled},
	JobStatusScheduled:  {JobStatusDispatched, JobStatusFailed, JobStatusCancelled},
	JobStatusDispatched: {JobStatusRunning, JobStatusFailed},
	JobStatusRunning:    {JobStatusCompleted, JobStatusFailed},
	JobStatusRetry:      {JobStatusDispatched, JobStatusFailed},
	JobStatusFailed:     {JobStatusRetry, JobStatusDead},
	JobStatusCompleted:  {},
	JobStatusDead:       {},
	JobStatusCancelled:  {},
}

func (j *Job) TransitionTo(status JobStatus) error {
	for _, s := range validTransitions[j.Status] {
		if s == status {
			j.Status = status
			return nil
		}
	}
	return fmt.Errorf("invalid transition from %s to %s", j.Status, status)
}
