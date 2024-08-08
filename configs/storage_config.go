package configs

import (
	"encoding/json"
	"io"
	"io.mt-borring.bot/models"
	"log"
	"os"
)

var generalDefinition models.GeneralDefinition
var teamCurrentSelection models.TeamCurrentSelection
var groupCurrentSelection models.GroupCurrentSelection

func LoadAllConfigurations() {
	generalDefinition = loadGeneralDefinition()
	teamCurrentSelection = loadTeamCurrentSelection()
	groupCurrentSelection = loadGroupCurrentSelection()
	defer SaveTeamSelectedUsers()
}

func loadGeneralDefinition() models.GeneralDefinition {
	var generalConfiguration models.GeneralDefinition
	file, err := os.Open("configuration.json")
	if err != nil {
		log.Println("Error opening file:", err)
		// File does not exist or error reading the file, return empty structure
		return models.GeneralDefinition{Teams: make(map[string]map[string]models.Task)}
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Println("Error closing General Definition file:", err)
		}
	}(file)

	data, err := io.ReadAll(file)
	if err != nil {
		log.Println("Error reading file:", err)
		return models.GeneralDefinition{Teams: make(map[string]map[string]models.Task)}
	}

	err = json.Unmarshal(data, &generalConfiguration)
	if err != nil {
		log.Println("Error parsing JSON:", err)
		return models.GeneralDefinition{Teams: make(map[string]map[string]models.Task)}
	}

	return generalConfiguration
}

func loadTeamCurrentSelection() models.TeamCurrentSelection {
	var currentSelectionStorage models.TeamCurrentSelection
	file, err := os.Open("current_selection_storage.json")

	if err != nil {
		log.Println("Error opening file:", err)
		// File does not exist or error reading the file, return empty structure
		return models.TeamCurrentSelection{Teams: make(map[string]map[string]models.TaskSelection)}
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Println("Error closing Team Current Selection file:", err)
		}
	}(file)

	data, err := io.ReadAll(file)
	if err != nil {
		// Error reading file, return empty structure
		log.Println("Error reading file:", err)
		return models.TeamCurrentSelection{Teams: make(map[string]map[string]models.TaskSelection)}
	}

	err = json.Unmarshal(data, &currentSelectionStorage)
	if err != nil {
		// Error parsing JSON, return empty structure
		log.Println("Error parsing JSON:", err)
		return models.TeamCurrentSelection{Teams: make(map[string]map[string]models.TaskSelection)}
	}

	return currentSelectionStorage
}

func loadGroupCurrentSelection() models.GroupCurrentSelection {
	var currentSelectionStorage models.GroupCurrentSelection
	file, err := os.Open("current_support_selection_storage.json")

	if err != nil {
		log.Println("Error opening file:", err)
		// File does not exist or error reading the file, return empty structure
		return models.GroupCurrentSelection{Groups: make(map[string]models.StoredSupportDefinition)}
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Println("Error closing Group Current Selection file:", err)
		}
	}(file)

	data, err := io.ReadAll(file)
	if err != nil {
		// Error reading file, return empty structure
		log.Println("Error reading file:", err)
		return models.GroupCurrentSelection{Groups: make(map[string]models.StoredSupportDefinition)}
	}

	err = json.Unmarshal(data, &currentSelectionStorage)
	if err != nil {
		// Error parsing JSON, return empty structure
		log.Println("Error parsing JSON:", err)
		return models.GroupCurrentSelection{Groups: make(map[string]models.StoredSupportDefinition)}
	}

	return currentSelectionStorage
}

func SaveTeamSelectedUsers() {
	data, err := json.MarshalIndent(teamCurrentSelection, "", "  ")
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

func SaveGroupSelectedUsers() {
	data, err := json.MarshalIndent(groupCurrentSelection, "", "  ")
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

func GetGeneralConfiguration() models.GeneralDefinition {
	return generalDefinition
}

func GetTeamCurrentSelection() models.TeamCurrentSelection {
	return teamCurrentSelection
}

func GetGroupCurrentSelection() models.GroupCurrentSelection {
	return groupCurrentSelection
}

func GetMessageToPublish(teamTaskMessage string, taskName string) string {
	if teamTaskMessage != "" {
		return teamTaskMessage
	}

	return GetGeneralConfiguration().Messages[taskName]
}

func GetCronExpression(cronExpression string) string {
	if cronExpression != "" {
		return cronExpression
	}

	return GetGeneralConfiguration().DefaultCron
}

func AddUserToGroupSelection(supportTeam string, users map[string][]string) {
	if _, ok := GetGroupCurrentSelection().Groups[supportTeam]; !ok {
		GetGroupCurrentSelection().Groups[supportTeam] = models.StoredSupportDefinition{Teams: make(map[string][]string)}
	}

	for teamName, _ := range users {
		if _, ok := GetGroupCurrentSelection().Groups[supportTeam].Teams[teamName]; !ok {
			GetGroupCurrentSelection().Groups[supportTeam].Teams[teamName] = []string{}
		}
	}

	for teamName, members := range users {
		teamTask := GetGroupCurrentSelection().Groups[supportTeam].Teams[teamName]
		teamTask = append(teamTask, members...)
		// Add member to the existing task
		GetGroupCurrentSelection().Groups[supportTeam].Teams[teamName] = teamTask
	}

	SaveGroupSelectedUsers()
}

func AddUserToTeamSelection(team string, task string, member string) {
	if _, ok := GetTeamCurrentSelection().Teams[team]; !ok {
		GetTeamCurrentSelection().Teams[team] = make(map[string]models.TaskSelection)
	}

	if _, ok := GetTeamCurrentSelection().Teams[team][task]; !ok {
		GetTeamCurrentSelection().Teams[team][task] = models.TaskSelection{
			Members: []string{},
		}
	}

	teamTask := GetTeamCurrentSelection().Teams[team][task]
	teamTask.Members = append(teamTask.Members, member)
	// Add member to the existing task
	GetTeamCurrentSelection().Teams[team][task] = teamTask

	SaveTeamSelectedUsers()
}
