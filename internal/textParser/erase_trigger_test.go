package textParser

import (
	"PandaBot/internal/protocol"
	"testing"
	"time"
)

func TestEraseTriggerDetection(t *testing.T) {
	parser := NewTextParser()
	user := "TestPlayer"
	message := "I need erase"

	chatLine := &protocol.ChatLine{
		Mode:      1,
		Sender:    user,
		Message:   message,
		Timestamp: time.Now().Unix(),
	}

	actions, err := parser.ParseMessage(chatLine)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	found := false
	for _, action := range actions {
		if action.TriggerType == "erase" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Trigger 'erase' not detected in message '%s'", message)
	}
}
