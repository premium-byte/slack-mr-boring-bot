package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
)

var generalconfiguration GeneralConfiguration
var currentSelectionStorage CurrentSelectionStorage
var currentSupportSelectionStorage CurrentSupportSelectionStorage
var slackApi *slack.Client

func main() {

	_ = godotenv.Load()
	slackToken := os.Getenv("SLACK_TOKEN")
	slackApi = slack.New(slackToken)

	r := gin.Default()
	r.POST("/replace", func(c *gin.Context) {
		fmt.Println("Hello Hello")
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	generalconfiguration = loadGeneralConfiguration()
	currentSelectionStorage = loadCurrentSelectionStorage()
	currentSupportSelectionStorage = loadCurrentSupportSelectionStorage()
	defer saveSelectedUsers(currentSelectionStorage)

	fmt.Println(generalconfiguration.Groups)

	var mu sync.Mutex
	var wg sync.WaitGroup

	for teamName, taskMap := range generalconfiguration.Teams {
		for taskName, task := range taskMap {
			wg.Add(1)
			go func(teamName string, taskName string, task Task) {
				defer wg.Done()

				c := cron.New()

				err := c.AddFunc(getCronExpression(task.Cron), func() {
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

	fmt.Println("Groups", generalconfiguration.Groups)
	for supportName, supportDefinition := range generalconfiguration.Groups {
		wg.Add(1)
		go func(supportName string, supportDefinition SupportDefinition) {
			defer wg.Done()

			c := cron.New()

			err := c.AddFunc(getCronExpression(supportDefinition.Cron), func() {
				mu.Lock()
				selectUsersForSupport(supportName, supportDefinition)
				mu.Unlock()
			})

			if err != nil {
				fmt.Println("Error scheduling Support:", err)
				return
			}

			c.Start()
		}(supportName, supportDefinition)
	}

	wg.Wait()
	err := r.Run(":9090")
	if err != nil {
		return
	}
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

// saveSelectedUsers saves the selected users to a file
func saveSupportedSelectedUsers(currentSupportSelectionStorage CurrentSupportSelectionStorage) {
	data, err := json.MarshalIndent(currentSupportSelectionStorage, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling selected users:", err)
		return
	}

	err = os.WriteFile("current_support_selection_storage.json", data, 0644)
	if err != nil {
		fmt.Println("Error writing selected users to file:", err)
		return
	}
}

func selectUsersForSupport(supportName string, supportDefinition SupportDefinition) {
	fmt.Println("Selecting users for support --> ", supportName)

	users := make(map[string][]string, len(supportDefinition.Teams))
	for teamName, teamDefinition := range supportDefinition.Teams {

		if len(teamDefinition.Members) < teamDefinition.Amount {
			log.Printf("Not enough members to select for support %s", supportName)
			return
		}

		if teamDefinition.Amount == 0 {
			log.Printf("No members to select for support %s", supportName)
			break
		}

		availableMembers := Difference(teamDefinition.Members, currentSupportSelectionStorage.Groups[supportName].Teams[teamName])
		if len(availableMembers) < teamDefinition.Amount {
			log.Printf("Not enough members to select for support %s", supportName)
			fmt.Println("Not enough users to select. Resetting...")
			currentSupportSelectionStorage.Groups[supportName].Teams[teamName] = []string{}
			saveSupportedSelectedUsers(currentSupportSelectionStorage) // clean selection
			selectUsersForSupport(supportName, supportDefinition)
			return
		}

		Shuffle(availableMembers)

		selectedMembers := availableMembers[:teamDefinition.Amount]
		users[teamName] = selectedMembers
	}

	var builder strings.Builder

	var counter int = 0
	for _, selectedUsers := range users {

		var counter2 int = 0
		for _, user := range selectedUsers {
			if counter != len(users)-1 {
				builder.WriteString("<@" + user + ">, ")
			}

			if counter == len(users)-1 && counter2 == len(selectedUsers)-1 {
				builder.WriteString("and <@" + user + ">")
			}
			counter2++
		}
		counter++

		//builder.WriteString("<" + strings.Join(selectedUsers, ">, "))
		// TODO PS - Improve this code
		//if counter != len(users)-1 {
		//	builder.WriteString(">, ")
		//}

	}

	fmt.Println("Selected users for support {} :: {}", supportName, builder.String())
	message := getMessageToPublish(supportDefinition.Message, supportName)
	sendMessageToSlack(message, builder.String(), supportDefinition.Channel, supportName)
	addMemberToCurrentSupportSelectionStorage(supportName, users)
}

func selectUserForTask(teamName string, taskName string) {
	fmt.Println("Selecting user for task", taskName)

	teamMembers := generalconfiguration.Teams[teamName][taskName].Members
	membersToSelect := generalconfiguration.Teams[teamName][taskName].Amount

	if len(teamMembers) < membersToSelect {
		log.Printf("Not enough members to select for task %s", taskName)
		return
	}

	if membersToSelect == 0 {
		log.Printf("No members to select for task %s", taskName)
		return
	}

	counter := 0
	listOfUsers := []string{}
	currentSelectedMembers := currentSelectionStorage.Teams[teamName][taskName].Members
	availableMembers := Difference(teamMembers, currentSelectedMembers)

	Shuffle(availableMembers)
	for _, member := range availableMembers {
		fmt.Printf("[%s] :: %s selected user %s \n", teamName, taskName, member)
		listOfUsers = append(listOfUsers, member)

		counter++
		if counter == membersToSelect {
			break
		}
	}

	if len(listOfUsers) == membersToSelect {

		for _, member := range listOfUsers {
			addMemberToCurrentSelectionStorage(teamName, taskName, member)
			taskInfo := generalconfiguration.Teams[teamName][taskName]
			sendMessageToSlack(taskInfo.Message, member, taskInfo.Channel, taskName)
		}
		return
	}

	// else if counter < membersToSelect
	fmt.Println("Not enough users to select. Resetting...")
	teamTask := currentSelectionStorage.Teams[teamName][taskName]
	teamTask.Members = []string{}
	currentSelectionStorage.Teams[teamName][taskName] = teamTask
	saveSelectedUsers(currentSelectionStorage) // clean selection
	selectUserForTask(teamName, taskName)
}

func Shuffle(slice []string) {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	rand.Shuffle(len(slice), func(i, j int) {
		slice[i], slice[j] = slice[j], slice[i]
	})
}

func Difference(A, B []string) []string {
	// Create a map from list B
	bMap := make(map[string]struct{})
	for _, item := range B {
		bMap[item] = struct{}{}
	}

	// Find elements in A that are not in B
	var diff []string
	for _, item := range A {
		if _, found := bMap[item]; !found {
			diff = append(diff, item)
		}
	}

	return diff
}

func sendMessageToSlack(slackMessage string, member string, channel string, taskName string) {
	messageToPublish := getMessageToPublish(slackMessage, taskName)

	// WORKING: Get user by email
	//_, err := slackApi.GetUserByEmail(member)
	//if err != nil {
	//	log.Fatalf("Failed to find user: %v", err)
	//}

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

func getCronExpression(cronExpression string) string {
	if cronExpression != "" {
		return cronExpression
	}

	return generalconfiguration.DefaultCron
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

func addMemberToCurrentSupportSelectionStorage(supportTeam string, users map[string][]string) {
	if _, ok := currentSupportSelectionStorage.Groups[supportTeam]; !ok {
		currentSupportSelectionStorage.Groups[supportTeam] = StoredSupportDefinition{Teams: make(map[string][]string)}
	}

	for teamName, _ := range users {
		if _, ok := currentSupportSelectionStorage.Groups[supportTeam].Teams[teamName]; !ok {
			currentSupportSelectionStorage.Groups[supportTeam].Teams[teamName] = []string{}
		}
	}

	for teamName, members := range users {
		teamTask := currentSupportSelectionStorage.Groups[supportTeam].Teams[teamName]
		teamTask = append(teamTask, members...)
		// Add member to the existing task
		currentSupportSelectionStorage.Groups[supportTeam].Teams[teamName] = teamTask
	}

	saveSupportedSelectedUsers(currentSupportSelectionStorage)
}

func loadGeneralConfiguration() GeneralConfiguration {

	files, err := ioutil.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		fmt.Println(file.Name())
	}

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

type StoredSupportDefinition struct {
	Teams map[string][]string `json:"teams"`
}

type CurrentSelectionStorage struct {
	Teams map[string]map[string]TaskSelection `json:"teams"`
}

type CurrentSupportSelectionStorage struct {
	Groups map[string]StoredSupportDefinition `json:"groups"`
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

func loadCurrentSupportSelectionStorage() CurrentSupportSelectionStorage {

	var currentSelectionStorage CurrentSupportSelectionStorage
	file, err := os.Open("current_support_selection_storage.json")
	if err != nil {
		fmt.Println("Error opening file:", err)
		// File does not exist or error reading the file, return empty structure
		return CurrentSupportSelectionStorage{Groups: make(map[string]StoredSupportDefinition)}
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		// Error reading file, return empty structure
		fmt.Println("Error reading file:", err)
		return CurrentSupportSelectionStorage{Groups: make(map[string]StoredSupportDefinition)}
	}

	err = json.Unmarshal(data, &currentSelectionStorage)
	if err != nil {
		// Error parsing JSON, return empty structure
		fmt.Println("Error parsing JSON:", err)
		return CurrentSupportSelectionStorage{Groups: make(map[string]StoredSupportDefinition)}
	}

	return currentSelectionStorage
}
