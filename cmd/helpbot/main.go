package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Raytar/helpbot"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	botName = "helpbot"
)

var log = &logrus.Logger{
	Out:       os.Stderr,
	Formatter: new(logrus.TextFormatter),
	Hooks:     make(logrus.LevelHooks),
	Level:     logrus.InfoLevel,
}

var (
	ag      *helpbot.QuickFeed
	cfgFile string
)

func main() {
	flag.StringVar(&cfgFile, "config", ".helpbotrc", "Path to configuration file")
	flag.Parse()

	fmt.Println("Starting", botName, "...")
	err := initConfig(cfgFile)
	if err != nil {
		log.Fatalln("Failed to init config:", err)
	}

	if viper.GetBool("quickfeed") {
		authToken := os.Getenv("QUICKFEED_AUTH_TOKEN")
		fmt.Println("QUICKFEED_AUTH_TOKEN:", authToken)
		if authToken == "" {
			log.Fatalln("QUICKFEED_AUTH_TOKEN is not set")
		}
		ag, err = helpbot.NewAutograder(authToken)
		if err != nil {
			log.Fatalln("Failed to init autograder:", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var bots []*helpbot.HelpBot

	for _, c := range cfg {
		bot, err := helpbot.New(c, log, ag)
		if err != nil {
			log.Fatalf("Failed to initialize bot: %v", err)
		}
		err = bot.Connect(ctx)
		if err != nil {
			log.Errorf("Failed to connect: %v", err)
			continue
		}
		bots = append(bots, bot)
	}

	fmt.Println("Running", botName, "...")
	// run until interrupted
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	// cleanup
	if ag != nil {
		for _, b := range bots {
			b.Disconnect()
		}
		cancel()
	}
}
