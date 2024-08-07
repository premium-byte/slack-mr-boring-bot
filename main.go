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
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
)

var generalConfiguration GeneralConfiguration
var currentSelectionStorage CurrentSelectionStorage
var currentSupportSelectionStorage CurrentSupportSelectionStorage
var slackApi *slack.Client

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

func main() {
	_ = godotenv.Load()
	slackToken := os.Getenv("SLACK_TOKEN")
	slackApi = slack.New(slackToken)
	regex := `(selected|available)\s+(teams|groups)\s+([\p{L}\p{N}-]+)\s+([\p{L}\p{N}-]+)`

	r := gin.Default()

	generalConfiguration = loadGeneralConfiguration()
	currentSelectionStorage = loadCurrentSelectionStorage()
	currentSupportSelectionStorage = loadCurrentSupportSelectionStorage()
	defer saveSelectedUsers(currentSelectionStorage)

	var mu sync.Mutex
	var wg sync.WaitGroup

	log.Println("Starting to process Teams section...")
	for teamName, taskMap := range generalConfiguration.Teams {
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
					log.Println("Error scheduling task:", err)
					return
				}

				c.Start()
			}(teamName, taskName, task)
		}
	}

	log.Println("Starting to process Groups section...")
	for supportName, supportDefinition := range generalConfiguration.Groups {
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
				log.Println("Error scheduling Support:", err)
				return
			}

			c.Start()
		}(supportName, supportDefinition)
	}

	wg.Wait()

	r.POST("/replace", func(c *gin.Context) {
		var command SlackCommand
		if err := c.ShouldBind(&command); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		rs := regexp.MustCompile(regex)
		var matches = rs.FindAllStringSubmatch(command.Text, -1)
		var username string
		var teamType string
		var teamOrGroup string
		var teamMeeting string
		for _, match := range matches {
			username = match[1]
			teamType = match[2]
			teamOrGroup = match[3]
			teamMeeting = match[4]
		}

		newMember := replaceUser(username, teamType, teamOrGroup, teamMeeting)

		log.Println("Text :: " + command.Text)
		log.Println("Command :: " + command.Command)
		log.Println("New Member :: " + newMember)
		c.JSON(http.StatusOK, gin.H{
			"response_type": "in_channel",
			"text":          "It's your turn, " + newMember,
		})
	})

	r.POST("/show", func(c *gin.Context) {
		var command SimpleSlackCommand
		if err := c.ShouldBind(&command); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		rs := regexp.MustCompile(regex)
		var matches = rs.FindAllStringSubmatch(command.Text, -1)
		var operationType string
		var teamType string
		var teamOrGroup string
		var teamMeeting string
		for _, match := range matches {
			operationType = match[1]
			teamType = match[2]
			teamOrGroup = match[3]
			teamMeeting = match[4]
		}

		users := showUsers(operationType, teamType, teamOrGroup, teamMeeting)

		log.Println("Text :: " + command.Text)
		log.Println("Command :: " + command.Command)
		log.Println("Users :: " + strings.Join(users, ", "))
		c.JSON(http.StatusOK, gin.H{
			"response_type": "in_channel",
			"text":          strings.Join(users, ", "),
		})
	})

	err := r.Run(":9090")
	if err != nil {
		return
	}
}

/**
 * @param username - pedro
 * @param replaceType - teams or groups
 * @param teamOrGroup - payments-zeus or payments-zeus
 * @param teamMeeting - daily
 */
func replaceUser(username string, teamType string, teamOrGroup string, teamMeeting string) string {
	if "teams" == teamType {
		currentSelectionMembers := currentSelectionStorage.Teams[teamOrGroup][teamMeeting].Members
		if len(currentSelectionMembers) == 0 {
			log.Printf("Not members to replace for team %s", teamOrGroup)
			return "nobody"
		}

		generalConfigurationMembers := generalConfiguration.Teams[teamOrGroup][teamMeeting].Members

		var newMember string
		availableMembers := Difference(generalConfigurationMembers, currentSelectionMembers)
		if len(availableMembers) == 0 {
			log.Printf("Not enough members to select for team %s", teamOrGroup)
			Shuffle(generalConfigurationMembers)
			newMember = generalConfigurationMembers[0]
		}

		Shuffle(availableMembers)
		newMember = availableMembers[0]

		replaceMemberToCurrentSelectionStorage(teamOrGroup, teamMeeting, newMember, username)
		return newMember
	}

	if "groups" == teamType {
		currentSelectionMembers := currentSupportSelectionStorage.Groups[teamOrGroup].Teams[teamMeeting]
		if len(currentSelectionMembers) == 0 {
			log.Printf("Not members to replace for team %s", teamOrGroup)
			return "nobody"
		}

		generalConfigurationMembers := generalConfiguration.Groups[teamOrGroup].Teams[teamMeeting].Members

		var newMember string
		availableMembers := Difference(generalConfigurationMembers, currentSelectionMembers)
		if len(availableMembers) == 0 {
			log.Printf("Not enough members to select for team %s", teamOrGroup)
			Shuffle(generalConfigurationMembers)
			newMember = generalConfigurationMembers[0]
		}

		Shuffle(availableMembers)
		newMember = availableMembers[0]

		replaceMemberToCurrentSupportSelectionStorage(username, teamOrGroup, teamMeeting, newMember)
		return newMember
	}

	return "nobody"
}

/**
 * @param username - pedro
 * @param replaceType - teams or groups
 * @param teamOrGroup - payments-zeus or payments-zeus
 * @param teamMeeting - daily
 */
func showUsers(operationType string, teamType string, teamOrGroup string, teamMeeting string) []string {
	if "selected" == operationType {
		if "teams" == teamType {
			return currentSelectionStorage.Teams[teamOrGroup][teamMeeting].Members
		}

		if "groups" == teamType {
			return currentSupportSelectionStorage.Groups[teamOrGroup].Teams[teamMeeting]
		}
	}

	if "available" == operationType {
		if "teams" == teamType {
			currentSelectionMembers := currentSelectionStorage.Teams[teamOrGroup][teamMeeting].Members
			generalConfigurationMembers := generalConfiguration.Teams[teamOrGroup][teamMeeting].Members

			return Difference(generalConfigurationMembers, currentSelectionMembers)
		}

		if "groups" == teamType {
			currentSelectionMembers := currentSupportSelectionStorage.Groups[teamOrGroup].Teams[teamMeeting]
			generalConfigurationMembers := generalConfiguration.Groups[teamOrGroup].Teams[teamMeeting].Members

			return Difference(generalConfigurationMembers, currentSelectionMembers)
		}
	}

	return []string{}
}

// saveSelectedUsers saves the selected users to a file
func saveSelectedUsers(currentSelectionStorage CurrentSelectionStorage) {
	data, err := json.MarshalIndent(currentSelectionStorage, "", "  ")
	if err != nil {
		log.Println("Error marshalling selected users:", err)
		return
	}

	err = os.WriteFile("current_selection_storage.json", data, 0644)
	if err != nil {
		log.Println("Error writing selected users to file:", err)
		return
	}
}

// saveSelectedUsers saves the selected users to a file
func saveSupportedSelectedUsers(currentSupportSelectionStorage CurrentSupportSelectionStorage) {
	data, err := json.MarshalIndent(currentSupportSelectionStorage, "", "  ")
	if err != nil {
		log.Println("Error marshalling selected users:", err)
		return
	}

	err = os.WriteFile("current_support_selection_storage.json", data, 0644)
	if err != nil {
		log.Println("Error writing selected users to file:", err)
		return
	}
}

func selectUsersForSupport(supportName string, supportDefinition SupportDefinition) {
	log.Println("Selecting users for support --> ", supportName)
	// TODO PS - Add validation for the empty scenarios

	userNames := []string{}
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
			log.Println("Not enough users to select. Resetting...")
			currentSupportSelectionStorage.Groups[supportName].Teams[teamName] = []string{}
			saveSupportedSelectedUsers(currentSupportSelectionStorage) // clean selection
			selectUsersForSupport(supportName, supportDefinition)
			return
		}

		Shuffle(availableMembers)

		selectedMembers := availableMembers[:teamDefinition.Amount]
		users[teamName] = selectedMembers
		userNames = append(userNames, selectedMembers...)
	}

	var builder strings.Builder

	var counter = 0
	for _, selectedUsers := range users {

		var counter2 = 0
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

	log.Println("Selected users for support {} :: {}", supportName, builder.String())
	message := getMessageToPublish(supportDefinition.Message, supportName)
	sendMessageToSlack(message, builder.String(), supportDefinition.Channel, supportName)
	addMemberToCurrentSupportSelectionStorage(supportName, users)
	updateSlackGroup(userNames, supportName) // TODO PS - Add condition for empty channel
}

func selectUserForTask(teamName string, taskName string) {
	log.Println("Selecting user for task", taskName)

	teamMembers := generalConfiguration.Teams[teamName][taskName].Members
	membersToSelect := generalConfiguration.Teams[teamName][taskName].Amount
	if membersToSelect == 0 {
		log.Println("No members to select for task ", taskName)
		return
	}

	if len(teamMembers) < membersToSelect {
		log.Println("Not enough members to select for task ", taskName)
		return
	}

	if membersToSelect == 0 {
		log.Println("No members to select for task ", taskName)
		return
	}

	counter := 0
	var listOfUsers []string
	currentSelectedMembers := currentSelectionStorage.Teams[teamName][taskName].Members
	availableMembers := Difference(teamMembers, currentSelectedMembers)

	Shuffle(availableMembers)
	for _, member := range availableMembers {
		log.Printf("[%s] :: %s selected user %s \n", teamName, taskName, member)
		listOfUsers = append(listOfUsers, member)

		counter++
		if counter == membersToSelect {
			break
		}
	}

	if len(listOfUsers) == membersToSelect {

		for _, member := range listOfUsers {
			addMemberToCurrentSelectionStorage(teamName, taskName, member)
			taskInfo := generalConfiguration.Teams[teamName][taskName]
			sendMessageToSlack(taskInfo.Message, member, taskInfo.Channel, taskName)
		}
		return
	}

	// else if counter < membersToSelect
	log.Println("Not enough users to select for task {}. Resetting...", taskName)
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
	if channel == "" {
		log.Println("Channel is empty, skipping sending message to Slack")
		return
	}

	messageToPublish := getMessageToPublish(slackMessage, taskName)
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

func updateSlackGroup(users []string, groupName string) {
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

func getMessageToPublish(teamTaskMessage string, taskName string) string {
	if teamTaskMessage != "" {
		return teamTaskMessage
	}

	return generalConfiguration.Messages[taskName]
}

func getCronExpression(cronExpression string) string {
	if cronExpression != "" {
		return cronExpression
	}

	return generalConfiguration.DefaultCron
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

func replaceMemberToCurrentSelectionStorage(team string, task string, member string, memberToReplace string) {
	if _, ok := currentSelectionStorage.Teams[team]; !ok {
		currentSelectionStorage.Teams[team] = make(map[string]TaskSelection)
	}

	if _, ok := currentSelectionStorage.Teams[team][task]; !ok {
		currentSelectionStorage.Teams[team][task] = TaskSelection{
			Members: []string{},
		}
	}

	for i, element := range currentSelectionStorage.Teams[team][task].Members {
		if memberToReplace == element {
			currentSelectionStorage.Teams[team][task].Members[i] = member
		}
	}

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

func replaceMemberToCurrentSupportSelectionStorage(username string, teamOrGroup string, teamMeeting string, newMember string) {

	for i, member := range currentSupportSelectionStorage.Groups[teamOrGroup].Teams[teamMeeting] {
		if username == member {
			currentSupportSelectionStorage.Groups[teamOrGroup].Teams[teamMeeting][i] = newMember
		}
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
		log.Println("Error opening file:", err)
		// File does not exist or error reading the file, return empty structure
		return GeneralConfiguration{Teams: make(map[string]map[string]Task)}
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		// Error reading file, return empty structure
		log.Println("Error reading file:", err)
		return GeneralConfiguration{Teams: make(map[string]map[string]Task)}
	}

	err = json.Unmarshal(data, &generalConfiguration)
	if err != nil {
		// Error parsing JSON, return empty structure
		log.Println("Error parsing JSON:", err)
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
		log.Println("Error opening file:", err)
		// File does not exist or error reading the file, return empty structure
		return CurrentSelectionStorage{Teams: make(map[string]map[string]TaskSelection)}
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		// Error reading file, return empty structure
		log.Println("Error reading file:", err)
		return CurrentSelectionStorage{Teams: make(map[string]map[string]TaskSelection)}
	}

	err = json.Unmarshal(data, &currentSelectionStorage)
	if err != nil {
		// Error parsing JSON, return empty structure
		log.Println("Error parsing JSON:", err)
		return CurrentSelectionStorage{Teams: make(map[string]map[string]TaskSelection)}
	}

	return currentSelectionStorage
}

func loadCurrentSupportSelectionStorage() CurrentSupportSelectionStorage {

	var currentSelectionStorage CurrentSupportSelectionStorage
	file, err := os.Open("current_support_selection_storage.json")
	if err != nil {
		log.Println("Error opening file:", err)
		// File does not exist or error reading the file, return empty structure
		return CurrentSupportSelectionStorage{Groups: make(map[string]StoredSupportDefinition)}
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		// Error reading file, return empty structure
		log.Println("Error reading file:", err)
		return CurrentSupportSelectionStorage{Groups: make(map[string]StoredSupportDefinition)}
	}

	err = json.Unmarshal(data, &currentSelectionStorage)
	if err != nil {
		// Error parsing JSON, return empty structure
		log.Println("Error parsing JSON:", err)
		return CurrentSupportSelectionStorage{Groups: make(map[string]StoredSupportDefinition)}
	}

	return currentSelectionStorage
}
