package helpbot

import (
	"errors"
	"fmt"

	"github.com/Raytar/helpbot/database"
	"github.com/Raytar/helpbot/models"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func (bot *HelpBot) initEvents() {
	// create a handler and bind it to new message events
	bot.client.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		// middleware
		user := getMember(s, i)
		if i.Member == nil || i.GuildID == "" {
			sendMsg(s, i.User, "This bot only works in a server.")
			return
		}
		fmt.Println("InteractionCreate", i.ApplicationCommandData().Name, user)
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		// ignore bot messages
		if i.Member.User.Bot {
			return
		}

		// handler
		bot.log.Infof("Received interaction: %+v", i)
		bot.discordMessageCreate(s, i)
	})

	bot.client.AddHandler(bot.discordServerJoin)
	bot.client.AddHandler(bot.discordServerUpdate)
}

func (bot *HelpBot) discordServerUpdate(s *discordgo.Session, e *discordgo.GuildUpdate) {
	bot.log.Infof("Server updated: %s", e.Name)
}

var (
	RoleStudent   = "Student"
	RoleAssistant = "Teaching Assistant"
	Hoist         = true
)

func (bot *HelpBot) discordServerJoin(s *discordgo.Session, e *discordgo.GuildCreate) {
	bot.log.Infof("Joined server: %s, id: %s, channel: %s", e.Name, e.ID, e.SystemChannelID)

	// Check if the server has been registered with a course
	// If not, send a message to the server owner to let them know
	// that the server needs to be registered with a course
	// and that the bot will not work until the server is registered

	_, err := bot.db.GetCourse(&models.Course{GuildID: e.ID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		bot.log.Errorf("Failed to get course: %s", err)
		bot.client.ChannelMessageSend(e.SystemChannelID, "This server has not been configured with a course. Please contact the server owner to configure this server for a course.")

		choices := courseChoices(bot.db)
		if len(choices) == 0 {
			bot.client.ChannelMessageSend(e.SystemChannelID, "There are no courses available to configure this server with. Please contact the server owner to add a course.")
			return
		}
		// TODO: Add a command to configure the server with a course
		if _, err := bot.client.ApplicationCommandCreate(bot.cfg.AppID, e.ID, &discordgo.ApplicationCommand{
			Name:                     "configure",
			Description:              "Configure this server with a course",
			DefaultMemberPermissions: &permAdmin,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "course",
					Description: "The course to configure this server with",
					Choices:     choices,
					Required:    true,
				},
			},
		}); err != nil {
			bot.log.Errorf("Failed to create command: %s", err)
		}
		return
	} else if err != nil {
		bot.log.Errorf("Failed to get course: %s", err)
		// TODO: Send a message to the server owner to let them know that the bot failed to get the course
		return
	}

	// Create roles and commands for the server
	_ = bot.initServer(s, e.ID)

	// Announce that the bot is online and ready to help
	// TODO: Might be best to send this to the server owner
	// bot.client.ChannelMessageSend(e.SystemChannelID, "HelpBot is online! :robot:")
}

	// Send a message to the server owner
	// to let them know that the server name should match the course name
	//sendMsg(s, &discordgo.User{ID: e.OwnerID}, fmt.Sprintf("The server name '%s' does not match any courses. Please change the server name to match the course name and then add the bot back to the server.", e.Name))

	// Remove the bot from the server
	//if err := s.GuildLeave(e.ID); err != nil {
	//	bot.log.Errorf("Failed to leave server: %s", err)
	//}
	//bot.log.Infof("Left server: %s", e.Name)
	//bot.log.Infof("The server name '%s' does not match any courses. Please change the server name to match the course name and then add the bot back to the server.", e.Name)

	commands := GetCommands(bot.courses)
	// Register slash commands. If a command already exists, it will be updated.
	for _, cmd := range commands {
		log.Info("Registering command: ", cmd.Name, " in server: ", e.Name, " with id: ", e.ID)
		// Set permissions for all commands to NoPermission, except for the base commands.
		// Base commands are commands that are available to everyone.
		// All other commands need to be explicitly added to a role by the server admin.
		if _, ok := bot.baseCommands[cmd.Name]; !ok {
			cmd.DefaultMemberPermissions = &NoPermission
		}
		_, err := bot.client.ApplicationCommandCreate(bot.cfg.AppID, e.ID, cmd)
		if err != nil {
			log.Errorln("Failed to create global command:", err)
		}
	}

	// Get all roles in the server.
	roles, err := bot.client.GuildRoles(e.ID)
	if err != nil {
		log.Errorln("Failed to get roles:", err)
	}

	// Create a map of role name to role ID.
	roleMap := make(map[string]string)
	for _, role := range roles {
		if role.Name == RoleStudent || role.Name == RoleAssistant {
			// Student or Teaching Assistant role already exists.
			log.Info("Role already exists: ", role.Name, " with id: ", role.ID)
			roleMap[role.Name] = role.ID
			// Skip creating the role.
		}
	}

	// Create roles that don't exist.
	for _, roleName := range []string{RoleStudent, RoleAssistant} {
		if _, ok := roleMap[roleName]; ok {
			// Role already exists.
			continue
		}

		log.Info("Creating role: ", roleName, " in server: ", e.Name, " with id: ", e.ID)
		role, err := bot.client.GuildRoleCreate(e.ID, &discordgo.RoleParams{
			Name:        roleName,
			Hoist:       &Hoist,
			Permissions: &NoPermission,
		})
		if err != nil {
			log.Errorln("Failed to create role:", err)
		}
		roleMap[roleName] = role.ID
	}

	bot.roles[e.ID] = roleMap
}

func (bot *HelpBot) discordMessageCreate(s *discordgo.Session, m *discordgo.InteractionCreate) {

	for _, content := range m.ApplicationCommandData().Options {
		fmt.Println("content", content)
	}

	command := m.ApplicationCommandData().Name

	gm := getMember(s, m)
	if gm == nil {
		bot.log.Infoln("messageCreate: Failed to get guild member:")
		return
	}

func courseChoices(db *database.Database) (choices []*discordgo.ApplicationCommandOptionChoice) {
	courses, err := db.GetCourses()
	if err != nil {
		log.Errorln("Failed to get courses:", err)
	}

	for _, course := range courses {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  course.Name,
			Value: course.Name,
		})
	}

	return choices
}
