package main

import (
	"context"
	"time"

	"github.com/armon/go-metrics"
	"github.com/bwmarrin/discordgo"
)

type contextKey string

func (c contextKey) String() string {
	return "dice-golem context key " + string(c)
}

var (
	KeySession     = contextKey("session")
	KeyInteraction = contextKey("interaction")
	KeyMessage     = contextKey("message")

	KeyRollInput = contextKey("roll")
)

// NewContext creates a child request context with supplied event data.
func NewContext(ctx context.Context, s *discordgo.Session, i *discordgo.Interaction, m *discordgo.Message) context.Context {
	defer metrics.MeasureSince([]string{"core", "new_context"}, time.Now())
	ctx = context.WithValue(ctx, KeySession, s)
	ctx = context.WithValue(ctx, KeyInteraction, i)
	ctx = context.WithValue(ctx, KeyMessage, m)
	return ctx
}

// FromContext returns the originating session, interaction, and message from a
// context. One of Interaction or Message should be nil.
func FromContext(ctx context.Context) (*discordgo.Session, *discordgo.Interaction, *discordgo.Message) {
	defer metrics.MeasureSince([]string{"core", "from_context"}, time.Now())
	s := ctx.Value(KeySession).(*discordgo.Session)
	i := ctx.Value(KeyInteraction).(*discordgo.Interaction)
	m := ctx.Value(KeyMessage).(*discordgo.Message)
	return s, i, m
}

func GetSession(ctx context.Context) (*discordgo.Session, bool) {
	s := ctx.Value(KeySession)
	if s != nil {
		return s.(*discordgo.Session), true
	}
	return nil, false
}

// A HandlerFunc is a function that acts using a context.
type HandlerFunc func(ctx context.Context)

// A MiddlewareFunc chains together HandlerFuncs.
type MiddlewareFunc func(HandlerFunc) HandlerFunc
