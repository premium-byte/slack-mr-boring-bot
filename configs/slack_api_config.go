package configs

import (
	"fmt"
	"github.com/slack-go/slack"
	"log"
	"os"
	"strings"
)

var slackApi *slack.Client

func InitSlackApi() {
	slackToken := os.Getenv("SLACK_TOKEN")
	slackApi = slack.New(slackToken)
}

func GetSlackApi() *slack.Client {
	return slackApi
}

func SendMessageToSlack(slackMessage string, member string, channel string, taskName string) {
	if channel == "" {
		log.Println("Channel is empty, skipping sending message to Slack")
		return
	}

	messageToPublish := GetMessageToPublish(slackMessage, taskName)
	messageToPublish = strings.Replace(messageToPublish, "{{name}}", member, -1)

	_, _, err := slackApi.PostMessage(
		channel,
		slack.MsgOptionText(messageToPublish, false),
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		log.Printf("Error sending message to Slack: %s\n", err)
		return
	}
}

func UpdateSlackGroup(users []string, groupName string) {
	if groupName == "" {
		log.Println("Group name is empty, skipping updating group")
		return
	}

	userGroups, err := slackApi.GetUserGroups(slack.GetUserGroupsOptionIncludeUsers(true))
	if err != nil {
		return
	}

	foundGroup := false
	groupId := ""
	for _, userGroup := range userGroups {
		if groupName == userGroup.Name {
			foundGroup = true
			groupId = userGroup.ID
		}
	}

	if !foundGroup {
		log.Println("Could not find any group with the name: ", groupName)
		return
	}

	userIds, _ := getUserIDsFromNames(users)
	_, err = slackApi.UpdateUserGroupMembers(groupId, strings.Join(userIds, ","))
	if err != nil {
		log.Printf("failed to remove user from group: %w", err)
		return
	}
}

func getUserIDsFromNames(userNames []string) ([]string, error) {
	var userIDs []string

	// List all users to find their IDs by their names
	users, err := slackApi.GetUsers()
	if err != nil {
		return nil, err
	}

	// Map user names to user IDs
	userNameToID := make(map[string]string)
	for _, user := range users {
		userNameToID[user.Name] = user.ID
	}

	// Convert user names to user IDs
	for _, userName := range userNames {
		userID, ok := userNameToID[userName]
		if !ok {
			return nil, fmt.Errorf("user %s not found", userName)
		}
		userIDs = append(userIDs, userID)
	}

	return userIDs, nil
}
