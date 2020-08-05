package main

import (
	"bytes"

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

func helpCommand(s disgord.Session, m *disgord.MessageCreate) {
	var (
		assistant = false
		student   = false
		buf       = new(bytes.Buffer)
	)
	gm, err := s.GetMember(m.Ctx, cfg.Guild, m.Message.Author.ID)
	if err != nil {
		log.Println("Failed to get guild member:", err)
		return
	}
	for _, role := range gm.Roles {
		if role == cfg.AssistantRole {
			assistant = true
		}
		if role == cfg.StudentRole {
			student = true
		}
	}
	if assistant {
		err := assitantHelp.Execute(buf, cfg)
		if err != nil {
			log.Println("Failed to execute assitant help template:", err)
			return
		}
	} else if student {
		err := studentHelp.Execute(buf, cfg)
		if err != nil {
			log.Println("Failed to execute student help template:", err)
			return
		}
	} else {
		return
	}
	_, _, err = m.Message.Author.SendMsgString(m.Ctx, s, buf.String())
	if err != nil {
		log.Println("Failed to send help message:", err)
	}
}
