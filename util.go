package helpbot

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

// replyMsg replies to an interaction with a message.
func replyMsg(s *discordgo.Session, m *discordgo.InteractionCreate, msg string) bool {
	err := s.InteractionRespond(m.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Title:   m.ApplicationCommandData().Name,
			Content: msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Errorln("Failed to get user:", err)
		return false
	}
	return true
}

func replyModal(s *discordgo.Session, m *discordgo.InteractionCreate, resp *discordgo.InteractionResponse) bool {
	if err := s.InteractionRespond(m.Interaction, resp); err != nil {
		log.Errorln("Failed to get user:", err)
		return false
	}
	return true
}

// sendMsg sends a direct message to a user.
func sendMsg(s *discordgo.Session, u *discordgo.User, msg string) bool {
	channel, err := s.UserChannelCreate(u.ID)
	if err != nil {
		log.Errorln("Failed to create private channel:", err)
		return false
	}
	s.ChannelMessageSend(channel.ID, msg)
	return true
}

// returns mention plus member's nickname if present, username otherwise.
func getMentionAndNick(gm *discordgo.Member) string {
	name := gm.User.Username
	if gm.Nick != "" {
		name = gm.Nick
	}
	return fmt.Sprintf("%s (%s)", gm.Mention(), name)
}
