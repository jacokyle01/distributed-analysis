//TODO run fishnet locally
// TODO api /queue to monitor queue

//TODO how do we piece analyzed moves back together?

package main

import (
	"src/models"
	"src/primaryserver"
	"src/worker"

	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"
)

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

		srv := primaryserver.NewServer()
		srv.StartServer(port)

	case "client":
		serverURL := "http://localhost:8080"
		enginePath := ""

		switch runtime.GOOS {
		case "windows":
			enginePath = "../stockfish/stockfish-windows-x86-64-avx2.exe"
		case "linux":
			enginePath = "../stockfish/stockfish-ubuntu-x86-64-avx512"
		default:
			log.Fatal("Unsupported OS: ", runtime.GOOS)
		}

		if len(os.Args) > 2 {
			serverURL = os.Args[2]
		}
		if len(os.Args) > 3 {
			enginePath = os.Args[3]
		}

		client, err := worker.NewClient(serverURL, enginePath)
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
		srv := primaryserver.NewServer()
		go srv.StartServer(":8080")

		// Wait for server to start
		time.Sleep(time.Second)

		// Example: Submit analysis job
		job := models.Job{
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
