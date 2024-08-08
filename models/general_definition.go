package models

type GeneralDefinition struct {
	DefaultCron string                       `json:"defaultCron"`
	Messages    map[string]string            `json:"messages"`
	Teams       map[string]map[string]Task   `json:"teams"`
	Groups      map[string]SupportDefinition `json:"groups"`
}

type Task struct {
	Cron    string   `json:"cron"`
	Members []string `json:"members"`
	Message string   `json:"message"`
	Channel string   `json:"channel"`
	Amount  int      `json:"amount"`
}

type SupportDefinition struct {
	Cron               string                    `json:"cron"`
	Teams              map[string]TeamDefinition `json:"teams"`
	Message            string                    `json:"message"`
	Channel            string                    `json:"channel"`
	AmountFromEachTeam int                       `json:"amountFromEachTeam"`
}

type TeamDefinition struct {
	Members []string `json:"members"`
	Amount  int      `json:"amount"`
}
