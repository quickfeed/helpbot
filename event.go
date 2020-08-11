package main

import (
	"context"
	"fmt"
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

	if hasRoles(gm, cfg.StudentRole) {
		if cmdFunc, ok := studentCommands[words[0]]; ok {
			cmdFunc(s, m)
			return
		}
		goto reply
	}

	if hasRoles(gm, cfg.AssistantRole) {
		if cmdFunc, ok := assistantCommands[words[0]]; ok {
			cmdFunc(s, m)
			return
		}
		goto reply
	}

	if cmdFunc, ok := baseCommands[words[0]]; ok {
		cmdFunc(s, m)
		return
	}

reply:
	ch, err := s.GetChannel(m.Ctx, m.Message.ChannelID)
	if err != nil {
		log.Errorln("Failed to get channel info:", err)
	}

	// if message is DM, then we will help the user. Otherwise, avoid spamming.
	if ch.Type == disgord.ChannelTypeDM {
		replyMsg(s, m, fmt.Sprintf("'%s' is not a recognized command. See %shelp for available commands.",
			words[0], cfg.Prefix))
	}
}
