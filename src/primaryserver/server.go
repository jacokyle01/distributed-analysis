package primaryserver

import (
	"log"
	"net/http"
	"src/models"
	"sync"
)

// Server manages the job queue and distributes work
type Server struct {
	jobs    chan models.Job
	mu      sync.RWMutex
	jobMap  map[string]models.Job
	batches map[string]*models.Batch
}

// NewServer creates a new analysis server
func NewServer() *Server {
	return &Server{
		jobs:    make(chan models.Job, 100),
		jobMap:  make(map[string]models.Job),
		batches: make(map[string]*models.Batch),
	}
}

// StartServer starts the HTTP server
func (s *Server) StartServer(addr string) {
	http.HandleFunc("/job", s.handleGetJob)
	http.HandleFunc("/result", s.handleSubmitResult)
	http.HandleFunc("/batch", s.handleGetBatch)
	http.HandleFunc("/queue", s.handleViewQueue)
	http.HandleFunc("/requestForAnalysis", s.requestForAnalysis)

	// get accuracy for batch 
	http.HandleFunc("/batch/accuracy", s.handleBatchAccuracy)


	log.Printf("Starting server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
