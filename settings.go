package main

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/bwmarrin/discordgo"
)

// Constant fmt string formats for settings keys.
const (
	SettingsKeyUserSettingsFormat  = "settings:user:%s"
	SettingsKeyGuildSettingsFormat = "settings:guild:%s"
)

// TODO: redo settings management with binary
type SettingName string

const (
	SettingNoRecent       SettingName = "norecent"
	SettingDetailed       SettingName = "detailed"
	SettingNoAutocomplete SettingName = "noautocomplete"
)

func (s SettingName) String() string {
	return string(s)
}

func SetPreference(u *discordgo.User, s SettingName) {
	DiceGolem.Redis.SAdd(fmt.Sprintf(SettingsKeyUserSettingsFormat, u.ID), s.String())
}

func UnsetPreference(u *discordgo.User, s SettingName) {
	DiceGolem.Redis.SRem(fmt.Sprintf(SettingsKeyUserSettingsFormat, u.ID), s.String())
}

func HasPreference(u *discordgo.User, s SettingName) bool {
	defer metrics.MeasureSince([]string{"redis", "sismember"}, time.Now())
	return DiceGolem.Redis.SIsMember(fmt.Sprintf(SettingsKeyUserSettingsFormat, u.ID), s.String()).Val()
}

func HasSetting(g *discordgo.Guild, s SettingName) bool {
	defer metrics.MeasureSince([]string{"redis", "sismember"}, time.Now())
	return DiceGolem.Redis.SIsMember(fmt.Sprintf(SettingsKeyGuildSettingsFormat, g.ID), s.String()).Val()
}
