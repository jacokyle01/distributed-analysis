package primaryserver

import (
	"log"
	"src/models"
)

// SubmitResult stores a completed analysis result
func (s *Server) SubmitResult(result models.Result) {
	s.mu.Lock()
	s.results_store[result.JobID] = result
	delete(s.jobMap, result.JobID)
	s.mu.Unlock()

	log.Printf("Received result for job %s: %s (eval: %d)",
		result.JobID, result.BestMove, result.Eval)
}

// GetResult retrieves a result by job ID
func (s *Server) GetResult(jobID string) (models.Result, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result, exists := s.results_store[jobID]
	return result, exists
}
