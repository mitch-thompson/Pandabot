package textParser

import (
	"PandaBot/internal/protocol"
	"math/rand"
	"testing"
	"time"
)

// **Feature: automated-gameplay-assistant, Property 5: Message forwarding**
// Property 5: Message forwarding
// For any chat message containing trigger words from authorized party members, the Lua_Plugin should forward the complete message to the Go_Server for processing
// Validates: Requirements 2.1

func TestMessageForwarding(t *testing.T) {
	// Run property-based test with 100 iterations
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_MessageForwarding", func(t *testing.T) {
			parser := NewTextParser()
			
			// Generate random authorized user
			authorizedUser := generateRandomUsername()
			parser.AddAuthorizedUser(authorizedUser)
			
			// Generate random trigger word and message containing it
			triggerWord := generateRandomTriggerWord()
			message := generateMessageWithTrigger(triggerWord)
			
			chatLine := &protocol.ChatLine{
				Mode:      1,
				Sender:    authorizedUser,
				Message:   message,
				Timestamp: time.Now().Unix(),
			}
			
			// Parse the message
			actions, err := parser.ParseMessage(chatLine)
			
			// Should successfully parse without error for authorized users with trigger words
			if err != nil {
				t.Errorf("Message forwarding failed for authorized user with trigger word: %v", err)
			}
			
			// Should return at least one action for messages containing trigger words
			if len(actions) == 0 {
				t.Error("No actions returned for message containing trigger word")
			}
			
			// Verify the action targets the correct sender
			for _, action := range actions {
				if action.Target != authorizedUser {
					t.Errorf("Action target %s does not match sender %s", action.Target, authorizedUser)
				}
			}
		})
	}
}

// **Feature: automated-gameplay-assistant, Property 7: Unauthorized player filtering**
// Property 7: Unauthorized player filtering
// For any trigger word detected from an unknown or unauthorized player, the Go_Server should ignore the request and log the event without taking action
// Validates: Requirements 2.5

func TestUnauthorizedPlayerFiltering(t *testing.T) {
	// Run property-based test with 100 iterations
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_UnauthorizedPlayerFiltering", func(t *testing.T) {
			parser := NewTextParser()
			
			// Add some authorized users but not the test user
			parser.AddAuthorizedUser("authorizedUser1")
			parser.AddAuthorizedUser("authorizedUser2")
			
			// Generate random unauthorized user
			unauthorizedUser := "unauthorized_" + generateRandomUsername()
			
			// Generate message with trigger word from unauthorized user
			triggerWord := generateRandomTriggerWord()
			message := generateMessageWithTrigger(triggerWord)
			
			chatLine := &protocol.ChatLine{
				Mode:      1,
				Sender:    unauthorizedUser,
				Message:   message,
				Timestamp: time.Now().Unix(),
			}
			
			// Parse the message
			actions, err := parser.ParseMessage(chatLine)
			
			// Should return error for unauthorized users
			if err == nil {
				t.Error("Expected error for unauthorized user, but got none")
			}
			
			// Should not return any actions for unauthorized users
			if len(actions) > 0 {
				t.Errorf("Unauthorized user triggered %d actions, expected 0", len(actions))
			}
		})
	}
}

func TestTriggerWordDetection(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_TriggerWordDetection", func(t *testing.T) {
			parser := NewTextParser()
			
			// Test with authorized user
			user := generateRandomUsername()
			parser.AddAuthorizedUser(user)
			
			// Test various trigger words
			triggerWords := []string{"stoned", "firebuffs", "heal", "cure", "paralyzed"}
			triggerWord := triggerWords[rand.Intn(len(triggerWords))]
			
			// Generate message containing the trigger word
			message := generateMessageWithTrigger(triggerWord)
			
			chatLine := &protocol.ChatLine{
				Mode:      1,
				Sender:    user,
				Message:   message,
				Timestamp: time.Now().Unix(),
			}
			
			actions, err := parser.ParseMessage(chatLine)
			
			if err != nil {
				t.Errorf("Unexpected error parsing message with trigger word: %v", err)
			}
			
			// Should detect the trigger word and return appropriate actions
			found := false
			for _, action := range actions {
				if len(action.Spells) > 0 {
					found = true
					break
				}
			}
			
			if !found {
				t.Errorf("Trigger word '%s' not detected in message '%s'", triggerWord, message)
			}
		})
	}
}

func TestMultipleTriggerWords(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_MultipleTriggerWords", func(t *testing.T) {
			parser := NewTextParser()
			
			user := generateRandomUsername()
			parser.AddAuthorizedUser(user)
			
			// Create message with multiple trigger words
			message := "I am stoned and need firebuffs please help"
			
			chatLine := &protocol.ChatLine{
				Mode:      1,
				Sender:    user,
				Message:   message,
				Timestamp: time.Now().Unix(),
			}
			
			actions, err := parser.ParseMessage(chatLine)
			
			if err != nil {
				t.Errorf("Unexpected error parsing message with multiple triggers: %v", err)
			}
			
			// Should detect multiple trigger words
			if len(actions) < 2 {
				t.Errorf("Expected at least 2 actions for multiple trigger words, got %d", len(actions))
			}
		})
	}
}

// Generator functions for property-based testing

func generateRandomUsername() string {
	prefixes := []string{"player", "user", "char", "hero"}
	suffix := rand.Intn(1000)
	return prefixes[rand.Intn(len(prefixes))] + string(rune('0'+suffix%10)) + string(rune('0'+(suffix/10)%10))
}

func generateRandomTriggerWord() string {
	triggers := []string{
		"stoned", "paralyzed", "silenced", "poisoned", "blinded",
		"firebuffs", "waterbuffs", "thunderbuffs", "earthbuffs",
		"windbuffs", "icebuffs", "lightbuffs", "darkbuffs",
		"heal", "cure", "help",
	}
	return triggers[rand.Intn(len(triggers))]
}

func generateMessageWithTrigger(triggerWord string) string {
	prefixes := []string{"I am ", "Please help I'm ", "Need help with ", ""}
	suffixes := []string{" please", " help me", " now", ""}
	
	prefix := prefixes[rand.Intn(len(prefixes))]
	suffix := suffixes[rand.Intn(len(suffixes))]
	
	return prefix + triggerWord + suffix
}

func generateRandomMessage() string {
	messages := []string{
		"hello everyone",
		"how is everyone doing",
		"ready for battle",
		"let's go",
		"good job team",
		"stoned help",
		"need firebuffs",
		"cure please",
		"I'm paralyzed",
		"silenced need help",
	}
	return messages[rand.Intn(len(messages))]
}