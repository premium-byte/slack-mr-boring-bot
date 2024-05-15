package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"github.com/robfig/cron"
	"github.com/slack-go/slack"
)

var generalconfiguration GeneralConfiguration
var currentSelectionStorage CurrentSelectionStorage
var slackApi *slack.Client

func main() {

	_ = godotenv.Load()

	slackToken := os.Getenv("SLACK_TOKEN")
	slackApi = slack.New(slackToken)

	generalconfiguration = loadGeneralConfiguration()
	currentSelectionStorage = loadCurrentSelectionStorage()
	defer saveSelectedUsers(currentSelectionStorage)

	var mu sync.Mutex
	var wg sync.WaitGroup

	for teamName, taskMap := range generalconfiguration.Teams {
		for taskName, task := range taskMap {
			wg.Add(1)
			go func(teamName string, taskName string, task Task) {
				defer wg.Done()

				c := cron.New()

				err := c.AddFunc(task.Cron, func() {
					mu.Lock()
					selectUserForTask(teamName, taskName)
					mu.Unlock()
				})

				if err != nil {
					fmt.Println("Error scheduling task:", err)
					return
				}

				c.Start()
			}(teamName, taskName, task)
		}
	}

	wg.Wait()
	// Keep the main function running indefinitely
	select {}
}

type SelectedTeams struct {
	Users map[string]bool `json:"users"`
}

type SelectedUsers struct {
	Teams []TeamSelected `json:"teams"`
}

type TeamSelected struct {
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

// saveSelectedUsers saves the selected users to a file
func saveSelectedUsers(currentSelectionStorage CurrentSelectionStorage) {
	data, err := json.MarshalIndent(currentSelectionStorage, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling selected users:", err)
		return
	}

	err = os.WriteFile("current_selection_storage.json", data, 0644)
	if err != nil {
		fmt.Println("Error writing selected users to file:", err)
		return
	}
}

func selectUserForTask(teamName string, taskName string) {
	finishedWithoutResults := true
	for _, member := range generalconfiguration.Teams[teamName][taskName].Members {
		if !strings.Contains(strings.Join(currentSelectionStorage.Teams[teamName][taskName].Members, ","), member) {
			finishedWithoutResults = false
			fmt.Printf("[%s] :: %s selected user %s \n", teamName, taskName, member)
			addMemberToCurrentSelectionStorage(teamName, taskName, member)

			taskInfo := generalconfiguration.Teams[teamName][taskName]
			sendMessageToSlack(taskInfo.Message, member, taskInfo.Channel, taskName)
			break
		}
	}

	if !finishedWithoutResults {
		return
	}

	fmt.Println("All users have been selected. Resetting...")
	teamTask := currentSelectionStorage.Teams[teamName][taskName]
	teamTask.Members = []string{}
	currentSelectionStorage.Teams[teamName][taskName] = teamTask
	saveSelectedUsers(currentSelectionStorage)
	//	selectUser() // test this better
}

func sendMessageToSlack(slackMessage string, member string, channel string, taskName string) {
	messageToPublish := getMessageToPublish(slackMessage, taskName)
	messageToPublish = strings.Replace(messageToPublish, "{{name}}", member, -1)

	_, _, err := slackApi.PostMessage(
		channel,
		slack.MsgOptionText(messageToPublish, false),
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		fmt.Printf("Error sending message to Slack: %s\n", err)
		return
	}
}

func getMessageToPublish(teamTaskMessage string, taskName string) string {
	if teamTaskMessage != "" {
		return teamTaskMessage
	}

	return generalconfiguration.Messages[taskName]
}

func addMemberToCurrentSelectionStorage(team string, task string, member string) {
	if _, ok := currentSelectionStorage.Teams[team]; !ok {
		currentSelectionStorage.Teams[team] = make(map[string]TaskSelection)
	}

	if _, ok := currentSelectionStorage.Teams[team][task]; !ok {
		currentSelectionStorage.Teams[team][task] = TaskSelection{
			Members: []string{},
		}
	}

	teamTask := currentSelectionStorage.Teams[team][task]
	teamTask.Members = append(teamTask.Members, member)
	// Add member to the existing task
	currentSelectionStorage.Teams[team][task] = teamTask

	saveSelectedUsers(currentSelectionStorage)
}

type Configuration struct {
	Cron  string              `json:"cron"`
	Teams map[string][]string `json:"teams"`
}

func loadGeneralConfiguration() GeneralConfiguration {

	var generalConfiguration GeneralConfiguration
	file, err := os.Open("configuration.json")
	if err != nil {
		fmt.Println("Error opening file:", err)
		// File does not exist or error reading the file, return empty structure
		return GeneralConfiguration{Teams: make(map[string]map[string]Task)}
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		// Error reading file, return empty structure
		fmt.Println("Error reading file:", err)
		return GeneralConfiguration{Teams: make(map[string]map[string]Task)}
	}

	err = json.Unmarshal(data, &generalConfiguration)
	if err != nil {
		// Error parsing JSON, return empty structure
		fmt.Println("Error parsing JSON:", err)
		return GeneralConfiguration{Teams: make(map[string]map[string]Task)}
	}

	return generalConfiguration
}

type GeneralConfiguration struct {
	Messages map[string]string          `json:"messages"`
	Teams    map[string]map[string]Task `json:"teams"`
}

type Task struct {
	Cron    string   `json:"cron"`
	Members []string `json:"members"`
	Message string   `json:"message"`
	Channel string   `json:"channel"`
}

type CurrentSelectionStorage struct {
	Teams map[string]map[string]TaskSelection `json:"teams"`
}

type TaskSelection struct {
	Members []string `json:"members"`
}

func loadCurrentSelectionStorage() CurrentSelectionStorage {

	var currentSelectionStorage CurrentSelectionStorage
	file, err := os.Open("current_selection_storage.json")
	if err != nil {
		fmt.Println("Error opening file:", err)
		// File does not exist or error reading the file, return empty structure
		return CurrentSelectionStorage{Teams: make(map[string]map[string]TaskSelection)}
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		// Error reading file, return empty structure
		fmt.Println("Error reading file:", err)
		return CurrentSelectionStorage{Teams: make(map[string]map[string]TaskSelection)}
	}

	err = json.Unmarshal(data, &currentSelectionStorage)
	if err != nil {
		// Error parsing JSON, return empty structure
		fmt.Println("Error parsing JSON:", err)
		return CurrentSelectionStorage{Teams: make(map[string]map[string]TaskSelection)}
	}

	return currentSelectionStorage
}
