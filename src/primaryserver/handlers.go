package primaryserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"src/models"
	"time"
	"math"
		"regexp"
	"sort"
	"strconv"
	// "log"

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

func (s *Server) handleGetBatch(w http.ResponseWriter, r *http.Request) {
	batchID := r.URL.Query().Get("id")
	if batchID == "" {
		http.Error(w, "missing id", 400)
		return
	}

	s.mu.RLock()
	batch, ok := s.batches[batchID]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "batch not found", 404)
		return
	}

	progress := float64(batch.Completed) / float64(batch.Total)

	response := map[string]interface{}{
		"batch_id":  batchID,
		"completed": batch.Completed,
		"total":     batch.Total,
		"progress":  fmt.Sprintf("%.2f%%", progress*100),
		"results":   batch.Results,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

	fmt.Println("=== Parsed Game ===")
	fmt.Println(game.String())

	moves := game.Moves()
	fmt.Println("=== Moves ===")
	for i, m := range moves {
		fmt.Printf("Move %d: %s\n", i, m)
	}

	positions := game.Positions()
	fmt.Println("=== Positions (FEN) ===")
	for i, pos := range positions {
		fmt.Printf("Before move %d → %s\n", i, pos.String())
	}

	// create batch entry
	batchID := fmt.Sprintf("batch_%d", time.Now().UnixNano())

	batch := &models.Batch{
		ID:        batchID,
		JobIDs:    []string{},
		Results:   make(map[string]models.Result),
		Completed: 0,
		Total:     len(moves),
	}

	s.mu.Lock()
	s.batches[batchID] = batch
	s.mu.Unlock()

	for i := 0; i < len(moves); i++ {
		fen := positions[i].String()

		jobID := fmt.Sprintf("%s_move_%d", batchID, i)
		job := models.Job{
			ID:       jobID,
			FEN:      fen,
			Depth:    25,
			TimeMS:   5000,
			Priority: 0,
		}

		// Add job ID to batch
		s.mu.Lock()
		batch.JobIDs = append(batch.JobIDs, jobID)
		s.mu.Unlock()

		// Add to queue
		s.AddJob(job)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"batch_id":   batchID,
		"move_count": len(moves),
	})
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func accuracyFromAvgLoss(avgLoss float64) float64 {
	const scale = 80.0
	return 100.0 * math.Exp(-avgLoss/scale)
}

// GET /batch/accuracy?batch_id=...
func (s *Server) handleBatchAccuracy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	batchID := r.URL.Query().Get("batch_id")
	if batchID == "" {
		http.Error(w, "missing batch_id", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	batch, ok := s.batches[batchID]
	s.mu.RUnlock()
	if !ok {
		http.Error(w, "batch not found", http.StatusNotFound)
		return
	}

	// Ensure completed
	if batch.Completed < batch.Total {
		http.Error(w, "batch not completed yet", http.StatusConflict)
		return
	}

	// Extract move index from job IDs like: batch_xxx_move_5
	re := regexp.MustCompile(`_move_(\d+)$`)

	type pair struct {
		i   int
		raw int // raw eval from your stored result (side-to-move perspective)
	}

	var evals []pair
	s.mu.RLock()
	for jobID, res := range batch.Results {
		m := re.FindStringSubmatch(jobID)
		if len(m) != 2 {
			continue
		}
		idx, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		evals = append(evals, pair{i: idx, raw: res.Eval})
	}
	s.mu.RUnlock()

	if len(evals) < 2 {
		http.Error(w, "not enough eval data to compute accuracy", http.StatusUnprocessableEntity)
		return
	}

	// Sort by move index
	sort.Slice(evals, func(a, b int) bool { return evals[a].i < evals[b].i })

	// flip black evals 
	E := make(map[int]float64, len(evals))
	for _, p := range evals {
		raw := float64(p.raw)
		if p.i%2 == 0 {
			E[p.i] = raw       
		} else {
			E[p.i] = -raw      
		}
	}

	var whiteLossSum float64
	var blackLossSum float64
	var whiteMoves int
	var blackMoves int

	for i := 0; i < batch.Total-1; i++ {
		eBefore, ok1 := E[i]
		eAfter, ok2 := E[i+1]
		if !ok1 || !ok2 {
			continue
		}

		if i%2 == 0 {
			loss := math.Max(0, eBefore-eAfter)
			whiteLossSum += loss
			whiteMoves++
		} else {
			loss := math.Max(0, eAfter-eBefore)
			blackLossSum += loss
			blackMoves++
		}
	}

	if whiteMoves == 0 && blackMoves == 0 {
		http.Error(w, "could not compute accuracy from available eval pairs", http.StatusUnprocessableEntity)
		return
	}

	var whiteAvgLoss float64
	var blackAvgLoss float64
	if whiteMoves > 0 {
		whiteAvgLoss = whiteLossSum / float64(whiteMoves)
	}
	if blackMoves > 0 {
		blackAvgLoss = blackLossSum / float64(blackMoves)
	}

	resp := map[string]any{
		"batch_id":        batchID,
		"white_accuracy":  accuracyFromAvgLoss(whiteAvgLoss),
		"black_accuracy":  accuracyFromAvgLoss(blackAvgLoss),
		"white_avg_cpl":   whiteAvgLoss, // centipawn loss
		"black_avg_cpl":   blackAvgLoss,
		"white_moves":     whiteMoves,
		"black_moves":     blackMoves,
		"note":            "Eval normalized to White perspective; CPL uses eval change between consecutive plies.",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, fmt.Sprintf("encode error: %v", err), http.StatusInternalServerError)
		return
	}
}

