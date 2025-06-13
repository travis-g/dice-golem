package main

import (
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

// Deprecated: Response is a message response for dice roll responses.
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

func executeResponseTemplate(b *strings.Builder, r *Response) {
	responseResultTemplateCompiled.Execute(b, r)
}

type RollResponse struct {
	*NamedRollInput
	User   *discordgo.User
	Rolls  []interface{}
	Errors []error
}

func (r *RollResponse) Count() int {
	return len(r.Rolls)
}
