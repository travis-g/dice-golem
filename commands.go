package main

import "github.com/bwmarrin/discordgo"

type BotCommands struct {
	Global []*discordgo.ApplicationCommand
	Home   []*discordgo.ApplicationCommand
}

// CommandsGlobalChat are the globally-enabled Slash commands supported by the
// bot. Commands must be removed from this list before removing their handler
// functions.
var CommandsGlobalChat = []*discordgo.ApplicationCommand{
	{
		Name:        "roll",
		Description: "Roll a dice expression",
		Options:     MakeApplicationCommandOptions(rollOptionsDefault, rollOptionsDetailed, rollOptionsSecret, rollOptionsPrivate),
	},
	{
		Name:        "help",
		Description: "Show help for using Dice Golem",
	},
	{
		Name:        "info",
		Description: "Show bot information for Dice Golem",
	},
	{
		Name:        "secret",
		Description: "Make an ephemeral roll that only you will see",
		Options:     MakeApplicationCommandOptions(rollOptionsDefault, rollOptionsDetailed),
	},
	{
		Name:         "private",
		Description:  "Make a roll to have DMed to you",
		Options:      MakeApplicationCommandOptions(rollOptionsDefault, rollOptionsDetailed),
		DMPermission: Bool(false), // already private if in DMs.
	},
	{
		Name:        "clear",
		Description: "Data management commands",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "recent",
				Description: "Clear your recent roll history",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
		},
	},
	{
		Name:        "settings",
		Description: "Configure settings and preferences",
		Type:        discordgo.ApplicationCommandType(discordgo.ApplicationCommandOptionSubCommand),
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "recent",
				Description: "Suggestions based on your recent rolls",
				Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "enable",
						Description: "Enable suggestions based on your recent rolls",
						Type:        discordgo.ApplicationCommandOptionSubCommand,
					},
					{
						Name:        "disable",
						Description: "Disable suggestions based on your recent rolls",
						Type:        discordgo.ApplicationCommandOptionSubCommand,
					},
				},
			},
			{
				Name:        "detailed",
				Description: "Default roll output preference",
				Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "enable",
						Description: "Enable detailed roll output by default",
						Type:        discordgo.ApplicationCommandOptionSubCommand,
					},
					{
						Name:        "disable",
						Description: "Disable detailed roll output by default",
						Type:        discordgo.ApplicationCommandOptionSubCommand,
					},
				},
			},
			{
				Name:        "autocomplete",
				Description: "Manage autocomplete settings for the full server",
				Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "enable",
						Description: "Enable autocompletion when typing expressions",
						Type:        discordgo.ApplicationCommandOptionSubCommand,
					},
					{
						Name:        "disable",
						Description: "Disable autocompletion when typing expressions",
						Type:        discordgo.ApplicationCommandOptionSubCommand,
					},
				},
			},
		},
	},
	// TODO: break into commandsGlobalMessage[]
	{
		Name: "Roll Message",
		Type: discordgo.MessageApplicationCommand,
	},
}

// Commands to enable in the bot's home server(s).
var CommandsHomeChat = []*discordgo.ApplicationCommand{
	{
		Name:        "state",
		Description: "Show internal bot state information (Owner-only)",
	},
	{
		Name:        "stats",
		Description: "Show bot statistics (Owner-only)",
	},
	{
		Name:        "ping",
		Description: "Check response times",
	},
	{
		Name:        "buttons",
		Description: "Try a mobile-friendly dice macro pad [BETA]",
	},
	{
		Name:        "debug",
		Description: "The debug interaction handler",
	},
}

// Option sets for commands.
var (
	// options common to multiple rolling commands. First item MUST be the roll
	// input string.
	rollOptionsDefault = []*discordgo.ApplicationCommandOption{
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "expression",
			Description:  "Dice expression to roll, like '2d6+1'",
			Required:     true,
			Autocomplete: true,
		},
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "label",
			Description:  "Roll label, like 'fire damage'",
			Autocomplete: true,
		},
	}
	rollOptionsDefaultNoAutocomplete = []*discordgo.ApplicationCommandOption{
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "expression",
			Description:  "Dice expression to roll, like '2d6+1'",
			Required:     true,
			Autocomplete: false,
		},
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "label",
			Description:  "Roll label, like 'fire damage'",
			Autocomplete: false,
		},
	}
	rollOptionsDetailed = []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        "detailed",
			Description: "Include detailed results of the roll",
		},
	}
	rollOptionsSecret = []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        "secret",
			Description: "Roll as an ephemeral roll",
		},
	}
	rollOptionsPrivate = []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        "private",
			Description: "Have the result DMed to you",
		},
	}
	// helper to fetch a command option value given a name rather than an index
	getOptionByName = func(opts []*discordgo.ApplicationCommandInteractionDataOption, name string) *discordgo.ApplicationCommandInteractionDataOption {
		for _, opt := range opts {
			if opt.Name == name {
				return opt
			}
		}
		return nil
	}
	// get focused option for autocompletion
	getFocusedOption = func(data discordgo.ApplicationCommandInteractionData) *discordgo.ApplicationCommandInteractionDataOption {
		for _, opt := range data.Options {
			if opt.Focused {
				return opt
			}
		}
		return nil
	}
)
