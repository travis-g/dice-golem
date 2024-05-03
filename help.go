package main

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const (
	owner     = "@travvyg"
	support   = "https://discord.gg/XUkXda5"
	id        = `581956766246633475`
	add       = `https://discord.com/api/oauth2/authorize?client_id=` + id
	directory = `discord://application-directory/` + id
	vote      = `https://top.gg/bot/` + id
	faq       = `https://dicegolem.io/faq`
	privacy   = `https://dicegolem.io/privacy`
	terms     = `https://dicegolem.io/terms`
)

var examples = strings.TrimSpace("" +
	"`2d20 + 1` - Roll two D20s and add 1.\n" +
	"`4dF` - Roll 4 Fate/Fudge dice.\n" +
	"`4d6d1` - Roll four D6s and drop the lowest die's result.\n" +
	"`3d20kl1` - Keep only the lowest result out of three D20s.\n" +
	"`2d20r1` - Roll two D20s and re-roll all 1s.\n" +
	"`2d20r<3` - Roll two D20s and re-roll any rolls of _3 or less_.\n" +
	"`d20ro1` - Roll a D20 and re-roll it only once if the result was a 1.\n" +
	"`8d6s` - Roll 8 D6s and sort the results.\n" +
	"`3d6 # Fire damage` - Add a label to a roll after a `#` or `\\`.",
)

func makeEmbedHelp() *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "Dice Golem Help",
		Description: "I roll dice! I respond to commands like `/roll d20` and @mentions.\n</info:581956766246633475> provides more bot information.",
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Examples",
				Value: examples,
			},
			{
				Name:  "Math",
				Value: "You can do basic math calculations with rolls, or by themselves. Calculations follow order of operations. Basic arithmetic operators `+-/*` are supported, as are `%` modulo and `**` exponent.",
			},
			{
				Name:  "Drop/Keep Dice",
				Value: "Drop or keep dice with the `d` and `k` modifiers. Dropped dice are excluded from results.\n`d`, `dl` - drop the lowest rolls\n`k`, `kh` - keep the highest rolls\n`dh` - drop the highest rolls\n`kl` - keep the lowest rolls",
			},
			{
				Name:  "Rerolling",
				Value: "Reroll dice with the `r` modifier. Reroll dice up to once with `ro`. Reroll by comparisons (`r<3`) or for individual possible results (`r2`). Multiple reroll modifiers can be specified (`r2r4`).",
			},
			// {
			// 	Name:  "Critical Successes/Failures",
			// 	Value: "You can override results that are treated as criticals and failures with `cs` and `cf`. Comparisons work here as well!",
			// },
			{
				Name:  "Sorting Dice",
				Value: "Sort dice of a roll with `s`.\n`s`, `sa` - sort rolls ascending\n`sd` - sort rolls descending",
			},
		},
	}
}

// InfoEmbedFields are fields embedded in info command embeds.
var InfoEmbedFields = []*discordgo.MessageEmbedField{
	{
		Name:   "Website",
		Value:  "[dicegolem.io](https://dicegolem.io)",
		Inline: true,
	},
	{
		Name:   "Source Code",
		Value:  "[github.com/travis-g/dice-golem](https://github.com/travis-g/dice-golem)",
		Inline: true,
	},
	{
		Name:   "Discord Library",
		Value:  "[DiscordGo](https://github.com/bwmarrin/discordgo)",
		Inline: true,
	},
	{
		Name: "Links",
		Value: fmt.Sprintf("[App Directory](%s) | [Support Server](%s) | [FAQ](%s) | [Info (Top.gg)](%s)\n[Privacy Policy](%s) | [Terms of Service](%s)",
			directory, support, faq, vote, privacy, terms),
	},
}

func makeEmbedInfo() *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		URL:         "https://dicegolem.io",
		Title:       "Dice Golem",
		Description: fmt.Sprintf("A simple, easy to use Discord bot for rolling RPG/TRPG dice. Dice rolls are made using a CSPRNG to ensure the results are completely random.\nDice Golem responds to Slash commands even in DMs! Use the %s command for help and examples.", CommandMention("help")),
		Footer: &discordgo.MessageEmbedFooter{
			Text:    fmt.Sprintf("Built with ❤️ and 🎲 by %s", owner),
			IconURL: DiceGolem.User.AvatarURL("64"),
		},
		Author: &discordgo.MessageEmbedAuthor{},
		Fields: InfoEmbedFields,
	}
}
