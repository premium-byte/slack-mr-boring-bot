package api

import (
	"github.com/gin-gonic/gin"
	"io.mt-borring.bot/configs"
	"io.mt-borring.bot/constants"
	"io.mt-borring.bot/models"
	"io.mt-borring.bot/utils"
	"log"
	"net/http"
	"regexp"
	"strings"
)

func ReplaceUserApi(r *gin.Engine) gin.IRoutes {
	return r.POST("/replace", func(c *gin.Context) {
		var command models.SlackCommand
		if err := c.ShouldBind(&command); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		rs := regexp.MustCompile(constants.SlackReplaceCommandRegex)
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
}

func ShowStats(r *gin.Engine) gin.IRoutes {
	return r.POST("/show", func(c *gin.Context) {
		var command models.SimpleSlackCommand
		if err := c.ShouldBind(&command); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		rs := regexp.MustCompile(constants.SlackReplaceCommandRegex)
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
}

/**
 * @param username - pedro
 * @param replaceType - teams or groups
 * @param teamOrGroup - payments-zeus or payments-zeus
 * @param teamMeeting - daily
 */
func replaceUser(username string, teamType string, teamOrGroup string, teamMeeting string) string {
	if "teams" == teamType {
		currentSelectionMembers := configs.GetTeamCurrentSelection().Teams[teamOrGroup][teamMeeting].Members
		if len(currentSelectionMembers) == 0 {
			log.Printf("Not members to replace for team %s", teamOrGroup)
			return "nobody"
		}

		generalConfigurationMembers := configs.GetGeneralConfiguration().Teams[teamOrGroup][teamMeeting].Members

		var newMember string
		availableMembers := utils.Difference(generalConfigurationMembers, currentSelectionMembers)
		if len(availableMembers) == 0 {
			log.Printf("Not enough members to select for team %s", teamOrGroup)
			utils.Shuffle(generalConfigurationMembers)
			newMember = generalConfigurationMembers[0]
		}

		utils.Shuffle(availableMembers)
		newMember = availableMembers[0]

		replaceUserInTeamCurrentSelection(teamOrGroup, teamMeeting, newMember, username)
		return newMember
	}

	if "groups" == teamType {
		currentSelectionMembers := configs.GetGroupCurrentSelection().Groups[teamOrGroup].Teams[teamMeeting]
		if len(currentSelectionMembers) == 0 {
			log.Printf("Not members to replace for team %s", teamOrGroup)
			return "nobody"
		}

		generalConfigurationMembers := configs.GetGeneralConfiguration().Groups[teamOrGroup].Teams[teamMeeting].Members

		var newMember string
		availableMembers := utils.Difference(generalConfigurationMembers, currentSelectionMembers)
		if len(availableMembers) == 0 {
			log.Printf("Not enough members to select for team %s", teamOrGroup)
			utils.Shuffle(generalConfigurationMembers)
			newMember = generalConfigurationMembers[0]
		}

		utils.Shuffle(availableMembers)
		newMember = availableMembers[0]

		replaceUserInGroupCurrentSelection(username, teamOrGroup, teamMeeting, newMember)
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
			return configs.GetTeamCurrentSelection().Teams[teamOrGroup][teamMeeting].Members
		}

		if "groups" == teamType {
			return configs.GetGroupCurrentSelection().Groups[teamOrGroup].Teams[teamMeeting]
		}
	}

	if "available" == operationType {
		if "teams" == teamType {
			currentSelectionMembers := configs.GetTeamCurrentSelection().Teams[teamOrGroup][teamMeeting].Members
			generalConfigurationMembers := configs.GetGeneralConfiguration().Teams[teamOrGroup][teamMeeting].Members

			return utils.Difference(generalConfigurationMembers, currentSelectionMembers)
		}

		if "groups" == teamType {
			currentSelectionMembers := configs.GetGroupCurrentSelection().Groups[teamOrGroup].Teams[teamMeeting]
			generalConfigurationMembers := configs.GetGeneralConfiguration().Groups[teamOrGroup].Teams[teamMeeting].Members

			return utils.Difference(generalConfigurationMembers, currentSelectionMembers)
		}
	}

	return []string{}
}

// replaceUserInTeamCurrentSelection -> replaceMemberToCurrentSelectionStorage
func replaceUserInTeamCurrentSelection(team string, task string, member string, memberToReplace string) {
	if _, ok := configs.GetTeamCurrentSelection().Teams[team]; !ok {
		configs.GetTeamCurrentSelection().Teams[team] = make(map[string]models.TaskSelection)
	}

	if _, ok := configs.GetTeamCurrentSelection().Teams[team][task]; !ok {
		configs.GetTeamCurrentSelection().Teams[team][task] = models.TaskSelection{
			Members: []string{},
		}
	}

	for i, element := range configs.GetTeamCurrentSelection().Teams[team][task].Members {
		if memberToReplace == element {
			configs.GetTeamCurrentSelection().Teams[team][task].Members[i] = member
		}
	}

	configs.SaveTeamSelectedUsers()
}

// replaceUserInGroupCurrentSelection -> replaceMemberToCurrentSupportSelectionStorage
func replaceUserInGroupCurrentSelection(username string, teamOrGroup string, teamMeeting string, newMember string) {

	for i, member := range configs.GetGroupCurrentSelection().Groups[teamOrGroup].Teams[teamMeeting] {
		if username == member {
			configs.GetGroupCurrentSelection().Groups[teamOrGroup].Teams[teamMeeting][i] = newMember
		}
	}

	configs.SaveGroupSelectedUsers()
}
