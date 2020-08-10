package main

import (
	"context"

	"github.com/andersfylling/disgord"
)

// replyMsg sends a direct message response to the author of the message
func replyMsg(s disgord.Session, m *disgord.MessageCreate, msg string) bool {
	return sendMsg(m.Ctx, s, m.Message.Author, msg)
}

// sendMsg sends a direct message to a user
func sendMsg(ctx context.Context, s disgord.Session, u *disgord.User, msg string) bool {
	_, _, err := u.SendMsgString(ctx, s, msg)
	if err != nil {
		log.Errorln("Sending message failed:", err)
		return false
	}
	return true
}

// returns member's nickname if present, username otherwise
func getMemberName(gm *disgord.Member) string {
	if gm.Nick != "" {
		return gm.Nick
	}
	return gm.User.Username
}
