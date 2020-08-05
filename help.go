package main

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/andersfylling/disgord"
)

var studentHelp = createTemplate("studentHelp", `Available commands:
`+"```"+`
{{.Prefix}}help: Shows this help text
{{.Prefix}}ta:   Request help from a teaching assistant
{{.Prefix}}aha:  Remove yourself from the queue
`+"```"+`
After requesting help, you can check the response message you got to see your position in the queue.
You will receive a message when you are next in queue.
Before you can be contacted by a teaching assitant, you must connect to the {{ch .LobbyChannel}} channel.
`)

var assitantHelp = createTemplate("assistantHelp", `Teaching Assistant commands:
`+"```"+`
{{.Prefix}}queue lenght:     Returns the number of students waiting for help.
{{.Prefix}}queue list <num>: Lists the next <num> students in the queue.
{{.Prefix}}next:             Removes and returns the first student from the queue.
{{.Prefix}}clear:            Clears the queue!
`+"```")

func helpCommand(s disgord.Session, m *disgord.MessageCreate, helpTmpl *template.Template) error {
	buf := new(bytes.Buffer)
	err := helpTmpl.Execute(buf, cfg)
	if err != nil {
		return fmt.Errorf("helpCommand: failed to execute template: %w", err)
	}
	_, _, err = m.Message.Author.SendMsgString(m.Ctx, s, buf.String())
	if err != nil {
		return fmt.Errorf("helpCommand: failed to send help message: %w", err)
	}
	return nil
}

func studentHelpCommand(s disgord.Session, m *disgord.MessageCreate) {
	err := helpCommand(s, m, studentHelp)
	if err != nil {
		log.Error(err)
	}
}

func assistantHelpCommand(s disgord.Session, m *disgord.MessageCreate) {
	err := helpCommand(s, m, assitantHelp)
	if err != nil {
		log.Error(err)
	}
}
