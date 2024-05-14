package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
	"github.com/slack-go/slack"
)

var originalTeams Configuration
var selectedUsers Configuration
var slackApi *slack.Client

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	slackToken := os.Getenv("SLACK_TOKEN")
	slackApi = slack.New(slackToken)

	// Load the configuration from the JSON file
	originalTeams, _ = loadConfiguration("configuration.json")
	// Load the selected users from the JSON file
	selectedUsers, _ = loadConfiguration("selected_users.json")

	defer saveSelectedUsers(selectedUsers)

	// Initial selection when the program starts
	selectUser()

	c := cron.New()
	// Schedule the function to run daily at 9:00 AM
	_, err := c.AddFunc("*/1 * * * *", dailyTask)
	if err != nil {
		fmt.Println("Error scheduling task:", err)
		return
	}

	// Start the scheduler
	c.Start()

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

func loadConfiguration(fileName string) (Configuration, error) {
	fmt.Println("Loading selected users from file...")

	var selectedUsers Configuration
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Error opening file:", err)
		// File does not exist or error reading the file, return empty structure
		return Configuration{Teams: make(map[string][]string)}, nil
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		// Error reading file, return empty structure
		fmt.Println("Error reading file:", err)
		return Configuration{Teams: make(map[string][]string)}, nil
	}

	err = json.Unmarshal(data, &selectedUsers)
	if err != nil {
		// Error parsing JSON, return empty structure
		fmt.Println("Error parsing JSON:", err)
		return Configuration{Teams: make(map[string][]string)}, nil
	}

	fmt.Println("Loading selected users from file...FINISHED")
	return selectedUsers, nil
}

// saveSelectedUsers saves the selected users to a file
func saveSelectedUsers(selectedUsers Configuration) {
	fmt.Println("Saving selected users to file...")
	data, err := json.MarshalIndent(selectedUsers, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling selected users:", err)
		return
	}

	err = os.WriteFile("selected_users.json", data, 0644)
	if err != nil {
		fmt.Println("Error writing selected users to file:", err)
		return
	}
	fmt.Println("Saving selected users to file...FINISHED")
}

func selectUser() {
	fmt.Println("Selecting user...")

	// Function to select a user
	finishedWithoutResults := true
	for team, members := range originalTeams.Teams {
		for _, member := range members {
			if !strings.Contains(strings.Join(selectedUsers.Teams[team], ","), member) {
				selectedUsers.Teams[team] = append(selectedUsers.Teams[team], member)
				fmt.Printf("Selected user: %s :: Team %s \n", member, team)

				message := fmt.Sprintf("Hello %s you have been selected to do something!", member)
				_, _, err := slackApi.PostMessage(
					"mysuperchannelprivate",
					slack.MsgOptionText(message, false),
				)
				if err != nil {
					fmt.Printf("Error sending message to Slack: %s\n", err)
					return
				}

				saveSelectedUsers(selectedUsers)
				finishedWithoutResults = false
				break
			}
		}
	}

	if !finishedWithoutResults {
		return
	}

	fmt.Println("All users have been selected. Resetting...")
	// Reset selected users
	selectedUsers.Teams = make(map[string][]string)
	saveSelectedUsers(selectedUsers)
	selectUser() // test this better
}

func dailyTask() {
	fmt.Println("Executing daily task at", time.Now())
	selectUser()
	// Your code logic here
}

type Configuration struct {
	Cron  string              `json:"cron"`
	Teams map[string][]string `json:"teams"`
}
