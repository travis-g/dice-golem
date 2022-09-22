package main

import (
	"context"

	"github.com/bwmarrin/discordgo"
	"github.com/travis-g/dice/math"
)

// Response is a message response for dice roll responses.
type Response struct {
	*math.ExpressionResult
	// Name of who made the roll (optional)
	Name          string
	Rolled        string
	Result        string
	Expression    string
	Label         string
	FriendlyError error

	Error error
}

func (r *Response) Interaction(ctx context.Context) (i *discordgo.Interaction) {
	_, in, _ := FromContext(ctx)
	u := UserFromInteraction(in)
	if isRollPublic(i) {
		r.Name = Mention(u)
	}

	return i
}

func (r *Response) MessageSend(ctx context.Context) *discordgo.MessageSend {
	return new(discordgo.MessageSend)
}
