package main

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func userHasRole(s *discordgo.Session, userID, roleID string) (bool, error) {
	gm, err := s.GuildMember(cfg.Guild, userID)
	if err != nil {
		return false, fmt.Errorf("userHasRole(): failed to get guild member: %w", err)
	}
	for _, r := range gm.Roles {
		if r == roleID {
			return true, nil
		}
	}
	return false, nil
}

func userIsStudent(s *discordgo.Session, userID string) (bool, error) {
	return userHasRole(s, userID, cfg.StudentRole)
}

func userIsAssitant(s *discordgo.Session, userID string) (bool, error) {
	return userHasRole(s, userID, cfg.AssistantRole)
}
