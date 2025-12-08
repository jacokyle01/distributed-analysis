package worker

import (
	"bufio"
	"fmt"
	"os/exec"
	"src/models"
	"strings"
)

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
func (e *ChessEngine) AnalyzePosition(fen string, depth int, timeMS int) (*models.Result, error) {
	// Set position
	e.sendCommand(fmt.Sprintf("position fen %s", fen))

	// Start analysis
	goCmd := fmt.Sprintf("go depth %d movetime %d", depth, timeMS)
	e.sendCommand(goCmd)

	// Read engine output
	lines := e.readUntilBestMove()

	res := &models.Result{
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
						fmt.Sscanf(parts[i+2], "%d", &res.Eval)
					}
				case "depth":
					if i+1 < len(parts) {
						fmt.Sscanf(parts[i+1], "%d", &res.Depth)
					}
				case "nodes":
					if i+1 < len(parts) {
						fmt.Sscanf(parts[i+1], "%d", &res.Nodes)
					}
				case "nps":
					if i+1 < len(parts) {
						fmt.Sscanf(parts[i+1], "%d", &res.NodesPerS)
					}
				case "pv":
					if i+1 < len(parts) {
						res.PV = strings.Join(parts[i+1:], " ")
					}
				}
			}
		} else if strings.HasPrefix(line, "bestmove") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				res.BestMove = parts[1]
			}
		}
	}

	return res, nil
}

func (e *ChessEngine) Close() {
	e.sendCommand("quit")
	e.cmd.Wait()
}
