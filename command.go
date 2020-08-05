package main

import "github.com/andersfylling/disgord"

var (
	studentCommands   = make(commandMap)
	assistantCommands = make(commandMap)
)

type command func(s disgord.Session, m *disgord.MessageCreate)

type commandMap map[string]command

func (commands commandMap) Register(name string, handler command) {
	commands[name] = handler
}

func initCommands() {
	studentCommands.Register("help", studentHelpCommand)
	studentCommands.Register("ta", helpRequestCommand)

	assistantCommands.Register("help", assistantHelpCommand)
}
