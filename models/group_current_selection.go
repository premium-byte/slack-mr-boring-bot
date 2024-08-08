package models

type GroupCurrentSelection struct {
	Groups map[string]StoredSupportDefinition `json:"groups"`
}

type StoredSupportDefinition struct {
	Teams map[string][]string `json:"teams"`
}
