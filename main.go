package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/spf13/viper"
)

const (
	botName = "helpbot"
	cfgFile = ".helpbotrc"
)

var (
	cfg      config
	commands = make(commandMap)
)

type command func(s *discordgo.Session, m *discordgo.Message)

type commandMap map[string]command

func (commands commandMap) Register(name string, handler command) {
	commands[cfg.Prefix+name] = handler
}

func main() {
	err := initConfig()
	if err != nil {
		log.Fatalln("Failed to read config:", err)
	}

	token := viper.GetString("token")
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalln("Error creating Discord session:", err)
	}

	initCommands()

	dg.AddHandler(discordMessageCreate)
	dg.AddHandler(discordReady)

	err = dg.Open()
	if err != nil {
		log.Fatalln("Failed to open connection to discord:", err)
	}

	fmt.Println(botName, "is now running. Press CTRL-C to exit.")
	fmt.Println("Use the following link to invite the bot to your server:")
	fmt.Printf("https://discord.com/api/oauth2/authorize?client_id=%s&permissions=0&scope=bot\n", dg.State.User.ID)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	<-sc

	dg.Close()
}

func initCommands() {
	commands.Register("help", helpCommand)
}

func discordMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages from self
	if m.Author.ID == s.State.User.ID {
		return
	}

	if !strings.HasPrefix(m.Content, cfg.Prefix) {
		return
	}

	words := strings.Fields(m.Content)
	if len(words) < 1 {
		return
	}

	if cmdFunc, ok := commands[words[0]]; ok {
		cmdFunc(s, m.Message)
	}
}

func discordReady(s *discordgo.Session, r *discordgo.Ready) {
	err := s.UpdateStatus(0, "DM me !help")
	if err != nil {
		log.Println("Failed to update status:", err)
	}
}
