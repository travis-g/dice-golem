package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/bwmarrin/discordgo"
	"github.com/travis-g/dice"
	"go.uber.org/zap"
	"gopkg.in/redis.v3"
)

// SendMessage sends message data to a channel using a session. Note that
// DMs are sent and received through session 0.
func SendMessage(session *discordgo.Session, channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
	defer metrics.MeasureSince([]string{"discord", "send_message"}, time.Now())
	return session.ChannelMessageSendComplex(channelID, data)
}

// IsDirectMessage returns whether the Message event was spawned by a DM. DMs
// only have the ID of the associated channel, and no associated guild Member.
func IsDirectMessage(m *discordgo.Message) bool {
	test := (m.GuildID == "" || m.Member == nil)
	logger.Debug("direct message check",
		zap.Bool("dm", test),
		zap.Bool("member", m.Member != nil),
		zap.Bool("user", m.Author != nil),
		zap.String("guild", m.GuildID),
		zap.String("channel", m.ChannelID),
	)
	return test
}

func newMessageSendFromInteractionResponse(i *discordgo.InteractionResponse) *discordgo.MessageSend {
	logger.Debug("converting interaction", zap.Any("interaction", i))
	return &discordgo.MessageSend{
		Content:         i.Data.Content,
		Embeds:          i.Data.Embeds,
		AllowedMentions: i.Data.AllowedMentions,
	}
}

func newEphemeralResponse(content string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   discordgo.MessageFlagsEphemeral,
			Content: content,
		},
	}
}

func newChoicesResponse(choices []*discordgo.ApplicationCommandOptionChoice) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	}
}

// HACK: fix this so that it doesn't hold on to interaction pointers.
func deleteInteractionResponse(s *discordgo.Session, i *discordgo.Interaction, t time.Duration) {
	time.Sleep(t)
	err := s.InteractionResponseDelete(i)
	if err != nil {
		logger.Warn("cleanup error", zap.Error(err))
	}
}

// trackRoll persists count information after a successful roll is made.
func trackRollFromContext(ctx context.Context) {
	// if no Redis cache, skip
	if DiceGolem.Redis == nil {
		return
	}

	defer recover()
	s, i, m := FromContext(ctx)
	if s == nil || (i == nil && m == nil) {
		panic("context data missing")
	}

	logger.Debug("tracking roll")
	metrics.IncrCounter([]string{"rolls"}, 1)

	var (
		uid string // user ID
		cid string // channel ID
		gid string // guild ID
	)

	switch {
	case m != nil:
		if m.Member != nil && m.Member.User != nil {
			uid = m.Member.User.ID
		} else {
			uid = m.Author.ID
		}
		cid = m.ChannelID
		gid = m.GuildID
	case i != nil:
		uid = UserFromInteraction(i).ID
		cid = i.ChannelID
		gid = i.GuildID
	default:
		panic("unhandled roll type")
	}

	defer metrics.MeasureSince([]string{"redis", "track_roll"}, time.Now())
	_, err := DiceGolem.Redis.Pipelined(func(pipe *redis.Pipeline) error {
		pipe.Incr("rolls:total")
		pipe.Incr(fmt.Sprintf("rolls:user:%s:total", uid))
		pipe.SAdd("rolls:users", uid)
		pipe.SAdd("rolls:channels", cid)
		pipe.Incr(fmt.Sprintf("rolls:guild:%s:chan:%s", gid, cid))
		if gid != "" {
			pipe.Incr(fmt.Sprintf("rolls:guild:%s", gid))
			pipe.SAdd("rolls:guilds", gid)
		}
		return nil
	})

	if err != nil {
		logger.Error("error counting roll", zap.Error(err))
		return
	}
}

// ServerCount is a payload of Shards and the Guilds tracked by each of them to
// upload to the Discord Bot List.
type ServerCount struct {
	// Shards is an slice of counts of Guilds per Shard.
	Shards []int `json:"shards"`
}

func postServerCount(b *Bot) error {
	_, shardCounts, err := guildCount(b)
	if err != nil {
		return err
	}
	count := &ServerCount{
		Shards: shardCounts,
	}
	jsonBytes, _ := json.Marshal(count)
	payload := bytes.NewReader(jsonBytes)

	url := fmt.Sprintf("https://top.gg/api/bots/%s/stats", b.DefaultSession.State.User.ID)
	logger.Debug("shard counts", zap.Any("data", count), zap.String("url", url))

	req, _ := http.NewRequest("POST", url, payload)
	req.Header.Add("Authorization", b.TopToken)
	req.Header.Add("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error("dbl post error", zap.Error(err))
		return err
	}
	if res.StatusCode != 200 {
		logger.Error("dbl non-OK", zap.Int("status", res.StatusCode))
		return err
	}
	logger.Debug("updated dbl server count")
	return nil
}

func guildCount(b *Bot) (guilds int, sharding []int, err error) {
	if b == nil {
		err = errors.New("nil bot")
		logger.Error(err.Error())
		return guilds, sharding, err
	}

	// counts of guilds per indexed shard
	sharding = make([]int, len(b.Sessions))

	for i, s := range b.Sessions {
		guilds += len(s.State.Guilds)
		sharding[i] = len(s.State.Guilds)
	}
	return
}

// MarkdownString converts a dice group into a Markdown-compatible text format.
func MarkdownString(ctx context.Context, group *dice.RollerGroup) string {
	var b strings.Builder
	write := b.WriteString
	for _, roller := range group.Group.Copy() {
		val, _ := roller.Value(ctx)
		sval := strconv.FormatFloat(val, 'f', -1, 64)
		// TODO: check if critical
		if roller.IsDropped(ctx) {
			write("~~")
			write(sval)
			write("~~")
		} else {
			write(sval)
		}
		write(", ")
	}
	s := strings.TrimSuffix(b.String(), ", ")
	val, _ := group.Total(ctx)
	// HACK: use string builder and include original notation
	s = fmt.Sprintf("[%s] \u21D2 **%s**", s, strconv.FormatFloat(val, 'f', -1, 64))
	return s
}

// MarkdownDetails returns text representations of an array of dice
// groups as Discord-flavored Markdown format.
func MarkdownDetails(ctx context.Context, groups []*dice.RollerGroup) string {
	logger.Debug("markdown string", zap.Any("group", groups))
	var b strings.Builder
	write := b.WriteString
	for _, group := range groups {
		write(MarkdownString(ctx, group))
		write("\n")
	}
	return b.String()
}

// FIXME: this shouldn't be full responses themselves
type RollLog struct {
	Entries []*Response
}

func MessageEmbeds(ctx context.Context, log *RollLog) []*discordgo.MessageEmbed {
	embeds := make([]*discordgo.MessageEmbed, len(log.Entries))
	if len(embeds) == 0 {
		return embeds
	}
	includeTitles := len(embeds) > 1
	for i, entry := range log.Entries {
		embed := new(discordgo.MessageEmbed)
		if includeTitles {
			embed.Footer = &discordgo.MessageEmbedFooter{
				Text: fmt.Sprintf("Roll %d", i+1),
			}
		}
		embed.Fields = []*discordgo.MessageEmbedField{
			embedField(entry),
		}
		embeds[i] = embed
	}
	return embeds
}

func embedField(response *Response) *discordgo.MessageEmbedField {
	var field = new(discordgo.MessageEmbedField)
	field.Inline = false
	field.Name = fmt.Sprintf("%s \u21D2 %s", response.Original, response.Result)
	var b strings.Builder
	write := b.WriteString
	for _, group := range response.Dice {
		write(MarkdownString(context.TODO(), group))
		write("\n")
	}
	if len(response.Dice) == 0 {
		write("```cs\n" + response.ExpressionResult.String() + "\n```")
	}
	field.Value = strings.TrimSpace(b.String())
	logger.Debug("field", zap.Any("data", field))
	return field
}

// SelfInUsers returns whether the bot's user is contained in a slice of users.
func SelfInUsers(users []*discordgo.User) (found bool) {
	for _, user := range users {
		if user.ID == DiceGolem.SelfID {
			return true
		}
	}
	return
}

// MentionUser returns a Discord mention string for a User.
func MentionUser(u *discordgo.User) string {
	return "<@" + u.ID + ">"
}

func MentionCommand(paths ...string) string {
	var b strings.Builder
	write := b.WriteString
	for _, path := range paths {
		write(path)
		write(" ")
	}
	path := b.String()
	path = path[:b.Len()-1] // truncate trailing space
	return fmt.Sprintf("</%s:%s>", path, DiceGolem.SelfID)
}

// Ptr returns a pointer to the passed value.
func Ptr[T any](v T) *T {
	return &v
}

// contains returns whether a slice of strings contains a specific string.
func contains(haystack []string, needle string) (found bool) {
	for _, hay := range haystack {
		if hay == needle {
			return true
		}
	}
	return
}

// trunc returns a slice of the first num items of an array if array's length is
// longer then num. If array is shorter than num, the original array is
// returned.
func trunc[T any](arr []T, num int) []T {
	if num < 0 {
		panic("cannot truncate to negative length")
	}
	if len(arr) > num {
		return arr[:num]
	}
	return arr
}

// truncString truncates a string to a length if the string's length is over
// num characters.
func truncString(s string, num int) string {
	if len(s) > num {
		return s[:num]
	}
	return s
}
