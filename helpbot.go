package helpbot

import (
	"context"
	"fmt"

	"github.com/andersfylling/disgord"
	"github.com/google/go-github/v32/github"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Token         string            `mapstructure:"token"`
	DBPath        string            `mapstructure:"db-path"`
	GHToken       string            `mapstructure:"gh-token"`
	Prefix        string            `mapstructure:"prefix"`
	Guild         disgord.Snowflake `mapstructure:"guild"`
	StudentRole   disgord.Snowflake `mapstructure:"student-role"`
	AssistantRole disgord.Snowflake `mapstructure:"assistant-role"`
	GitHubOrg     string            `mapstructure:"gh-org"`
	CourseCode    string            `mapstructure:"course-code"`
	CourseYear    uint32            `mapstructure:"course-year"`
	Autograder    bool              `mapstructure:"autograder"`
}

type HelpBot struct {
	cfg    Config
	client *disgord.Client
	db     *gorm.DB
	gh     *github.Client
	ag     *Autograder
	log    *logrus.Logger

	// role to command mappings
	baseCommands      commandMap
	studentCommands   commandMap
	assistantCommands commandMap
}

func (bot *HelpBot) Connect(ctx context.Context) error {
	bot.gh = initGithub(ctx, bot.cfg.GHToken)
	return bot.client.Connect(ctx)
}

func (bot *HelpBot) Disconnect() error {
	return bot.client.Disconnect()
}

func New(cfg Config, log *logrus.Logger, ag *Autograder) (bot *HelpBot, err error) {
	bot = &HelpBot{cfg: cfg, log: log, ag: ag}
	bot.client = disgord.New(disgord.Config{BotToken: cfg.Token})
	bot.db, err = OpenDatabase(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	bot.initCommands()
	bot.initEvents()

	bot.client.Ready(func() {
		err := bot.client.UpdateStatusString(fmt.Sprintf("DM me %shelp", cfg.Prefix))
		if err != nil {
			log.Errorln("Failed to update status:", err)
		}
	})

	return bot, nil
}
