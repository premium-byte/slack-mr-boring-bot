package models

type SlackCommand struct {
	Token       string `form:"token" binding:"required"`
	TeamID      string `form:"team_id" binding:"required"`
	TeamDomain  string `form:"team_domain" binding:"required"`
	ChannelID   string `form:"channel_id" binding:"required"`
	ChannelName string `form:"channel_name" binding:"required"`
	UserID      string `form:"user_id" binding:"required"`
	UserName    string `form:"user_name" binding:"required"`
	Command     string `form:"command" binding:"required"`
	Text        string `form:"text"`
	ResponseURL string `form:"response_url" binding:"required"`
}

type SimpleSlackCommand struct {
	Command string `form:"command"`
	Text    string `form:"text"`
}
