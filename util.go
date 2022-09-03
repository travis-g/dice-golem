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

// IsDirectMessage returns whether the Message event was spawned by a DM.
// Messages only have the ID of the associated channel, not a pointer.
func IsDirectMessage(m *discordgo.Message) bool {
	dm := (m.GuildID == "" || m.Member == nil)
	logger.Debug("direct message check",
		zap.Bool("dm", dm),
		zap.Bool("member", m.Member != nil),
		zap.Bool("user", m.Author != nil),
		zap.String("guild", m.GuildID),
		zap.String("channel", m.ChannelID),
	)
	return dm
}

// FIXME: completely redo this
func extractExpressionFromString(input string) (expression string) {
	expression = strings.TrimSpace(input)
	parts := strings.FieldsFunc(expression, commentFieldFunc)
	// lowercase everything
	expression = strings.TrimSpace(strings.ToLower(parts[0]))
	return
}

func messageSendFromInteraction(i *discordgo.InteractionResponse) *discordgo.MessageSend {
	logger.Debug("converting interaction", zap.Any("interaction", i))
	return &discordgo.MessageSend{
		Content:         i.Data.Content,
		Embeds:          i.Data.Embeds,
		AllowedMentions: i.Data.AllowedMentions,
	}
}

// trackRoll persists count information after a successful roll is made.
func trackRollFromContext(ctx context.Context) {
	defer recover()
	s, i, m := FromContext(ctx)
	if s == nil || (i == nil && m == nil) {
		panic(errors.New("context data missing"))
	}

	logger.Debug("tracking roll")
	metrics.IncrCounter([]string{"rolls"}, 1)

	// if no Redis cache, skip
	if DiceGolem.Redis == nil {
		return
	}

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
		pipe.Incr(fmt.Sprintf("rolls:total"))
		pipe.Incr(fmt.Sprintf("rolls:user:%s:total", uid))
		pipe.SAdd(fmt.Sprintf("rolls:users"), uid)
		pipe.SAdd(fmt.Sprintf("rolls:channels"), cid)
		pipe.Incr(fmt.Sprintf("rolls:guild:%s:chan:%s", gid, cid))
		if gid != "" {
			pipe.Incr(fmt.Sprintf("rolls:guild:%s", gid))
			pipe.SAdd(fmt.Sprintf("rolls:guilds"), gid)
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
	_, _, shardCounts, err := guildCount(b)
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

func guildCount(b *Bot) (guilds int, largeGuilds int, sharding []int, err error) {
	if b == nil {
		err = errors.New("nil bot")
		logger.Error(err.Error())
		return guilds, largeGuilds, sharding, err
	}

	// counts of guilds per indexed shard
	sharding = make([]int, len(b.Sessions))

	for i, s := range b.Sessions {
		guilds += len(s.State.Guilds)
		sharding[i] = len(s.State.Guilds)
		// count "large guilds"
		for _, guild := range s.State.Guilds {
			if guild.Large {
				largeGuilds += 1
			}
		}
	}
	return
}

// MarkdownString converts a dice group into a Markdown-compatible display
// format.
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

// MarkdownDetails returns text representations of an array of dice groups
// in Discord-flavored Markdown format.
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

// SelfInUsers returns whether the bot's user is contained in a slice of users.
func SelfInUsers(users []*discordgo.User) (found bool) {
	for _, user := range users {
		if user.ID == DiceGolem.SelfID {
			return true
		}
	}
	return
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

// distinct returns the distinct strings of a slice as a new slice. Blanks are
// omitted.
func distinct(in []string) (out []string) {
	uniques := make(map[string]bool)
	for _, item := range in {
		if item == "" {
			continue
		}
		if _, found := uniques[item]; !found {
			uniques[item] = true
			out = append(out, item)
		}
	}
	return
}

// String is a helper to return a pointer to the supplied string.
func String(v string) *string {
	return &v
}

// Bool is a helper to return a pointer to the supplied boolean.
func Bool(v bool) *bool {
	return &v
}

func Int64(i int64) *int64 {
	return &i
}
