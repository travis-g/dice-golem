package main

import "errors"

var (
	ErrUnexpectedError        = errors.New("Sorry! Something errored unexpectedly. Please check to make sure your command was valid.")
	ErrInvalidCommand         = errors.New("Sorry! Dice Golem is not configured to handle that command. Please try restarting Discord to refresh available commands.")
	ErrDMError                = errors.New("Sorry! A direct message couldn't be sent. Do you allow DMs from users in this server?")
	ErrSendMessagePermissions = errors.New("Sorry! A response message could not be posted. Please make sure Dice Golem has _Send Messages_ permissions in the channel.")
)
