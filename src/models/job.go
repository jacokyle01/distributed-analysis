package models

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