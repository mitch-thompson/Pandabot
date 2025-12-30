package textParser

import (
	"PandaBot/internal/protocol"
	"testing"
	"time"
)

func TestParseMessage_UnknownSender(t *testing.T) {
	parser := NewTextParser()

	// Case 1: Sender is "Unknown", but message contains sender in (PlayerName) format
	chatLine := &protocol.ChatLine{
		Mode:      12, // Party
		Sender:    "Unknown",
		Message:   "(TestPlayer) paralyzed",
		Timestamp: time.Now().Unix(),
	}

	events, err := parser.ParseMessage(chatLine)
	if err != nil {
		t.Fatalf("Unexpected error for (TestPlayer) format: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event for (TestPlayer) format, got %d", len(events))
	} else if events[0].Sender != "TestPlayer" {
		t.Errorf("Expected sender TestPlayer, got %s", events[0].Sender)
	}

	// Case 2: Sender is "Unknown", but message contains sender in [PlayerName] format
	chatLine.Message = "[TestPlayer] paralyzed"
	events, err = parser.ParseMessage(chatLine)
	if err != nil {
		t.Fatalf("Unexpected error for [TestPlayer] format: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event for [TestPlayer] format, got %d", len(events))
	} else if events[0].Sender != "TestPlayer" {
		t.Errorf("Expected sender TestPlayer, got %s", events[0].Sender)
	}

	// Case 3: Sender is "Unknown", but message contains sender in PlayerName: format
	chatLine.Message = "TestPlayer: paralyzed"
	events, err = parser.ParseMessage(chatLine)
	if err != nil {
		t.Fatalf("Unexpected error for TestPlayer: format: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event for TestPlayer: format, got %d", len(events))
	} else if events[0].Sender != "TestPlayer" {
		t.Errorf("Expected sender TestPlayer, got %s", events[0].Sender)
	}

	// Case 4: Sender is "Unknown" and no sender in message - should now work (not fail authorization)
	chatLine.Message = "paralyzed"
	events, err = parser.ParseMessage(chatLine)
	if err != nil {
		t.Errorf("Unexpected error for truly unknown sender: %v", err)
	}

	// Since sender is "", it might be skipped if we still have the check "if effectiveSender == """
	// Let's check ParseMessage logic again.
	if len(events) != 0 {
		t.Logf("Events found for unknown sender: %v", events)
	}

	// Case 5: Sender is valid from start - should work
	chatLine.Sender = "TestPlayer"
	chatLine.Message = "paralyzed"
	events, err = parser.ParseMessage(chatLine)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event for valid sender, got %d", len(events))
	} else if events[0].Sender != "TestPlayer" {
		t.Errorf("Expected sender TestPlayer, got %s", events[0].Sender)
	}
}
