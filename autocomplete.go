package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/bwmarrin/discordgo"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"go.uber.org/zap"
)

type Serialer interface {
	Serialize() string
	Deserialize(s string)
	String() string
}

func fuzzyFilterSerials(partial string, recentSerials, savedSerials []string) (matchingSerials []string) {
	serials := append(savedSerials, recentSerials...)
	matches := fuzzy.RankFindNormalizedFold(partial, serials)
	sort.Sort(matches)
	matchingSerials = TargetsFromRanks(matches)
	return
}

// ranked filter option choices using a partial string input
func fuzzyFilterOptionChoices(partial string, choices []*discordgo.ApplicationCommandOptionChoice) (matches []*discordgo.ApplicationCommandOptionChoice) {
	choices = DistinctChoices(choices)
	mchoices := make(map[string]*discordgo.ApplicationCommandOptionChoice)
	schoices := make([]string, len(choices))
	for i, choice := range choices {
		mchoices[choice.Name] = choice
		schoices[i] = choice.Name
	}
	rmatches := fuzzy.RankFindNormalizedFold(partial, schoices)
	sort.Sort(rmatches)
	smatches := TargetsFromRanks(rmatches)
	matches = make([]*discordgo.ApplicationCommandOptionChoice, len(smatches))
	for i, smatch := range smatches {
		matches[i] = mchoices[smatch]
	}
	return
}

func RollSliceFromSerials(serials []string) RollSlice {
	rolls := make([]*NamedRollInput, len(serials))
	for i, serial := range serials {
		var ri = new(NamedRollInput)
		ri.Deserialize(serial)
		rolls[i] = ri
	}
	return rolls
}

// SuggestRollsFromOption suggests named and unnamed expressions based on the
// value of the `option` field of the Interaction stored within the provided
// context.
func SuggestRollsFromOption(ctx context.Context, option string) {
	s, i, _ := FromContext(ctx)

	data := i.ApplicationCommandData()
	u := UserFromInteraction(i)

	recents, err := CachedSerials(u)
	logger.Debug("cached serials", zap.Any("serials", recents))
	if err != nil {
		logger.Error("cache error", zap.Error(err))
	}

	var choices []*discordgo.ApplicationCommandOptionChoice
	saved := SavedNamedRolls(fmt.Sprintf(KeyCacheUserGlobalExpressionsFmt, u.ID))

	input := getOptionByName(data.Options, option).StringValue()
	if input == "" {
		// rank the choices with recents first and with saved rolls after
		choices = ChoicesFromRollSliceExpression(trunc(RollSliceFromSerials(recents), 5))
		choices = append(choices, ChoicesFromRollSlice(saved)...)
	} else {
		// pull all recents, add saved expressions, and then sort by similarity
		// only if there is input to fuzzy-filter with
		choices = ChoicesFromRollSliceExpression(RollSliceFromSerials(recents))
		choices = append(choices, ChoicesFromRollSlice(saved)...)
		choices = fuzzyFilterOptionChoices(input, choices)
		// HACK: this is wildly inefficient, but re-allocate/re-size and preface
		// the option set with the user's current entered text
		choices = append(
			[]*discordgo.ApplicationCommandOptionChoice{{Name: input, Value: input}},
			choices...,
		)
	}

	// what's funnier than 24? 25...which is of course Discord's accepted number
	// of autocomplete options
	choices = trunc(DistinctChoices(choices), 25)

	logger.Debug("choices", zap.String("input", input), zap.Any("options", choices))

	if err := MeasureInteractionRespond(s.InteractionRespond, i, newChoicesResponse(choices)); err != nil {
		logger.Error("autocomplete", zap.Error(err), zap.String("user", u.ID))
	}
}

// SuggestRolls suggests named and unnamed expressions based on the `expression`
// option of the Interaction stored within the provided Context.
func SuggestRolls(ctx context.Context) {
	SuggestRollsFromOption(ctx, "expression")
}

func SuggestExpressions(ctx context.Context) {
	s, i, _ := FromContext(ctx)

	data := i.ApplicationCommandData()
	u := UserFromInteraction(i)

	recents, err := CachedSerials(u)
	if err != nil {
		logger.Error("cache error", zap.Error(err))
	}

	var choices []*discordgo.ApplicationCommandOptionChoice
	saved := SavedNamedRolls(fmt.Sprintf(KeyCacheUserGlobalExpressionsFmt, u.ID))

	// fuzzy-filtered stored rolls
	input := getOptionByName(data.Options, "expression").StringValue()
	choices = ChoicesFromRollSliceExpression(RollSliceFromSerials(recents))
	choices = append(choices, ChoicesFromRollSliceExpression(saved)...)
	if input != "" {
		// only sort by similarity if the user's entered something. by default
		// the ranking should be by recency
		choices = fuzzyFilterOptionChoices(input, choices)
		choices = append(
			[]*discordgo.ApplicationCommandOptionChoice{{Name: input, Value: input}},
			choices...,
		)
	}

	choices = DistinctChoices(choices)
	choices = trunc(choices, 25)

	logger.Debug("choices", zap.String("input", input), zap.Any("options", choices))

	if err := MeasureInteractionRespond(s.InteractionRespond, i, newChoicesResponse(choices)); err != nil {
		logger.Error("autocomplete", zap.Error(err), zap.String("user", u.ID))
	}
}

func SuggestNames(ctx context.Context) {
	s, i, _ := FromContext(ctx)

	data := i.ApplicationCommandData()
	u := UserFromInteraction(i)

	choices := []*discordgo.ApplicationCommandOptionChoice{}
	var input string
	switch {
	case getOptionByName(data.Options, "name") != nil:
		input = getOptionByName(data.Options, "name").StringValue()
	case getOptionByName(data.Options, "expression") != nil:
		input = getOptionByName(data.Options, "expression").StringValue()
	default:
		panic("unreachable code")
	}

	rolls := SavedNamedRolls(fmt.Sprintf(KeyCacheUserGlobalExpressionsFmt, u.ID))

	switch {
	case data.Name == "expressions" && data.Options[0].Name == "unsave":
		options := make([]string, len(rolls))
		stringMap := make(map[string]*NamedRollInput)
		for i, option := range rolls {
			options[i] = option.String()
			stringMap[option.String()] = option
		}

		if input != "" {
			matches := fuzzy.RankFindNormalizedFold(input, options)
			sort.Sort(matches)
			options = TargetsFromRanks(matches)
		}

		// build the choices list from filtered opts
		for _, option := range options {
			entry := stringMap[option]
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  entry.String(),
				Value: entry.ID(),
			})
		}
	case data.Name == "expressions" && data.Options[0].Name == "save":
		// only suggest existing expression names to overwrite or current input
		options := []string{}
		nameMap := make(map[string]*NamedRollInput)
		for _, option := range rolls {
			if option.Name != "" {
				options = append(options, option.Name)
				nameMap[option.Name] = option
			}
		}

		// if we have input, add the input as a choice, then rank filter the
		// other options
		if input != "" {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  input,
				Value: input,
			})

			matches := fuzzy.RankFindNormalizedFold(input, options)
			sort.Sort(matches)
			options = TargetsFromRanks(matches)
		}

		for _, option := range options {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  nameMap[option].String(),
				Value: nameMap[option].ID(),
			})
		}
	default:
		panic("unreachable code")
	}

	// truncate to max options count
	choices = trunc(choices, 25)
	logger.Debug("name choices", zap.Any("data", choices))

	if err := MeasureInteractionRespond(s.InteractionRespond, i,
		newChoicesResponse(choices)); err != nil {
		logger.Error("autocomplete", zap.Error(err))
	}
}

func SuggestLabel(ctx context.Context) {
	s, i, _ := FromContext(ctx)

	data := i.ApplicationCommandData()
	user := UserFromInteraction(i)
	rolls, err := CachedRolls(user)
	if err != nil {
		logger.Error("cache error", zap.Error(err))
	}
	logger.Debug("cached rolls", zap.Any("rolls", rolls))

	options := DistinctRollLabels(rolls)
	input := getOptionByName(data.Options, "label").StringValue()
	if input != "" {
		// only sort by similarity if the user's entered something. by default
		// the ranking should be by recency
		options = append([]string{input}, options...)
		matches := fuzzy.RankFindNormalizedFold(input, options)
		sort.Sort(matches)
		options = TargetsFromRanks(matches)
	}

	choices := DistinctChoices(ChoicesFromStrings(options))
	choices = trunc(choices, 25)
	logger.Debug("choices", zap.Any("data", choices))

	if err = MeasureInteractionRespond(s.InteractionRespond, i,
		newChoicesResponse(choices)); err != nil {
		logger.Error("autocomplete", zap.Error(err))
	}
}

func ChoicesFromRollSliceExpression(rolls RollSlice) []*discordgo.ApplicationCommandOptionChoice {
	if len(rolls) == 0 {
		return make([]*discordgo.ApplicationCommandOptionChoice, 0)
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, len(rolls))
	for i, roll := range rolls {
		choice := &discordgo.ApplicationCommandOptionChoice{
			Name:  roll.Expression,
			Value: roll.Expression,
		}
		choices[i] = choice
	}
	return choices
}

func ExpressionChoicesFromRollSlice(rolls RollSlice) []*discordgo.ApplicationCommandOptionChoice {
	if len(rolls) == 0 {
		return make([]*discordgo.ApplicationCommandOptionChoice, 0)
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, len(rolls))
	for i, roll := range rolls {
		choices[i] = &discordgo.ApplicationCommandOptionChoice{
			Name:  roll.Expression,
			Value: roll.Expression,
		}
	}
	return choices
}

func ChoicesFromRollSlice(rolls RollSlice) []*discordgo.ApplicationCommandOptionChoice {
	if len(rolls) == 0 {
		return make([]*discordgo.ApplicationCommandOptionChoice, 0)
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, len(rolls))
	for i, roll := range rolls {
		choice := new(discordgo.ApplicationCommandOptionChoice)
		choice.Name = roll.String()
		choice.Value = roll.RollableString()
		choices[i] = choice
	}
	return choices
}

func ChoicesFromRollSliceNames(rolls RollSlice) []*discordgo.ApplicationCommandOptionChoice {
	if len(rolls) == 0 {
		return make([]*discordgo.ApplicationCommandOptionChoice, 0)
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, len(rolls))
	for i, roll := range rolls {
		choice := &discordgo.ApplicationCommandOptionChoice{
			Name:  roll.String(),
			Value: roll.RollableString(),
		}
		choices[i] = choice
	}
	return choices
}

func ChoicesFromStrings(slice []string) []*discordgo.ApplicationCommandOptionChoice {
	if len(slice) == 0 {
		return make([]*discordgo.ApplicationCommandOptionChoice, 0)
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, len(slice))
	for i, value := range slice {
		choice := &discordgo.ApplicationCommandOptionChoice{
			Value: value,
			Name:  value,
		}
		choices[i] = choice
	}
	return choices
}

// DistinctChoices deduplicates a set of option choices by the choices' Names.
func DistinctChoices(choices []*discordgo.ApplicationCommandOptionChoice) (list []*discordgo.ApplicationCommandOptionChoice) {
	if len(choices) == 0 {
		return make([]*discordgo.ApplicationCommandOptionChoice, 0)
	}
	uniques := make(map[string]bool)
	for _, choice := range choices {
		if _, found := uniques[choice.Name]; !found {
			uniques[choice.Name] = true
			list = append(list, choice)
		}
	}
	return list
}

func DistinctRollExpressions(rolls []NamedRollInput) (expressions []string) {
	if len(rolls) == 0 {
		return make([]string, 0)
	}
	uniques := make(map[string]bool)
	for _, roll := range rolls {
		if roll.Expression == "" {
			continue
		}
		if _, found := uniques[roll.Expression]; !found {
			uniques[roll.Expression] = true
			expressions = append(expressions, roll.Expression)
		}
	}
	return expressions
}

func DistinctRollLabels(rolls []NamedRollInput) (labels []string) {
	uniques := make(map[string]bool)
	for _, roll := range rolls {
		if roll.Label == "" {
			continue
		}
		if _, found := uniques[roll.Label]; !found {
			uniques[roll.Label] = true
			labels = append(labels, roll.Label)
		}
	}
	return labels
}

func DistinctExpressionNames(rolls RollSlice) (names []string) {
	if len(rolls) == 0 {
		return make([]string, 0)
	}
	uniques := make(map[string]bool)
	for _, roll := range rolls {
		if roll.Name == "" {
			continue
		}
		if _, found := uniques[roll.Name]; !found {
			uniques[roll.Name] = true
			names = append(names, roll.Name)
		}
	}
	return names
}

func TargetsFromRanks(ranks fuzzy.Ranks) []string {
	var targets = make([]string, len(ranks))
	for i, rank := range ranks {
		targets[i] = rank.Target
	}
	return targets
}
