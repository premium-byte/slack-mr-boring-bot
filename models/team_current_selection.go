package models

type TeamCurrentSelection struct {
	Teams map[string]map[string]TaskSelection `json:"teams"`
}

type TaskSelection struct {
	Members []string `json:"members"`
}
