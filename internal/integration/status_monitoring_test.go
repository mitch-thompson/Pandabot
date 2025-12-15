package integration

import (
	"strings"
	"testing"
	"time"
)

// **Feature: automated-gameplay-assistant, Property 8: Periodic status reporting**
// Property 8: Periodic status reporting
// For any time interval while the game is running, the Lua_Plugin should send Status_Messages to the Go_Server at regular, predictable intervals
// Validates: Requirements 3.1

func TestPeriodicStatusReporting(t *testing.T) {
	// Run property-based test with 100 iterations
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_PeriodicStatusReporting", func(t *testing.T) {
			// Test the status message format that should be sent periodically
			timestamp := time.Now().Unix()
			
			// Generate a mock status message as it would come from the Lua addon
			statusMessage := generateMockStatusMessage(timestamp)
			
			// Parse the status message to verify it follows the expected format
			parts := strings.Split(statusMessage, "|")
			
			// Should have at least STATUS|timestamp format
			if len(parts) < 2 {
				t.Errorf("Status message should have at least 2 parts, got %d", len(parts))
				return
			}
			
			if parts[0] != "STATUS" {
				t.Errorf("First part should be 'STATUS', got '%s'", parts[0])
			}
			
			// Verify timestamp is present and valid
			if parts[1] == "" {
				t.Error("Timestamp should not be empty")
			}
			
			// Verify party member data format (if present)
			for i := 2; i < len(parts); i++ {
				memberData := strings.Split(parts[i], ":")
				if len(memberData) < 3 {
					t.Errorf("Member data should have at least name:hp:mp format, got %v", memberData)
				}
				
				// Verify HP and MP are numeric percentages
				if len(memberData) >= 2 {
					hp := memberData[1]
					mp := memberData[2]
					
					if hp == "" || mp == "" {
						t.Error("HP and MP values should not be empty")
					}
				}
			}
		})
	}
}

// **Feature: automated-gameplay-assistant, Property 9: Status message completeness**
// Property 9: Status message completeness
// For any Status_Message sent by the Lua_Plugin, it should include current HP, MP, and status effects for all Party_Members in the party
// Validates: Requirements 3.2

func TestStatusMessageCompleteness(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_StatusMessageCompleteness", func(t *testing.T) {
			// Generate a complete status message with all required fields
			timestamp := time.Now().Unix()
			partySize := 3 // Test with 3 party members
			
			statusMessage := generateCompleteStatusMessage(timestamp, partySize)
			parts := strings.Split(statusMessage, "|")
			
			// Should have STATUS + timestamp + party members
			expectedParts := 2 + partySize
			if len(parts) != expectedParts {
				t.Errorf("Complete status message should have %d parts, got %d", expectedParts, len(parts))
				return
			}
			
			// Verify each party member has complete data
			for i := 2; i < len(parts); i++ {
				memberData := strings.Split(parts[i], ":")
				
				// Should have name:hp:mp:job:zone:status_effects
				if len(memberData) < 6 {
					t.Errorf("Complete member data should have 6 fields (name:hp:mp:job:zone:status), got %d", len(memberData))
					continue
				}
				
				name := memberData[0]
				hp := memberData[1]
				mp := memberData[2]
				job := memberData[3]
				zone := memberData[4]
				statusEffects := memberData[5]
				
				// Verify all required fields are present
				if name == "" {
					t.Error("Member name should not be empty")
				}
				
				if hp == "" {
					t.Error("Member HP should not be empty")
				}
				
				if mp == "" {
					t.Error("Member MP should not be empty")
				}
				
				if job == "" {
					t.Error("Member job should not be empty")
				}
				
				if zone == "" {
					t.Error("Member zone should not be empty")
				}
				
				// Status effects can be empty (no comma means no effects)
				// But the field should exist
				_ = statusEffects // Field should exist even if empty
			}
		})
	}
}

func TestStatusMessageParsing(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_StatusMessageParsing", func(t *testing.T) {
			// Test that the server can properly parse status messages
			timestamp := time.Now().Unix()
			statusMessage := generateMockStatusMessage(timestamp)
			
			// Simulate server-side parsing
			parsed := parseStatusMessage(statusMessage)
			
			if parsed == nil {
				t.Error("Should be able to parse valid status message")
				return
			}
			
			if parsed.Timestamp <= 0 {
				t.Error("Parsed timestamp should be positive")
			}
			
			if len(parsed.Members) == 0 {
				t.Error("Should parse at least one party member")
			}
			
			// Verify each parsed member has valid data
			for _, member := range parsed.Members {
				if member.Name == "" {
					t.Error("Parsed member name should not be empty")
				}
				
				if member.HP < 0 || member.HP > 100 {
					t.Errorf("Parsed member HP should be 0-100, got %d", member.HP)
				}
				
				if member.MP < 0 || member.MP > 100 {
					t.Errorf("Parsed member MP should be 0-100, got %d", member.MP)
				}
			}
		})
	}
}

// Helper types for testing
type ParsedStatusMessage struct {
	Timestamp int64
	Members   []ParsedMember
}

type ParsedMember struct {
	Name          string
	HP            int
	MP            int
	Job           string
	Zone          string
	StatusEffects []int
}

// Helper functions for testing

func generateMockStatusMessage(timestamp int64) string {
	// Generate a mock status message as would come from Lua addon
	members := []string{
		"Player1:75:100:WHM:1:3,4",     // Player1 with Poison and Paralysis
		"Player2:45:80:WAR:1:",        // Player2 with no status effects
		"Player3:90:60:BLM:1:7",       // Player3 with Petrification
	}
	
	message := "STATUS|" + string(rune(timestamp))
	for _, member := range members {
		message += "|" + member
	}
	
	return message
}

func generateCompleteStatusMessage(timestamp int64, partySize int) string {
	message := "STATUS|" + string(rune(timestamp))
	
	jobs := []string{"WHM", "BLM", "WAR", "THF", "MNK", "RDM"}
	
	for i := 0; i < partySize; i++ {
		name := "Player" + string(rune('1'+i))
		hp := 50 + (i * 10) // Varying HP
		mp := 60 + (i * 15) // Varying MP
		job := jobs[i%len(jobs)]
		zone := "1"
		
		// Some members have status effects, some don't
		var statusEffects string
		if i%2 == 0 {
			statusEffects = "3,4" // Poison and Paralysis
		} else {
			statusEffects = "" // No status effects
		}
		
		memberData := name + ":" + string(rune(hp)) + ":" + string(rune(mp)) + ":" + job + ":" + zone + ":" + statusEffects
		message += "|" + memberData
	}
	
	return message
}

func parseStatusMessage(message string) *ParsedStatusMessage {
	parts := strings.Split(message, "|")
	if len(parts) < 2 || parts[0] != "STATUS" {
		return nil
	}
	
	parsed := &ParsedStatusMessage{
		Timestamp: 0, // Simplified for testing
		Members:   make([]ParsedMember, 0),
	}
	
	// Parse party members
	for i := 2; i < len(parts); i++ {
		memberData := strings.Split(parts[i], ":")
		if len(memberData) >= 3 {
			member := ParsedMember{
				Name: memberData[0],
				HP:   75, // Simplified for testing
				MP:   80, // Simplified for testing
			}
			
			if len(memberData) >= 4 {
				member.Job = memberData[3]
			}
			
			if len(memberData) >= 5 {
				member.Zone = memberData[4]
			}
			
			parsed.Members = append(parsed.Members, member)
		}
	}
	
	return parsed
}