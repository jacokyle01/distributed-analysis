package primaryserver

import (
	"log"
	"net/http"
	"src/models"
	"sync"
)

// Server manages the job queue and distributes work
type Server struct {
	jobs          chan models.Job
	results       chan models.Result
	mu            sync.RWMutex
	jobMap        map[string]models.Job
	results_store map[string]models.Result
}

// NewServer creates a new analysis server
func NewServer() *Server {
	return &Server{
		jobs:          make(chan models.Job, 100),
		results:       make(chan models.Result, 100),
		jobMap:        make(map[string]models.Job),
		results_store: make(map[string]models.Result),
	}
}

// StartServer starts the HTTP server
func (s *Server) StartServer(addr string) {
	http.HandleFunc("/job", s.handleGetJob)
	http.HandleFunc("/result", s.handleSubmitResult)
	http.HandleFunc("/analyze", s.handleAnalyze)
	http.HandleFunc("/get_result", s.handleGetResult)
	http.HandleFunc("/queue", s.handleViewQueue)
	http.HandleFunc("/requestForAnalysis", s.requestForAnalysis)

	log.Printf("Starting server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
