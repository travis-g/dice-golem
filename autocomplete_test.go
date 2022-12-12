package main

import (
	"reflect"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestDistinctChoices(t *testing.T) {
	type args struct {
		choices []*discordgo.ApplicationCommandOptionChoice
	}
	tests := []struct {
		name string
		args args
		want []*discordgo.ApplicationCommandOptionChoice
	}{
		{
			name: "empty",
			args: args{},
			want: []*discordgo.ApplicationCommandOptionChoice{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DistinctChoices(tt.args.choices); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DistinctChoices() = %v, want %v", got, tt.want)
			}
		})
	}
}
