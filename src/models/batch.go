package models

type Batch struct {
	ID        string            `json:"id"`
	JobIDs    []string          `json:"job_ids"`
	Results   map[string]Result `json:"results"`
	Completed int               `json:"completed"`
	Total     int               `json:"total"`
}
