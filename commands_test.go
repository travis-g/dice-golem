package main

import (
	"reflect"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func Test_getFocusedOption(t *testing.T) {
	type args struct {
		data discordgo.ApplicationCommandInteractionData
	}

	expressionOption := discordgo.ApplicationCommandInteractionDataOption{
		Name:    "expression",
		Focused: true,
	}
	nameOption := discordgo.ApplicationCommandInteractionDataOption{
		Name:    "name",
		Focused: true,
	}

	tests := []struct {
		name       string
		args       args
		wantOption *discordgo.ApplicationCommandInteractionDataOption
		wantPath   string
	}{
		{
			name: "roll expression",
			args: args{
				discordgo.ApplicationCommandInteractionData{
					Name: "roll",
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						&expressionOption,
					},
				},
			},
			wantOption: &expressionOption,
			wantPath:   "roll:expression",
		},
		{
			name: "expressions save name",
			args: args{
				discordgo.ApplicationCommandInteractionData{
					Name: "expressions",
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{
							Name: "save",
							Type: discordgo.ApplicationCommandOptionSubCommand,
							Options: []*discordgo.ApplicationCommandInteractionDataOption{
								{
									Name: "expression",
								},
								&nameOption,
							},
						},
					},
				},
			},
			wantOption: &nameOption,
			wantPath:   "expressions save:name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOption, gotPath := getFocusedOption(tt.args.data)
			if !reflect.DeepEqual(gotOption, tt.wantOption) {
				t.Errorf("getFocusedOption() gotOption = %v, want %v", gotOption, tt.wantOption)
			}
			if gotPath != tt.wantPath {
				t.Errorf("getFocusedOption() gotPath = %v, want %v", gotPath, tt.wantPath)
			}
		})
	}
}
