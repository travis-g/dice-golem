package main

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

type BotCommands struct {
	Global []*discordgo.ApplicationCommand
	Home   []*discordgo.ApplicationCommand
}

// Defaults for command settings.
var (
	defaultIntegrationTypes = []discordgo.IntegrationType{discordgo.GuildInstallIntegrationType, discordgo.UserInstallIntegrationType}
	defaultContextTypes     = []discordgo.ContextType{discordgo.GuildContextType, discordgo.BotDMContextType, discordgo.PrivateChannelContextType}
	buttonsContextTypes     = []discordgo.ContextType{discordgo.GuildContextType, discordgo.BotDMContextType}
)

// CommandsGlobalChat are the globally-enabled Slash commands supported by the
// bot. Commands must be removed from this list before removing their handler
// functions.
var CommandsGlobalChat = []*discordgo.ApplicationCommand{
	{
		Name:             "roll",
		Description:      "Roll a dice expression",
		IntegrationTypes: defaultIntegrationTypes,
		Contexts:         defaultContextTypes,
		Options:          MergeApplicationCommandOptions(rollOptionsDefault, rollOptionsDetailed, rollOptionsSecret, rollOptionsPrivate),
		NameLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "tirar",
		},
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "Tirar un expressión de dados",
		},
	},
	{
		Name:             "help",
		Description:      "Show help for using Dice Golem.",
		IntegrationTypes: defaultIntegrationTypes,
		Contexts:         defaultContextTypes,
		NameLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "ayuda",
		},
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "Mostrar ayuda para el uso de Dice Golem.",
		},
	},
	{
		Name:             "info",
		Description:      "Show bot information for Dice Golem.",
		IntegrationTypes: defaultIntegrationTypes,
		Contexts:         defaultContextTypes,
		NameLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "información",
		},
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "Mostrar información sobre Dice Golem.",
		},
	},
	{
		Name:             "secret",
		Description:      "Make an ephemeral roll that only you will see",
		IntegrationTypes: defaultIntegrationTypes,
		Contexts:         defaultContextTypes,
		Options:          MergeApplicationCommandOptions(rollOptionsDefault, rollOptionsDetailed),
		NameLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "secreto",
		},
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "Tirar un expressión de dados que solo tu verás",
		},
	},
	{
		Name:             "private",
		Description:      "Make a roll to have DMed to you",
		IntegrationTypes: defaultIntegrationTypes,
		Contexts:         []discordgo.ContextType{discordgo.GuildContextType, discordgo.PrivateChannelContextType},
		Options:          MergeApplicationCommandOptions(rollOptionsDefault, rollOptionsDetailed),
		NameLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "privado",
		},
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "Tirar un expressión de dados en un mensaje directo",
		},
	},
	{
		Name:             "clear",
		Description:      "Data removal commands",
		IntegrationTypes: defaultIntegrationTypes,
		Contexts:         defaultContextTypes,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "recent",
				Description: "Clear all of your recent roll history.",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "expressions",
				Description: "Clear all of your saved roll exressions.",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
		},
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "Comandos de eliminación de detos",
		},
	},
	{
		Name:             "preferences",
		Description:      "Configure your preferences",
		IntegrationTypes: defaultIntegrationTypes,
		Contexts:         defaultContextTypes,
		Type:             discordgo.ApplicationCommandType(discordgo.ApplicationCommandOptionSubCommand),
		NameLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "preferencias",
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "recent",
				Description: "Suggestions based on your recent rolls",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Name:        "enabled",
						Description: "Enable suggestions based on your recent rolls",
						Required:    true,
					},
				},
			},
			{
				Name:        "output",
				Description: "Roll output preferences",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Name:        "detailed",
						Description: "Prefer detailed roll output by default",
						Required:    true,
					},
				},
			},
		},
	},
	{
		Name:             "buttons",
		Description:      "Mobile-friendly dice buttons and macro pads",
		IntegrationTypes: defaultIntegrationTypes,
		Contexts:         buttonsContextTypes,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "dnd5e",
				Description: "Common D&D 5e system dice rolls.",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "fate",
				Description: "Common Fate (and Fudge) system rolls.",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "d20",
				Description: "Modifiers for a D20 roll.",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "modifiers",
				Description: "Modifiers for a base expression",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:         discordgo.ApplicationCommandOptionString,
						Name:         "expression",
						Description:  "Base expression for the button pad, like '2d6+1'",
						Required:     true,
						Autocomplete: true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "lowest",
						Description: "Lowest integer modifier for the button pad, like '-3'",
						MinValue:    Ptr[float64](-24),
						MaxValue:    float64(0),
					},
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "highest",
						Description: "Highest integer modifier for the button pad, like '10'",
						MinValue:    Ptr[float64](0),
						MaxValue:    float64(24),
					},
				},
				NameLocalizations: map[discordgo.Locale]string{
					discordgo.SpanishES: "modificadores",
				},
			},
		},
		NameLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "botones",
		},
	},
	{
		Name:             "expressions",
		Description:      "Save and manage dice expressions",
		IntegrationTypes: defaultIntegrationTypes,
		Contexts:         defaultContextTypes,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "save",
				Description: "Save an expression with an optional name and label",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options:     MergeApplicationCommandOptions(rollOptionsDefault, rollOptionsName),
			},
			{
				Name:        "unsave",
				Description: "Remove a saved expression",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: MergeApplicationCommandOptions([]*discordgo.ApplicationCommandOption{
					{
						Type:         discordgo.ApplicationCommandOptionString,
						Name:         "expression",
						Description:  "Saved expression to remove",
						Autocomplete: true,
						Required:     true,
					},
				}),
			},
			{
				Name:        "edit",
				Description: "Edit your saved expressions (experimental)",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "export",
				Description: "Export your saved expressions to a CSV.",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "clear",
				Description: "Clear all of your saved roll exressions.",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
		},
		NameLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "expresiones",
		},
	},
	{
		Name:                     "ping",
		Description:              "View response times.",
		IntegrationTypes:         defaultIntegrationTypes,
		DefaultMemberPermissions: Ptr(int64(discordgo.PermissionManageServer)),
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "Ver tiempos de respuesta",
		},
	},
	{
		Name:             "add",
		IntegrationTypes: defaultIntegrationTypes,
		Contexts:         defaultContextTypes,
		Description:      "Add the bot to a server or your account.",
		NameLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "añadir",
		},
		DescriptionLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "Añadir el bot a un servidor o a tu cuenta",
		},
	},
	{
		Name:             "Roll Message",
		Type:             discordgo.MessageApplicationCommand,
		IntegrationTypes: defaultIntegrationTypes,
		Contexts:         defaultContextTypes,
		NameLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "Tirar Mensaje",
		},
	},
	{
		Name:             "Save Expression",
		Type:             discordgo.MessageApplicationCommand,
		IntegrationTypes: defaultIntegrationTypes,
		Contexts:         defaultContextTypes,
		NameLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "Guardar Tira",
		},
	},
}

// Commands to enable in the bot's home server(s).
var CommandsHomeChat = []*discordgo.ApplicationCommand{
	{
		Name:                     "health",
		Description:              "Show bot health information.",
		DefaultMemberPermissions: Ptr(int64(discordgo.PermissionAdministrator)),
	},
	{
		Name:                     "stats",
		Description:              "Show bot statistics.",
		DefaultMemberPermissions: Ptr(int64(discordgo.PermissionAdministrator)),
	},
	{
		Name:                     "settings",
		Description:              "Server settings commands (experimental)",
		DMPermission:             Ptr(false),
		DefaultMemberPermissions: Ptr(int64(discordgo.PermissionManageServer)),
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "forward",
				Description: "Configure roll forwarding for the current channel",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:         "channel",
						Description:  "Destination channel",
						Type:         discordgo.ApplicationCommandOptionChannel,
						ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText},
						Required:     true,
					},
				},
			},
		},
		NameLocalizations: &map[discordgo.Locale]string{
			discordgo.SpanishES: "ajustes",
		},
	},
	{
		Name:                     "golemancy",
		Description:              "Bot control commands",
		DefaultMemberPermissions: Ptr(int64(discordgo.PermissionAdministrator)),
		DMPermission:             Ptr(false),
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "restart",
				Description: "Close and reopen bot connections to Discord",
				Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "session",
						Description: "Close and reopen a shard's session",
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Options: []*discordgo.ApplicationCommandOption{
							{
								Name:        "id",
								Description: "ID of the target shard",
								Type:        discordgo.ApplicationCommandOptionInteger,
								Required:    true,
							},
						},
					},
				},
			},
		},
	},
	{
		Name:                     "debug",
		Description:              "The debug interaction handler",
		DefaultMemberPermissions: Ptr(int64(discordgo.PermissionAdministrator)),
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
			NameLocalizations: map[discordgo.Locale]string{
				discordgo.SpanishES: "privado",
			},
		},
	}
	rollOptionsName = []*discordgo.ApplicationCommandOption{
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "name",
			Description:  "Name for the expression, like 'Fireball'",
			Autocomplete: true,
		},
	}
	// helper to fetch a command option value given a name rather than an index
	getOptionByName = func(opts []*discordgo.ApplicationCommandInteractionDataOption, name string) *discordgo.ApplicationCommandInteractionDataOption {
		for _, opt := range opts {
			if opt.Name == name {
				return opt
			}
			for _, subOpt := range opt.Options {
				if subOpt.Name == name {
					return subOpt
				}
			}
		}
		return nil
	}
	// getOptionByName, but panics if the option is not found
	mustGetOptionByName = func(opts []*discordgo.ApplicationCommandInteractionDataOption, name string) *discordgo.ApplicationCommandInteractionDataOption {
		opt := getOptionByName(opts, name)
		if opt == nil {
			panic("nil option")
		}
		return opt
	}
	getModalTextInputComponents = func(modal discordgo.ModalSubmitInteractionData) map[string]interface{} {
		data := make(map[string]interface{})
		for _, irow := range modal.Components {
			row := irow.(*discordgo.ActionsRow)
			for _, ifield := range row.Components {
				field, ok := ifield.(*discordgo.TextInput)
				if ok {
					data[field.CustomID] = field.Value
				}
			}
		}
		return data
	}
)

// get focused option and path in the command tree
func getFocusedOption(data discordgo.ApplicationCommandInteractionData) (option *discordgo.ApplicationCommandInteractionDataOption, path string) {
	var param strings.Builder
	param.WriteString(data.Name)
	for _, opt := range data.Options {
		if opt.Focused {
			param.WriteString(":" + opt.Name)
			return opt, param.String()
		}
		for _, subOpt := range opt.Options {
			if subOpt.Focused {
				param.WriteString(" " + opt.Name)
				param.WriteString(":" + subOpt.Name)
				return subOpt, param.String()
			}
		}
	}
	return nil, param.String()
}

func getApplicationCommandPaths(data discordgo.ApplicationCommandInteractionData) []string {
	path := []string{data.Name}
	for _, opt := range data.Options {
		if opt.Type == discordgo.ApplicationCommandOptionSubCommandGroup || opt.Type == discordgo.ApplicationCommandOptionSubCommand {
			path = append(path, opt.Name)
			if opt.Type == discordgo.ApplicationCommandOptionSubCommandGroup {
				path = append(path, opt.Options[0].Name)
			}
		}
	}
	return path
}
