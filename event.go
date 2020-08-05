package main

import (
	"context"
	"strings"

	"github.com/andersfylling/disgord"
	"github.com/andersfylling/disgord/std"
)

func initEvents(client disgord.Session) {
	filter, _ := std.NewMsgFilter(context.Background(), client)
	filter.SetPrefix(cfg.Prefix)

	// create a handler and bind it to new message events
	client.On(disgord.EvtMessageCreate,
		// middleware
		filter.NotByBot, // ignore bot messages
		filter.HasPrefix,
		filter.StripPrefix,
		// handler
		discordMessageCreate)
}

func discordMessageCreate(s disgord.Session, m *disgord.MessageCreate) {
	words := strings.Fields(m.Message.Content)
	if len(words) < 1 {
		return
	}

	gm, err := s.GetMember(m.Ctx, cfg.Guild, m.Message.Author.ID)
	if err != nil {
		log.Infoln("messageCreate: Failed to get guild member:", err)
		return
	}

	if hasRoles(s, gm, cfg.StudentRole) {
		if cmdFunc, ok := studentCommands[words[0]]; ok {
			cmdFunc(s, m)
		}
	}

	if hasRoles(s, gm, cfg.AssistantRole) {
		if cmdFunc, ok := assistantCommands[words[0]]; ok {
			cmdFunc(s, m)
		}
	}
}
