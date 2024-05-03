package main

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Constant fmt string formats for settings keys. Channels themselves can be
// separate from guilds (ex. DMs) and so can be stored separately.
//
// User global preference
// User guild preference
// User DM/GDM preference
// Guild global setting
// Guild channel setting
const (
	KeyUserPreferencesFmt        = "settings:user:%s"                  // User global preferences
	KeyUserGuildPreferencesFmt   = KeyUserPreferencesFmt + ":guild:%s" // User guild preferences
	KeyUserChannelPreferencesFmt = KeyUserPreferencesFmt + ":chan:%s"  // User DM/GDM preferences
	KeyGuildSettingsFmt          = "settings:guild:%s"                 // Guild global settings
	KeyChannelSettingsFmt        = KeyGuildSettingsFmt + ":chan:%s"    // Guild channel settings (overrides)
	KeyChannelNamedSettingFmt    = KeyChannelSettingsFmt + ":%s"
)

// TODO: redo settings management with binary
type SettingName string

// Setting/preference name contants
const (
	SettingNoRecent       SettingName = "norecent"
	SettingDetailed       SettingName = "detailed"
	SettingNoAutocomplete SettingName = "noautocomplete"
	SettingSilent         SettingName = "silent"

	SettingForward SettingName = "forward"
)

func (s SettingName) String() string {
	return string(s)
}

func UserSetPreference(u *discordgo.User, s SettingName) {
	ctx := context.TODO()
	DiceGolem.Cache.Redis.SAdd(ctx, fmt.Sprintf(KeyUserPreferencesFmt, u.ID), s.String())
}

func UserUnsetPreference(u *discordgo.User, s SettingName) {
	ctx := context.TODO()
	DiceGolem.Cache.Redis.SRem(ctx, fmt.Sprintf(KeyUserPreferencesFmt, u.ID), s.String())
}

func UserHasPreference(u *discordgo.User, s SettingName) bool {
	ctx := context.TODO()
	return DiceGolem.Cache.SIsMember(ctx, fmt.Sprintf(KeyUserPreferencesFmt, u.ID), s.String())
}

func GuildHasSetting(g *discordgo.Guild, s SettingName) bool {
	ctx := context.TODO()
	return DiceGolem.Cache.SIsMember(ctx, fmt.Sprintf(KeyGuildSettingsFmt, g.ID), s.String())
}

func GuildSetSetting(g *discordgo.Guild, s SettingName) {
	ctx := context.TODO()
	key := fmt.Sprintf(KeyGuildSettingsFmt, g.ID)
	DiceGolem.Cache.Redis.SAdd(ctx, key, s.String())
	defer DiceGolem.Cache.Redis.Expire(ctx, key, DiceGolem.DataTTL)
}

func GuildUnsetSetting(g *discordgo.Guild, s SettingName) {
	ctx := context.TODO()
	key := fmt.Sprintf(KeyGuildSettingsFmt, g.ID)
	DiceGolem.Cache.Redis.SRem(ctx, key, s.String())
}

// Persists a setting to storage with a TTL duration.
func GuildChannelSetNamedSettingWithExpiry(gid, cid string, s SettingName, value string, ttl time.Duration) {
	ctx := context.TODO()
	key := fmt.Sprintf(KeyChannelNamedSettingFmt, gid, cid, s.String())
	DiceGolem.Cache.Redis.Set(ctx, key, value, ttl)
}

// Persists a setting to storage with the default data TTL.
func GuildChannelSetNamedSetting(gid, cid string, s SettingName, value string) {
	GuildChannelSetNamedSettingWithExpiry(gid, cid, s, value, DiceGolem.DataTTL)
}
