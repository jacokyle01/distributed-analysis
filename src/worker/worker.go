package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"src/models"
	"time"
)

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

	var job models.Job
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		log.Printf("Error decoding job: %v", err)
		return
	}

	log.Printf("Processing job %s: %s", job.ID, job.FEN)

	// Analyze position
	result, err := c.engine.AnalyzePosition(job.FEN, job.Depth, job.TimeMS)
	if err != nil {
		log.Printf("Error analyzing position: %v", err)
		result = &models.Result{Error: err.Error()}
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
