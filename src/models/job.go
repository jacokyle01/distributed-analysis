package models

// Job represents a chess position analysis job
// right now jobs are just a FEN, which is one chess move
// TODO
type Job struct {
	ID       string `json:"id"`
	FEN      string `json:"fen"`
	Depth    int    `json:"depth"`
	TimeMS   int    `json:"time_ms"`
	Priority int    `json:"priority"`
}