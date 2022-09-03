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
		// NameLocalizations: &map[discordgo.Locale]string{
		// 	discordgo.SpanishES: "tirar",
		// },
		// DescriptionLocalizations: &map[discordgo.Locale]string{
		// 	discordgo.SpanishES: "Tirar un expressión de dados",
		// },
	},
	{
		Name:        "help",
		Description: "Show help for using Dice Golem",
		// NameLocalizations: &map[discordgo.Locale]string{
		// 	discordgo.SpanishES: "ayuda",
		// },
		// DescriptionLocalizations: &map[discordgo.Locale]string{
		// 	discordgo.SpanishES: "Mostrar ayuda para el uso de Dice Golem",
		// },
	},
	{
		Name:        "info",
		Description: "Show bot information for Dice Golem",
		// NameLocalizations: &map[discordgo.Locale]string{
		// 	discordgo.SpanishES: "información",
		// },
		// DescriptionLocalizations: &map[discordgo.Locale]string{
		// 	discordgo.SpanishES: "Mostrar información sobre Dice Golem",
		// },
	},
	{
		Name:        "secret",
		Description: "Make an ephemeral roll that only you will see",
		Options:     MakeApplicationCommandOptions(rollOptionsDefault, rollOptionsDetailed),
		// NameLocalizations: &map[discordgo.Locale]string{
		// 	discordgo.SpanishES: "secreto",
		// },
		// DescriptionLocalizations: &map[discordgo.Locale]string{
		// 	discordgo.SpanishES: "Tirar un expressión de dados que solo tu verás",
		// },
	},
	{
		Name:         "private",
		Description:  "Make a roll to have DMed to you",
		Options:      MakeApplicationCommandOptions(rollOptionsDefault, rollOptionsDetailed),
		DMPermission: Bool(false), // already private if in DMs.
		// NameLocalizations: &map[discordgo.Locale]string{
		// 	discordgo.SpanishES: "privado",
		// },
		// DescriptionLocalizations: &map[discordgo.Locale]string{
		// 	discordgo.SpanishES: "Tirar un expressión de dados en un mensaje directo",
		// },
	},
	{
		Name:        "clear",
		Description: "Data removal commands",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "recent",
				Description: "Clear your recent roll history",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
		},
		// DescriptionLocalizations: &map[discordgo.Locale]string{
		// 	discordgo.SpanishES: "Comandos de eliminación de detos",
		// },
	},
	{
		Name:        "settings",
		Description: "Configure settings and preferences",
		Type:        discordgo.ApplicationCommandType(discordgo.ApplicationCommandOptionSubCommand),
		// NameLocalizations: &map[discordgo.Locale]string{
		// 	discordgo.SpanishES: "ajustes",
		// },
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
						Description: "Enable printing detailed roll output by default",
						Type:        discordgo.ApplicationCommandOptionSubCommand,
					},
					{
						Name:        "disable",
						Description: "Disable printing detailed roll output by default",
						Type:        discordgo.ApplicationCommandOptionSubCommand,
					},
				},
			},
		},
	},
	{
		Name:        "buttons",
		Description: "Mobile-friendly dice button pads",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "dnd5e",
				Description: "A dice button pad of common D&D 5e system dice rolls",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "fate",
				Description: "A dice button pad of common Fate (and Fudge) system rolls",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
		},
		// NameLocalizations: &map[discordgo.Locale]string{
		// 	discordgo.SpanishES: "botones",
		// },
	},
	// TODO: break into commandsGlobalMessage[]
	{
		Name: "Roll Message",
		Type: discordgo.MessageApplicationCommand,
		// NameLocalizations: &map[discordgo.Locale]string{
		// 	discordgo.SpanishES: "Tirar mensaje",
		// },
	},
}

// Commands to enable in the bot's home server(s).
var CommandsHomeChat = []*discordgo.ApplicationCommand{
	{
		Name:                     "state",
		Description:              "Show internal bot state information",
		DefaultMemberPermissions: Int64(0),
		DefaultPermission:        Bool(false),
	},
	{
		Name:                     "stats",
		Description:              "Show bot statistics",
		DefaultMemberPermissions: Int64(0),
		DefaultPermission:        Bool(false),
	},
	{
		Name:                     "ping",
		Description:              "Check response times",
		DefaultMemberPermissions: Int64(0),
		DefaultPermission:        Bool(false),
	},
	// {
	// 	Name:        "macro",
	// 	Description: "Commands for saved rolls",
	// 	Options: []*discordgo.ApplicationCommandOption{
	// 		{
	// 			Name:        "set",
	// 			Description: "Save a roll with an optional name and label",
	// 			Type:        discordgo.ApplicationCommandOptionSubCommand,
	// 		},
	// 		{
	// 			Name:        "delete",
	// 			Description: "Delete a saved macro",
	// 			Type:        discordgo.ApplicationCommandOptionSubCommand,
	// 		},
	// 		{
	// 			Name:        "list",
	// 			Description: "List your saved macros",
	// 			Type:        discordgo.ApplicationCommandOptionSubCommand,
	// 		},
	// 		// {
	// 		// 	Name:        "import",
	// 		// 	Description: "Import a set of saved rolls",
	// 		// 	Type:        discordgo.ApplicationCommandOptionSubCommand,
	// 		// },
	// 		// {
	// 		// 	Name:        "export",
	// 		// 	Description: "Export your list of saved rolls",
	// 		// 	Type:        discordgo.ApplicationCommandOptionSubCommand,
	// 		// },
	// 	},
	// },
	// {
	// 	Name:        "debug",
	// 	Description: "The debug interaction handler",
	// },
	// {
	// 	Name: "Save Roll",
	// 	Type: discordgo.MessageApplicationCommand,
	// 	NameLocalizations: &map[discordgo.Locale]string{
	// 		discordgo.SpanishES: "Guardar tira",
	// 	},
	// },
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
	rollOptionsName = []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        "name",
			Description: "Friendly name of the roll",
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
