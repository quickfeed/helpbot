package helpbot

import "github.com/bwmarrin/discordgo"

func createModal(name, description string, withPrivacy bool) *discordgo.InteractionResponse {

	embeds := []*discordgo.MessageEmbed{
		{
			Title:       name,
			Color:       0x00ff00,
			Description: description,
		},
	}

	if withPrivacy {
		embeds = append(embeds, &discordgo.MessageEmbed{
			Title: ":mega: Data collection and privacy :mega:",
			Color: 0xff0000,
			Description: `
By using this bot, you consent that your:
- Full name
- Student ID
- Discord ID
may be collected for the purposes of identifying you on this server.
If you wish to delete your data, please contact a teaching assistant.
`,
		})
	}

	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: embeds,
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	}
}
