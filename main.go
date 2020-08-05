package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/andersfylling/disgord"
	"github.com/andersfylling/disgord/std"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	botName = "helpbot"
	cfgFile = ".helpbotrc"
)

var (
	cfg      config
	db       *gorm.DB
	commands = make(commandMap)
	log      = &logrus.Logger{
		Out:       os.Stderr,
		Formatter: new(logrus.TextFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.ErrorLevel,
	}
)

type command func(s disgord.Session, m *disgord.MessageCreate)

type commandMap map[string]command

func (commands commandMap) Register(name string, handler command) {
	commands[cfg.Prefix+name] = handler
}

func initCommands() {
	commands.Register("help", helpCommand)
	commands.Register("ta", helpRequestCommand)
}

func main() {
	err := initConfig()
	if err != nil {
		log.Fatalln("Failed to read config:", err)
	}

	err = initDB()
	if err != nil {
		log.Fatalln("Failed to init database:", err)
	}
	defer db.Close()

	client := disgord.New(disgord.Config{
		BotToken: viper.GetString("token"),
	})

	defer func() {
		err := client.StayConnectedUntilInterrupted(context.Background())
		log.Println("Discord exited with error:", err)
	}()

	initCommands()
	filter, _ := std.NewMsgFilter(context.Background(), client)
	filter.SetPrefix("!")

	// create a handler and bind it to new message events
	// tip: read the documentation for std.CopyMsgEvt and understand why it is used here.
	client.On(disgord.EvtMessageCreate,
		// middleware
		filter.NotByBot,    // ignore bot messages
		filter.HasPrefix,   // read original
		std.CopyMsgEvt,     // read & copy original
		filter.StripPrefix, // write copy
		// handler
		discordMessageCreate) // handles copy

	client.Ready(func() {
		err := client.UpdateStatusString(fmt.Sprintf("DM me %shelp", cfg.Prefix))
		if err != nil {
			log.Println("Failed to update status:", err)
		}
	})
}

func discordMessageCreate(s disgord.Session, m *disgord.MessageCreate) {
	words := strings.Fields(m.Message.Content)
	if len(words) < 1 {
		return
	}

	if cmdFunc, ok := commands[words[0]]; ok {
		cmdFunc(s, m)
	}
}
