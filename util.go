package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/bwmarrin/discordgo"
	"github.com/redis/go-redis/v9"
	"github.com/travis-g/dice"
	"go.uber.org/zap"
)

// Revision is the build commit identifier for the version of Dice Golem
var Revision string

func init() {
	Revision = func() string {
		var revision string
		var modified bool
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				switch setting.Key {
				case "vcs.revision":
					revision = setting.Value
				case "vcs.modified":
					modified = (setting.Value == "true")
				}
			}
			var b strings.Builder
			write := b.WriteString
			write(truncString(revision, 7))
			if modified {
				write("+changes")
			}
			return b.String()
		}
		return "unknown"
	}()
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
	if DiceGolem.Cache.Redis == nil {
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
	_, err := DiceGolem.Cache.Redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Incr(ctx, "rolls:total")
		pipe.Incr(ctx, fmt.Sprintf("rolls:user:%s:total", uid))
		pipe.SAdd(ctx, "rolls:users", uid)
		pipe.SAdd(ctx, "rolls:channels", cid)
		pipe.Incr(ctx, fmt.Sprintf("rolls:guild:%s:chan:%s", gid, cid))
		if gid != "" {
			pipe.Incr(ctx, fmt.Sprintf("rolls:guild:%s", gid))
			pipe.SAdd(ctx, "rolls:guilds", gid)
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
	// ServerCount is an slice of counts of Guilds per Shard.
	ServerCount []int `json:"server_count"`
}

func postGuildCount(b *Bot) error {
	_, shardCounts, err := guildCount(b)
	if err != nil {
		logger.Error("stats posting error", zap.Error(err))
		return err
	}
	data := &ServerCount{
		ServerCount: shardCounts,
	}
	jsonBytes, _ := json.Marshal(data)
	payload := bytes.NewReader(jsonBytes)

	url := fmt.Sprintf("https://top.gg/api/bots/%s/stats", b.SelfID)
	logger.Debug("shard counts", zap.Any("data", data), zap.String("url", url))

	req, _ := http.NewRequest("POST", url, payload)
	req.Header.Add("Authorization", *b.TopToken)
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
	ctx := context.TODO()
	if b == nil {
		err = errors.New("nil bot")
		logger.Error(err.Error())
		return guilds, sharding, err
	}

	// counts of guilds per indexed shard
	_, err = DiceGolem.Cache.Redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		keys, err := DiceGolem.Cache.Redis.Keys(ctx, fmt.Sprintf(KeyStateShardGuildsFmt, "*")).Result()
		if err != nil {
			return err
		}
		sharding = make([]int, len(keys))
		for i, key := range keys {
			card, err := DiceGolem.Cache.Redis.SCard(ctx, key).Result()
			if err != nil {
				return err
			}
			sharding[i] = int(card)
		}
		return nil
	})

	for _, i := range sharding {
		guilds += i
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

func makeEmbedFooter() *discordgo.MessageEmbedFooter {
	return &discordgo.MessageEmbedFooter{
		Text:    DiceGolem.User.Username,
		IconURL: DiceGolem.User.AvatarURL("64"),
	}
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
func SelfInUsers(users []*discordgo.User) bool {
	for _, user := range users {
		if user.ID == DiceGolem.SelfID {
			return true
		}
	}
	return false
}

func HasSendMessagesPermission(s *discordgo.Session, cid string) bool {
	var c *discordgo.Channel
	var err error
	if c, err = s.Channel(cid); err != nil {
		logger.Error("channel error", zap.Error(err))
		return false
	}
	logger.Debug("channel data", zap.Any("c", c))
	// HACK: make the API reqs which are deprecated by the lib for some reason
	return true
}

func CommandMention(paths ...string) string {
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

// Ptr returns the pointer to the passed value.
func Ptr[T any](v T) *T {
	return &v
}

// contains returns whether a slice of strings contains a specific string.
func contains(haystack []string, needle string) bool {
	for _, hay := range haystack {
		if hay == needle {
			return true
		}
	}
	return false
}

// trunc returns a slice of the first n items of an array if array's length is
// longer then n. If array is shorter than n, the original array is returned.
func trunc[T any](arr []T, n int) []T {
	if n < 0 {
		panic("cannot truncate to negative length")
	}
	if len(arr) > n {
		return arr[:n]
	}
	return arr
}

// truncString returns a slice of a string truncted to length n if the string's
// length is over n characters.
func truncString(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
