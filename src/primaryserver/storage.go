package primaryserver

import (
	"log"
	"src/models"
)

// SubmitResult stores a completed analysis result
func (s *Server) SubmitResult(result models.Result) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove from pending jobs
	delete(s.jobMap, result.JobID)

	// Find batch that contains this job
	for _, batch := range s.batches {
		for _, id := range batch.JobIDs {
			if id == result.JobID {
				// Add result to batch
				batch.Results[result.JobID] = result
				batch.Completed++

				log.Printf("[Batch %s] %d/%d jobs complete",
					batch.ID, batch.Completed, batch.Total)

				break
			}
		}
	}

	log.Printf("Received result for job %s: %s (eval: %d)",
		result.JobID, result.BestMove, result.Eval)
}

// // GetResult retrieves a result by job ID
// func (s *Server) GetResult(jobID string) (models.Result, bool) {
// 	s.mu.RLock()
// 	defer s.mu.RUnlock()
// 	result, exists := s.results_store[jobID]
// 	return result, exists
// }
