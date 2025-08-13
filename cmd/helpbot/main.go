package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Raytar/helpbot"
	"github.com/sirupsen/logrus"
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

func main() {
	var cfgFile string
	flag.StringVar(&cfgFile, "config", "config.json", "Path to configuration file")
	flag.Parse()

	config, err := loadConfig(cfgFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if config.GHToken == "" {
		log.Fatalln("QUICKFEED_AUTH_TOKEN is not set")
	}
	qf, err := helpbot.NewQuickFeed(config.GHToken)
	if err != nil {
		log.Fatalln("Failed to init autograder:", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bot, err := helpbot.New(*config, log, qf)
	if err != nil {
		log.Fatalf("Failed to initialize bot: %v with config %v", err, config)
	}
	err = bot.Connect(ctx)
	if err != nil {
		log.Errorf("Failed to connect: %v", err)
		return
	}

	// run until interrupted
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	// cleanup
}

func loadConfig(cfgFile string) (config *helpbot.Config, err error) {
	file, err := os.Open(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %s: %w", cfgFile, err)
	}
	defer file.Close()

	config = &helpbot.Config{}
	if err := json.NewDecoder(file).Decode(config); err != nil {
		return nil, fmt.Errorf("failed to decode config file %s: %w", cfgFile, err)
	}
	return config, nil
}
