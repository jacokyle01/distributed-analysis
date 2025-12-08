package primaryserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"src/models"
	"time"

	"github.com/notnil/chess"
)

// HTTP handlers
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	job, ok := s.GetJob()
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (s *Server) handleSubmitResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var result models.Result
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	s.SubmitResult(result)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var job models.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if job.ID == "" {
		job.ID = fmt.Sprintf("job_%d", time.Now().UnixNano())
	}
	if job.Depth == 0 {
		job.Depth = 15
	}
	if job.TimeMS == 0 {
		job.TimeMS = 5000
	}

	s.AddJob(job)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"job_id": job.ID})
}

func (s *Server) handleGetResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		http.Error(w, "Missing job_id parameter", http.StatusBadRequest)
		return
	}

	result, exists := s.GetResult(jobID)
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleViewQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	pendingJobs := make([]models.Job, 0, len(s.jobMap))
	for _, job := range s.jobMap {
		pendingJobs = append(pendingJobs, job)
	}

	status := map[string]interface{}{
		"queue_length": len(s.jobMap),
		"pending_jobs": pendingJobs,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// TODO at some point, add queue mechanism (redis?) in between submitting games and acquiring jobs
func (s *Server) requestForAnalysis(w http.ResponseWriter, r *http.Request) {
	/*
		steps
		1) deserialize game
		2) parse game into moves
		2b) associate moves with some batchId (so when the moves get analyzed we have a place to record results)
		3) push moves (as work) to job queue
	*/

	// create request struct
	var req struct {
		Pgn string `json:"pgn"` // e.g. "e4 e5 Nf3 Nf6"
	}

	// deserialize request, fill request body
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), 400)
		return
	}

	// parse PGN (pgn --> game)

	game := chess.NewGame()
	err := game.UnmarshalText([]byte(req.Pgn))
	if err != nil {
		http.Error(w, "invalid PGN: "+err.Error(), 400)
		return
	}

	//moves := game.Moves()

}
