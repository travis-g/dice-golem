package main

import "errors"

var (
	ErrUnexpectedError = errors.New("Sorry! Something errored unexpectedly. Please check to make sure your command was valid.")
	ErrInvalidCommand  = errors.New("Sorry! Dice Golem is not configured to handle that command. Please try restarting Discord to refresh available commands.")
)
