package helpbot

import (
	"context"
	"fmt"
	"strings"

	"github.com/andersfylling/disgord"
	"github.com/andersfylling/disgord/std"
)

func (bot *HelpBot) initEvents() {
	filter, _ := std.NewMsgFilter(context.Background(), bot.client)
	filter.SetPrefix(bot.cfg.Prefix)

	// create a handler and bind it to new message events
	bot.client.On(disgord.EvtMessageCreate,
		// middleware
		filter.NotByBot, // ignore bot messages
		filter.HasPrefix,
		filter.StripPrefix,
		// handler
		bot.discordMessageCreate)
}

func (bot *HelpBot) discordMessageCreate(_ disgord.Session, m *disgord.MessageCreate) {
	words := strings.Fields(m.Message.Content)
	if len(words) < 1 {
		return
	}

	gm, err := bot.client.GetMember(m.Ctx, bot.cfg.Guild, m.Message.Author.ID)
	if err != nil {
		bot.log.Infoln("messageCreate: Failed to get guild member:", err)
		return
	}

	if hasRoles(gm, bot.cfg.StudentRole) {
		if cmdFunc, ok := bot.studentCommands[words[0]]; ok {
			cmdFunc(m)
			return
		}
		goto reply
	}

	if hasRoles(gm, bot.cfg.AssistantRole) {
		if cmdFunc, ok := bot.assistantCommands[words[0]]; ok {
			cmdFunc(m)
			return
		}
		goto reply
	}

	if cmdFunc, ok := bot.baseCommands[words[0]]; ok {
		cmdFunc(m)
		return
	}

reply:
	ch, err := bot.client.GetChannel(m.Ctx, m.Message.ChannelID)
	if err != nil {
		bot.log.Errorln("Failed to get channel info:", err)
	}

	// if message is DM, then we will help the user. Otherwise, avoid spamming.
	if ch.Type == disgord.ChannelTypeDM {
		replyMsg(bot.client, m, fmt.Sprintf("'%s' is not a recognized command. See %shelp for available commands.",
			words[0], bot.cfg.Prefix))
	}
}
