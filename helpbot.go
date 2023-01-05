package helpbot

import (
	"context"
	"fmt"

	"github.com/bufbuild/connect-go"
	"github.com/bwmarrin/discordgo"
	"github.com/google/go-github/v32/github"
	"github.com/jinzhu/gorm"
	qfpb "github.com/quickfeed/quickfeed/qf"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Token         string `mapstructure:"token"`
	DBPath        string `mapstructure:"db-path"`
	AppID         string `mapstructure:"app-id"`
	GHToken       string `mapstructure:"gh-token"`
	Prefix        string `mapstructure:"prefix"`
	Guild         string `mapstructure:"guild"`
	StudentRole   string `mapstructure:"student-role"`
	AssistantRole string `mapstructure:"assistant-role"`
	GitHubOrg     string `mapstructure:"gh-org"`
	CourseCode    string `mapstructure:"course-code"`
	CourseYear    uint32 `mapstructure:"course-year"`
	QuickFeed     bool   `mapstructure:"quickfeed"`
}

type HelpBot struct {
	cfg     Config
	client  *discordgo.Session
	db      *gorm.DB
	gh      *github.Client
	qf      *QuickFeed
	log     *logrus.Logger
	courses []*qfpb.Course

	// role mappings
	roles map[string]map[string]string

	// role to command mappings
	baseCommands      commandMap
	studentCommands   commandMap
	assistantCommands commandMap
}

func (bot *HelpBot) Connect(ctx context.Context) error {
	bot.gh = initGithub(ctx, bot.cfg.GHToken)
	if bot.client == nil {
		return fmt.Errorf("disgord client not initialized for course: %s", bot.cfg.CourseCode)
	}
	return bot.client.Open()
}

func (bot *HelpBot) Disconnect() error {
	return bot.client.Close()
}

func GetCommands(courses []*qfpb.Course) []*discordgo.ApplicationCommand {
	var courseChoices []*discordgo.ApplicationCommandOptionChoice
	for _, course := range courses {
		courseChoices = append(courseChoices, &discordgo.ApplicationCommandOptionChoice{
			Name:  course.Name,
			Value: course.Code,
		})
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
			Name:        "unregister",
			Description: "Unregister from a course.",
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
			Name:        "gethelp",
			Description: "Get help from a teaching assistant.",
		},
		{
			Name:        "approve",
			Description: "Get your lab approved by a teaching assistant.",
		},
		{
			Name:        "cancel",
			Description: "Cancels a pending request for help and removes you from the queue.",
		},
		{
			Name:        "status",
			Description: "Get the status of your help request.",
		},
		{
			Name:        "list",
			Description: "List <number> of students in the queue. If no number is given, all students in the queue are listed. ",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "number",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Description: "the number of students to list",
					Required:    false,
				},
			},
		},
	}
}

var (
	// No permissions
	NoPermission int64 = 0
)

func New(cfg Config, log *logrus.Logger, qf *QuickFeed) (bot *HelpBot, err error) {
	bot = &HelpBot{cfg: cfg, log: log, qf: qf, roles: make(map[string]map[string]string)}

	if bot.client, err = discordgo.New("Bot " + cfg.Token); err != nil {
		return nil, err
	}

	if bot.db, err = OpenDatabase(cfg.DBPath); err != nil {
		return nil, err
	}

	if courses, err := bot.qf.qf.GetCourses(context.Background(), &connect.Request[qfpb.Void]{}); err != nil {
		return nil, err
	} else {
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
