package main

import (
	"errors"
	"fmt"

	"github.com/travis-g/dice"
	"github.com/travis-g/dice/math"
	"go.uber.org/zap"
)

type BotError struct {
	error
	Human error
}

// Errors.
var (
	ErrNilExpressionResult = errors.New("nil expression result")
	ErrTokenTransition     = errors.New("token transition error")
	ErrTooManyDice         = errors.New("too many dice")
	ErrNotImplemented      = errors.New("not implemented")
)

var (
	ErrUnexpectedError        = errors.New("Sorry! Something errored unexpectedly. Please check to make sure your command was valid.")
	ErrInvalidCommand         = errors.New("Sorry! Dice Golem is not configured to handle that command. Please try again later.")
	ErrDMError                = errors.New("Sorry! A direct message couldn't be sent. Do you allow DMs from users in this server?")
	ErrSendMessagePermissions = errors.New("Sorry! A response message could not be posted in the channel. Please make sure Dice Golem has _Send Messages_ permissions in the channel.")
)

func createFriendlyError(err error) error {
	logger.Debug("error", zap.Error(err))
	switch err {
	case dice.ErrInvalidExpression:
		return fmt.Errorf("I can't evaluate that expression. Is that roll valid?")
	case ErrNilExpressionResult:
		return fmt.Errorf("Something's wrong with that expression, it was empty.")
	case ErrTooManyDice:
		return fmt.Errorf("Your roll may require too many dice, please try a smaller roll (under %d dice).", DiceGolem.MaxDice)
	case math.ErrNilResult:
		return fmt.Errorf("Your roll didn't yield a result.")
	case ErrTokenTransition:
		return fmt.Errorf("An error was thrown when evaluating your expression. Please check for extra spaces in notations or missing math operators.")
	default:
		return fmt.Errorf("Something unexpected errored. Please check </help:%s>.", DiceGolem.SelfID)
	}
}
