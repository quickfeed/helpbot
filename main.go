package main

import (
	"context"
	"fmt"
	"os"

	"github.com/andersfylling/disgord"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	botName = "helpbot"
	cfgFile = ".helpbotrc"
)

var log = &logrus.Logger{
	Out:       os.Stderr,
	Formatter: new(logrus.TextFormatter),
	Hooks:     make(logrus.LevelHooks),
	Level:     logrus.ErrorLevel,
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	initGithub(ctx)

	client := disgord.New(disgord.Config{
		BotToken: viper.GetString("token"),
	})

	initEvents(client)

	defer func() {
		err := client.StayConnectedUntilInterrupted(context.Background())
		if err != nil {
			log.Errorln("Discord exited with error:", err)
		}
	}()

	client.Ready(func() {
		err := client.UpdateStatusString(fmt.Sprintf("DM me %shelp", cfg.Prefix))
		if err != nil {
			log.Errorln("Failed to update status:", err)
		}
	})
}
