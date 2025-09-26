//TODO run fishnet locally
// TODO api /queue to monitor queue

//TODO how do we piece analyzed moves back together? 

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Job represents a chess position analysis job
// right now jobs are just a FEN, we can possibly make them larger, that is, parts of a PGN
// TODO
type Job struct {
	ID       string `json:"id"`
	FEN      string `json:"fen"`
	Depth    int    `json:"depth"`
	TimeMS   int    `json:"time_ms"`
	Priority int    `json:"priority"`
}

// Result represents the analysis result
type Result struct {
	JobID     string `json:"job_id"`
	BestMove  string `json:"best_move"`
	Eval      int    `json:"eval"` // centipawns
	Depth     int    `json:"depth"`
	Nodes     int64  `json:"nodes"`
	NodesPerS int64  `json:"nodes_per_s"`
	PV        string `json:"pv"` // principal variation
	Time      int    `json:"time_ms"`
	Error     string `json:"error,omitempty"`
}

// Server manages the job queue and distributes work
type Server struct {
	jobs          chan Job
	results       chan Result
	mu            sync.RWMutex
	jobMap        map[string]Job
	results_store map[string]Result
}

// NewServer creates a new analysis server
func NewServer() *Server {
	return &Server{
		jobs:          make(chan Job, 100),
		results:       make(chan Result, 100),
		jobMap:        make(map[string]Job),
		results_store: make(map[string]Result),
	}
}

// AddJob adds a new analysis job to the queue
func (s *Server) AddJob(job Job) {
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
func (s *Server) GetJob() (Job, bool) {
	select {
	case job := <-s.jobs:
		return job, true
	case <-time.After(5 * time.Second):
		return Job{}, false
	}
}

// SubmitResult stores a completed analysis result
func (s *Server) SubmitResult(result Result) {
	s.mu.Lock()
	s.results_store[result.JobID] = result
	delete(s.jobMap, result.JobID)
	s.mu.Unlock()

	log.Printf("Received result for job %s: %s (eval: %d)",
		result.JobID, result.BestMove, result.Eval)
}

// GetResult retrieves a result by job ID
func (s *Server) GetResult(jobID string) (Result, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result, exists := s.results_store[jobID]
	return result, exists
}

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

	var result Result
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

	var job Job
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

	pendingJobs := make([]Job, 0, len(s.jobMap))
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


//TODO at some point, add queue mechanism (redis?) in between submitting games and acquiring jobs
func (s *Server) requestForAnalysis(w http.ResponseWriter, r *http.Request) {
	/*
		steps
		1) deserialize game 
		2) parse game into moves
		2b) associate moves with some batchId (so when the moves get analyzed we have a place to record results)
		3) push moves (as work) to job queue 
	*/
}


// StartServer starts the HTTP server
func (s *Server) StartServer(addr string) {
	http.HandleFunc("/job", s.handleGetJob)
	http.HandleFunc("/result", s.handleSubmitResult)
	http.HandleFunc("/analyze", s.handleAnalyze)
	http.HandleFunc("/get_result", s.handleGetResult)
	http.HandleFunc("/queue", s.handleViewQueue)
	http.handleFunc("/requestForAnalysis", s.requestForAnalysis)



	log.Printf("Starting server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// ChessEngine wraps a UCI chess engine
type ChessEngine struct {
	cmd    *exec.Cmd
	stdin  *bufio.Writer
	stdout *bufio.Scanner
}

// NewChessEngine creates a new chess engine instance
func NewChessEngine(enginePath string) (*ChessEngine, error) {
	cmd := exec.Command(enginePath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	engine := &ChessEngine{
		cmd:    cmd,
		stdin:  bufio.NewWriter(stdin),
		stdout: bufio.NewScanner(stdout),
	}

	// Initialize UCI
	engine.sendCommand("uci")
	engine.waitForResponse("uciok")
	engine.sendCommand("isready")
	engine.waitForResponse("readyok")

	return engine, nil
}

func (e *ChessEngine) sendCommand(cmd string) {
	e.stdin.WriteString(cmd + "\n")
	e.stdin.Flush()
}

func (e *ChessEngine) waitForResponse(expected string) string {
	for e.stdout.Scan() {
		line := e.stdout.Text()
		if strings.Contains(line, expected) {
			return line
		}
	}
	return ""
}

func (e *ChessEngine) readUntilBestMove() []string {
	var lines []string
	for e.stdout.Scan() {
		line := e.stdout.Text()
		lines = append(lines, line)
		if strings.HasPrefix(line, "bestmove") {
			break
		}
	}
	return lines
}

// AnalyzePosition analyzes a chess position
func (e *ChessEngine) AnalyzePosition(fen string, depth int, timeMS int) (*Result, error) {
	// Set position
	e.sendCommand(fmt.Sprintf("position fen %s", fen))

	// Start analysis
	goCmd := fmt.Sprintf("go depth %d movetime %d", depth, timeMS)
	e.sendCommand(goCmd)

	// Read engine output
	lines := e.readUntilBestMove()

	result := &Result{
		Depth: depth,
		Time:  timeMS,
	}

	// Parse engine output
	for _, line := range lines {
		if strings.HasPrefix(line, "info") {
			parts := strings.Fields(line)
			for i, part := range parts {
				switch part {
				case "score":
					if i+2 < len(parts) && parts[i+1] == "cp" {
						fmt.Sscanf(parts[i+2], "%d", &result.Eval)
					}
				case "depth":
					if i+1 < len(parts) {
						fmt.Sscanf(parts[i+1], "%d", &result.Depth)
					}
				case "nodes":
					if i+1 < len(parts) {
						fmt.Sscanf(parts[i+1], "%d", &result.Nodes)
					}
				case "nps":
					if i+1 < len(parts) {
						fmt.Sscanf(parts[i+1], "%d", &result.NodesPerS)
					}
				case "pv":
					if i+1 < len(parts) {
						result.PV = strings.Join(parts[i+1:], " ")
					}
				}
			}
		} else if strings.HasPrefix(line, "bestmove") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				result.BestMove = parts[1]
			}
		}
	}

	return result, nil
}

func (e *ChessEngine) Close() {
	e.sendCommand("quit")
	e.cmd.Wait()
}

// Client represents a worker client
type Client struct {
	serverURL string
	engine    *ChessEngine
}

// NewClient creates a new worker client
func NewClient(serverURL, enginePath string) (*Client, error) {
	engine, err := NewChessEngine(enginePath)
	if err != nil {
		return nil, err
	}

	return &Client{
		serverURL: serverURL,
		engine:    engine,
	}, nil
}

// WorkLoop runs the main worker loop
func (c *Client) WorkLoop(ctx context.Context) {
	log.Printf("Starting worker, connecting to %s", c.serverURL)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			c.processJob()
		}
	}
}

// TODO: backoff algorithm + ensure healthy client
// queue.rs#500
func (c *Client) processJob() {
	// Get job from server
	resp, err := http.Get(c.serverURL + "/job")
	if err != nil {
		log.Printf("Error getting job: %v", err)
		time.Sleep(5 * time.Second)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		log.Printf("No jobs available, waiting...")
		time.Sleep(2 * time.Second)
		return
	}

	var job Job
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		log.Printf("Error decoding job: %v", err)
		return
	}

	log.Printf("Processing job %s: %s", job.ID, job.FEN)

	// Analyze position
	result, err := c.engine.AnalyzePosition(job.FEN, job.Depth, job.TimeMS)
	if err != nil {
		log.Printf("Error analyzing position: %v", err)
		result = &Result{Error: err.Error()}
	}

	result.JobID = job.ID

	// Submit result
	resultJSON, _ := json.Marshal(result)
	_, err = http.Post(c.serverURL+"/result", "application/json",
		bytes.NewBuffer(resultJSON))
	if err != nil {
		log.Printf("Error submitting result: %v", err)
	}
}

func (c *Client) Close() {
	c.engine.Close()
}

// Example usage and main function
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  go run main.go server [port]          - Run server")
		fmt.Println("  go run main.go client [server_url] [engine_path] - Run client")
		fmt.Println("  go run main.go example                - Run example")
		return
	}

	switch os.Args[1] {
	case "server":
		port := ":8080"
		if len(os.Args) > 2 {
			port = ":" + os.Args[2]
		}

		server := NewServer()
		server.StartServer(port)

	case "client":
		serverURL := "http://localhost:8080"
		enginePath := "../stockfish/stockfish-ubuntu-x86-64-avx512" // Assumes stockfish is in PATH

		if len(os.Args) > 2 {
			serverURL = os.Args[2]
		}
		if len(os.Args) > 3 {
			enginePath = os.Args[3]
		}

		client, err := NewClient(serverURL, enginePath)
		if err != nil {
			log.Fatal("Error creating client:", err)
		}
		defer client.Close()

		ctx := context.Background()
		client.WorkLoop(ctx)

	case "example":
		// Example of how to use the API
		fmt.Println("Starting example server...")

		// TODO better separation between client & server
		server := NewServer()
		go server.StartServer(":8080")

		// Wait for server to start
		time.Sleep(time.Second)

		// Example: Submit analysis job
		job := Job{
			ID:     "example_job",
			FEN:    "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			Depth:  10,
			TimeMS: 3000,
		}

		jobJSON, _ := json.Marshal(job)
		resp, err := http.Post("http://localhost:8080/analyze",
			"application/json", bytes.NewBuffer(jobJSON))
		if err != nil {
			log.Fatal("Error submitting job:", err)
		}

		var response map[string]string
		json.NewDecoder(resp.Body).Decode(&response)
		fmt.Printf("Submitted job, ID: %s\n", response["job_id"])

		fmt.Println("Now run a client to process the job:")
		fmt.Println("go run main.go client http://localhost:8080 /path/to/stockfish")

		// Keep server running
		select {}

	default:
		fmt.Println("Unknown command:", os.Args[1])
	}
}
