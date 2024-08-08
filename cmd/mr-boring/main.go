package main

import (
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/robfig/cron"
	"io.mt-borring.bot/api"
	"io.mt-borring.bot/configs"
	"io.mt-borring.bot/models"
	"io.mt-borring.bot/utils"
	"log"
	"strings"
	"sync"
)

func main() {
	_ = godotenv.Load()

	// Start the Slack API
	configs.InitSlackApi()

	r := gin.Default()

	// Load general configuration, current team configuration and current group configuration
	configs.LoadAllConfigurations()

	var mu sync.Mutex
	var wg sync.WaitGroup

	log.Println("Starting to process Teams section...")
	for teamName, taskMap := range configs.GetGeneralConfiguration().Teams {
		for taskName, task := range taskMap {
			wg.Add(1)
			go func(teamName string, taskName string, task models.Task) {
				defer wg.Done()

				c := cron.New()

				err := c.AddFunc(configs.GetCronExpression(task.Cron), func() {
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
	for supportName, supportDefinition := range configs.GetGeneralConfiguration().Groups {
		wg.Add(1)
		go func(supportName string, supportDefinition models.SupportDefinition) {
			defer wg.Done()

			c := cron.New()

			err := c.AddFunc(configs.GetCronExpression(supportDefinition.Cron), func() {
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

	api.ReplaceUserApi(r)
	api.ShowStats(r)

	err := r.Run(":9090")
	if err != nil {
		return
	}
}

func selectUsersForSupport(supportName string, supportDefinition models.SupportDefinition) {
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

		availableMembers := utils.Difference(teamDefinition.Members, configs.GetGroupCurrentSelection().Groups[supportName].Teams[teamName])
		if len(availableMembers) < teamDefinition.Amount {
			log.Printf("Not enough members to select for support %s", supportName)
			log.Println("Not enough users to select. Resetting...")
			configs.GetGroupCurrentSelection().Groups[supportName].Teams[teamName] = []string{}
			configs.SaveGroupSelectedUsers() // clean selection
			selectUsersForSupport(supportName, supportDefinition)
			return
		}

		utils.Shuffle(availableMembers)

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

	log.Printf("Selected users for support %s :: %s\n", supportName, builder.String())
	message := configs.GetMessageToPublish(supportDefinition.Message, supportName)
	configs.SendMessageToSlack(message, builder.String(), supportDefinition.Channel, supportName)
	configs.AddUserToGroupSelection(supportName, users)
	configs.UpdateSlackGroup(userNames, supportName)
}

func selectUserForTask(teamName string, taskName string) {
	log.Println("Selecting user for task", taskName)

	teamMembers := configs.GetGeneralConfiguration().Teams[teamName][taskName].Members
	membersToSelect := configs.GetGeneralConfiguration().Teams[teamName][taskName].Amount
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
	currentSelectedMembers := configs.GetTeamCurrentSelection().Teams[teamName][taskName].Members
	availableMembers := utils.Difference(teamMembers, currentSelectedMembers)

	utils.Shuffle(availableMembers)
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
			configs.AddUserToTeamSelection(teamName, taskName, member)
			taskInfo := configs.GetGeneralConfiguration().Teams[teamName][taskName]
			configs.SendMessageToSlack(taskInfo.Message, member, taskInfo.Channel, taskName)
		}
		return
	}

	// else if counter < membersToSelect
	log.Println("Not enough users to select for task {}. Resetting...", taskName)
	teamTask := configs.GetTeamCurrentSelection().Teams[teamName][taskName]
	teamTask.Members = []string{}
	configs.GetTeamCurrentSelection().Teams[teamName][taskName] = teamTask
	// TODO PS - mode this into a method inside configs, like a action from the list
	configs.SaveTeamSelectedUsers() // clean selection
	selectUserForTask(teamName, taskName)
}
