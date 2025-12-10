package models

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