package main

import "github.com/andersfylling/disgord"

// hasRoles filters out messages that don't contain any of the given roles
func hasRoles(client disgord.Session, gm *disgord.Member, roleIDs ...disgord.Snowflake) bool {
	for _, i := range gm.Roles {
		for _, j := range roleIDs {
			if i == j {
				return true
			}
		}
	}

	return false
}
