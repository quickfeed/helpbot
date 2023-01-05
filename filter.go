package helpbot

import "github.com/bwmarrin/discordgo"

// hasRoles filters out messages that don't contain any of the given roles.
func (bot *HelpBot) hasRoles(gm *discordgo.Member, roles ...string) bool {

	guildRoles, ok := bot.roles[gm.GuildID]
	if !ok {
		return false
	}

	if len(roles) == 0 {
		return true
	}

	roleIDs := []string{}
	for _, role := range roles {
		if id, ok := guildRoles[role]; ok {
			roleIDs = append(roleIDs, id)
		}
	}

	for _, i := range gm.Roles {
		for _, j := range roleIDs {
			if i == j {
				return true
			}
		}
	}

	return false
}

func (bot *HelpBot) GetRole(guildID, roleName string) string {
	return bot.roles[guildID][roleName]
}
