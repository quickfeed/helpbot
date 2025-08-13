package helpbot

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/Raytar/helpbot/database"
	"github.com/Raytar/helpbot/models"
	"github.com/bwmarrin/discordgo"
	qfpb "github.com/quickfeed/quickfeed/qf"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Token     string `json:"token"`
	DBPath    string `json:"database"`
	AppID     string `json:"app_id"`
	GHToken   string `json:"auth_token"`
	QuickFeed bool   `json:"quickfeed"`
}

type HelpBot struct {
	cfg     Config
	client  *discordgo.Session
	db      *database.Database
	qf      *QuickFeed
	log     *logrus.Logger
	courses []*qfpb.Course

	// role mappings
	roles map[string]map[string]string

	// command mappings. key is the command name, value is the function to call
	commands commandMap
}

func (bot *HelpBot) Connect(ctx context.Context) error {
	if bot.client == nil {
		return fmt.Errorf("Discord client is not initialized")
	}
	return bot.client.Open()
}

func (bot *HelpBot) Disconnect() error {
	return bot.client.Close()
}

func GetCommands(course *models.Course) []*discordgo.ApplicationCommand {
	courseChoices := []*discordgo.ApplicationCommandOptionChoice{
		{
			Name:  fmt.Sprintf("%s %d", course.Name, course.Year),
			Value: fmt.Sprintf("%d", course.CourseID),
		},
	}

	return []*discordgo.ApplicationCommand{
		{
			Name:        "register",
			Description: "Register using your GitHub username",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "username",
					Type:        discordgo.ApplicationCommandOptionString,
					Description: "your GitHub username",
					Required:    true,
				},
				{
					Name:        "course",
					Type:        discordgo.ApplicationCommandOptionString,
					Description: "the course you want to register for.",
					Required:    true,
					Choices:     courseChoices,
				},
			},
		},

		{
			Name:        "help",
			Description: "Get a list of all commands.",
		},
		{
			Name:                     "unregister",
			Description:              "Unregister from a course.",
			DefaultMemberPermissions: &permStudent,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "course",
					Type:        discordgo.ApplicationCommandOptionString,
					Description: "the course you want to unregister from.",
					Required:    true,
					Choices:     courseChoices,
				},
			},
		},
		{
			Name:                     "gethelp",
			DefaultMemberPermissions: &permStudent,
			Description:              "Get help from a teaching assistant.",
		},
		{
			Name:                     "approve",
			DefaultMemberPermissions: &permStudent,
			Description:              "Get your lab approved by a teaching assistant.",
		},
		{
			Name:                     "cancel",
			DefaultMemberPermissions: &permStudent,
			Description:              "Cancels a pending request for help and removes you from the queue.",
		},
		{
			Name:                     "status",
			DefaultMemberPermissions: &permStudent,
			Description:              "Get the status of your help request.",
		},
		{
			Name:                     "list",
			DefaultMemberPermissions: &permAssistant,
			Description:              "List <number> of students in the queue. If no number is given, all students in the queue are listed. ",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "number",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Description: "the number of students to list",
					Required:    false,
				},
			},
		},
		{
			Name:                     "next",
			DefaultMemberPermissions: &permAssistant,
			Description:              "Get the next student in the queue.",
		},
		{
			Name:                     "clear",
			DefaultMemberPermissions: &permAssistant,
			Description:              "Clear the queue of all students waiting for help.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "confirm",
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Description: "Confirm that you want to clear the queue. This cannot be undone.",
					Required:    true,
				},
			},
		},
		{
			Name:                     "config",
			Description:              "Configure this server with a course.",
			DefaultMemberPermissions: &permAdmin,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "course",
					Type:        discordgo.ApplicationCommandOptionString,
					Description: "the course you want to configure.",
					Required:    true,
					Choices:     courseChoices,
				},
			},
		},
	}
}

var (
	// No permissions
	NoPermission int64 = 0
	// https://discord.com/developers/docs/topics/permissions
	basePermissions int64 = discordgo.PermissionViewChannel |
		discordgo.PermissionSendMessages |
		discordgo.PermissionEmbedLinks |
		discordgo.PermissionAttachFiles |
		discordgo.PermissionAddReactions |
		discordgo.PermissionUseExternalEmojis |
		discordgo.PermissionReadMessageHistory |
		discordgo.PermissionUseSlashCommands |
		discordgo.PermissionVoiceConnect |
		discordgo.PermissionVoiceSpeak |
		discordgo.PermissionVoiceStreamVideo |
		discordgo.PermissionCreatePublicThreads |
		discordgo.PermissionSendMessagesInThreads
	// Student permissions
	permStudent int64 = basePermissions
	// Teaching assistant permissions
	permAssistant int64 = basePermissions |
		discordgo.PermissionManageNicknames |
		discordgo.PermissionManageRoles |
		discordgo.PermissionManageMessages |
		discordgo.PermissionKickMembers |
		discordgo.PermissionMentionEveryone |
		discordgo.PermissionVoiceMoveMembers |
		discordgo.PermissionManageThreads
	permAdmin int64 = discordgo.PermissionAdministrator
)

func New(cfg Config, log *logrus.Logger, qf *QuickFeed) (bot *HelpBot, err error) {
	bot = &HelpBot{cfg: cfg, log: log, qf: qf, roles: make(map[string]map[string]string)}

	if bot.client, err = discordgo.New("Bot " + cfg.Token); err != nil {
		return nil, err
	}

	if bot.db, err = database.OpenDatabase(cfg.DBPath, log); err != nil {
		return nil, err
	}

	if courses, err := bot.qf.qf.GetCourses(context.Background(), &connect.Request[qfpb.Void]{}); err != nil {
		return nil, err
	} else {
		// Update the list of courses in the database
		if err := bot.db.UpdateCourses(courses.Msg.GetCourses()); err != nil {
			return nil, err
		}
		bot.courses = courses.Msg.GetCourses()
	}

	bot.client.AddHandler(func(s *discordgo.Session, h *discordgo.Ready) {
		if err := s.UpdateGameStatus(0, "Type '/' in chat to see available commands"); err != nil {
			log.Errorln("Failed to update status:", err)
		}
	})

	bot.initCommands()
	bot.initEvents()

	return bot, nil
}
