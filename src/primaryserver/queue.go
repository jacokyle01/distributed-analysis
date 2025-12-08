package primaryserver

import (
	"log"
	"src/models"
	"time"
)

// AddJob adds a new analysis job to the queue
func (s *Server) AddJob(job models.Job) {
	s.mu.Lock()
	s.jobMap[job.ID] = job
	s.mu.Unlock()

	select {
	case s.jobs <- job:
		log.Printf("Added job %s to queue", job.ID)
	default:
		log.Printf("Job queue full, dropping job %s", job.ID)
	}
}

// GetJob returns the next job for a worker
func (s *Server) GetJob() (models.Job, bool) {
	select {
	case job := <-s.jobs:
		return job, true
	case <-time.After(5 * time.Second):
		return models.Job{}, false
	}
}
