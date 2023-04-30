package main

import (
	"context"
	"strings"
	"text/template"

	"github.com/bwmarrin/discordgo"
	"github.com/travis-g/dice/math"
)

// Response templates for dice roll message responses.
var (
	ResponseTemplate = "{{if .Name}}{{.Name}} rolled{{end}}{{if .Expression}} `{{.Expression}}`{{end}}{{if .Label}} _{{.Label}}_{{end}}: `{{.Rolled}}` = **{{.Result}}**"
)

var (
	responseResultTemplateCompiled = template.Must(
		template.New("result").Parse(ResponsePrefix + ResponseTemplate),
	)
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
		r.Name = u.Mention()
	}

	return i
}

func (r *Response) MessageSend(ctx context.Context) *discordgo.MessageSend {
	return new(discordgo.MessageSend)
}

func executeResponseTemplate(b *strings.Builder, r *Response) {
	responseResultTemplateCompiled.Execute(b, r)
}
