package main

import "testing"

func TestChatInteractionMap(t *testing.T) {
	for _, command := range CommandsGlobalChat {
		if _, ok := handlers[command.Name]; !ok {
			t.Errorf("handler for command '%s' missing", command.Name)
		}
	}
}
