package textParser

import (
	"PandaBot/internal/protocol"
	"testing"
)

func TestProtectShellTriggers(t *testing.T) {
	parser := NewTextParser()

	tests := []struct {
		message  string
		expected string
	}{
		{"protect", "protect"},
		{"please protect me", "protect"},
		{"shell", "shell"},
		{"shell please", "shell"},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			chatLine := &protocol.ChatLine{
				Sender:  "Sender",
				Message: tt.message,
			}
			events, err := parser.ParseMessage(chatLine)
			if err != nil {
				t.Fatalf("Failed to parse message: %v", err)
			}

			found := false
			for _, event := range events {
				if event.TriggerType == tt.expected {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Expected trigger %s not found in events %v", tt.expected, events)
			}
		})
	}
}
