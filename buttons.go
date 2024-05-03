package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// Message components for default macro pads, set during init.
var (
	Dnd5ePadComponents []discordgo.MessageComponent
	FatePadComponents  []discordgo.MessageComponent
	D20PadComponents   []discordgo.MessageComponent
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
	FatePadComponents = makeModifierButtonPad("4dF", -7, 7)
	D20PadComponents = makeModifierButtonPad("d20", -7, 7)
}

func makeModifierRange(lowest, highest int) []int {
	len := highest - lowest + 1
	if len > 25 {
		panic("range too large")
	}
	modifiers := make([]int, len)
	for i := range modifiers {
		modifiers[i] = lowest + i
	}
	return modifiers
}

func makeModifierButtonPad(expression string, lowest, highest int) []discordgo.MessageComponent {
	modifiers := makeModifierRange(lowest, highest)
	maxCols := 5
	rows := (len(modifiers) + (maxCols - 1)) / maxCols

	var components = make([]discordgo.MessageComponent, rows)
	for c := range components {
		cols := math.Min(float64(maxCols), float64(len(modifiers)-(c*maxCols)))
		components[c] = discordgo.ActionsRow{Components: make([]discordgo.MessageComponent, int(cols))}
	}

	index := 0
	for i := 0; i < rows; i++ {
		for j := 0; j < maxCols; j++ {
			if index < len(modifiers) {
				var label string
				var style discordgo.ButtonStyle
				modifier := modifiers[index]
				if modifier != 0 {
					// ensure integers are signed on button labels (ex. "+3")
					label = fmt.Sprintf("%+d", modifier)
					style = discordgo.SecondaryButton
				} else {
					label = expression
					style = discordgo.PrimaryButton
				}
				expression := strings.ReplaceAll(fmt.Sprintf("%s+%d", expression, modifier), "+-", "-")

				components[i].(discordgo.ActionsRow).Components[j] = discordgo.Button{
					Label:    label,
					Style:    style,
					CustomID: "macro_" + expression,
				}
				index++
			} else {
				break
			}
		}
	}
	return components
}
