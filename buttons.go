package main

import "github.com/bwmarrin/discordgo"

// Dnd5ePadComponents is the set of message components of the default macro
// pad.
var (
	Dnd5ePadComponents []discordgo.MessageComponent
	FatePadComponents  []discordgo.MessageComponent
)

func init() {
	Dnd5ePadComponents = []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "d4",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_d4",
				},
				discordgo.Button{
					Label:    "d6",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_d6",
				},
				discordgo.Button{
					Label:    "d8",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_d8",
				},
				discordgo.Button{
					Label:    "d10",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_d10",
				},
				discordgo.Button{
					Label:    "d12",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_d12",
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "2d4",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_2d4",
				},
				discordgo.Button{
					Label:    "2d6",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_2d6",
				},
				discordgo.Button{
					Label:    "2d8",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_2d8",
				},
				discordgo.Button{
					Label:    "2d10",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_2d10",
				},
				discordgo.Button{
					Label:    "2d12",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_2d12",
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "3d4",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_3d4",
				},
				discordgo.Button{
					Label:    "3d6",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_3d6",
				},
				discordgo.Button{
					Label:    "3d8",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_3d8",
				},
				discordgo.Button{
					Label:    "3d10",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_3d10",
				},
				discordgo.Button{
					Label:    "3d12",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_3d12",
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "d20",
					CustomID: "macro_d20",
				},
				discordgo.Button{
					Label:    "ADV",
					Style:    discordgo.SuccessButton,
					CustomID: "macro_2d20k1|d20 (ADV)",
				},
				discordgo.Button{
					Label:    "DIS",
					Style:    discordgo.DangerButton,
					CustomID: "macro_2d20kl1|d20 (DIS)",
				},
				discordgo.Button{
					Label:    "2d20",
					CustomID: "macro_2d20",
					Style:    discordgo.SecondaryButton,
				},
				discordgo.Button{
					Label:    "d100",
					CustomID: "macro_d100",
					Style:    discordgo.SecondaryButton,
				},
			},
		},
	}
	FatePadComponents = []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "-2",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_4dF-2",
				},
				discordgo.Button{
					Label:    "-1",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_4dF-1",
				},
				discordgo.Button{
					Label:    "4dF",
					CustomID: "macro_4dF",
				},
				discordgo.Button{
					Label:    "+1",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_4dF+1",
				},
				discordgo.Button{
					Label:    "+2",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_4dF+2",
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "+3",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_4dF+3",
				},
				discordgo.Button{
					Label:    "+4",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_4dF+4",
				},
				discordgo.Button{
					Label:    "+5",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_4dF+5",
				},
				discordgo.Button{
					Label:    "+6",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_4dF+6",
				},
				discordgo.Button{
					Label:    "+7",
					Style:    discordgo.SecondaryButton,
					CustomID: "macro_4dF+7",
				},
			},
		},
	}
}
