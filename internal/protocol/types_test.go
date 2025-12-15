package protocol

import (
	"math/rand"
	"testing"
	"time"
)

// **Feature: automated-gameplay-assistant, Property 22: Protocol compliance**
// Property 22: Protocol compliance
// For any message sent between the Lua_Plugin and Go_Server, it should use the structured JSON protocol for reliable data transmission
// Validates: Requirements 7.3

func TestProtocolCompliance(t *testing.T) {
	// Run property-based test with 100 iterations as specified in design
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_ProtocolCompliance", func(t *testing.T) {
			// Generate random message type
			msgType := generateRandomMessageType()
			
			// Generate appropriate body for the message type
			body := generateRandomBodyForType(msgType)
			
			// Create message
			msg := &Message{
				Type: msgType,
				Body: body,
			}
			
			// Validate message follows protocol structure
			err := ValidateMessage(msg)
			
			// For valid message types with valid bodies, validation should pass
			if isValidMessageType(msgType) && body != nil {
				if err != nil {
					t.Errorf("Valid message failed validation: %v", err)
				}
			}
			
			// Message should have required fields
			if msg.Type == 0 {
				t.Error("Message type should not be zero")
			}
		})
	}
}

func TestExecuteCommandValidation(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_ExecuteCommandValidation", func(t *testing.T) {
			cmd := generateRandomExecuteCommand()
			err := ValidateExecuteCommand(cmd)
			
			// Check that validation correctly identifies invalid commands
			if cmd.Command == "" {
				if err == nil {
					t.Error("Empty command should fail validation")
				}
			}
			
			if cmd.Priority < 1 || cmd.Priority > 10 {
				if err == nil {
					t.Errorf("Invalid priority %d should fail validation", cmd.Priority)
				}
			}
			
			if cmd.Timeout < 0 {
				if err == nil {
					t.Errorf("Negative timeout %d should fail validation", cmd.Timeout)
				}
			}
		})
	}
}

func TestChatLineValidation(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_ChatLineValidation", func(t *testing.T) {
			chat := generateRandomChatLine()
			err := ValidateChatLine(chat)
			
			// Check validation rules
			if chat.Sender == "" {
				if err == nil {
					t.Error("Empty sender should fail validation")
				}
			}
			
			if chat.Message == "" {
				if err == nil {
					t.Error("Empty message should fail validation")
				}
			}
			
			if chat.Timestamp <= 0 {
				if err == nil {
					t.Errorf("Invalid timestamp %d should fail validation", chat.Timestamp)
				}
			}
		})
	}
}

func TestStatusUpdateValidation(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_StatusUpdateValidation", func(t *testing.T) {
			status := generateRandomStatusUpdate()
			err := ValidateStatusUpdate(status)
			
			// Check validation rules
			if status.Timestamp <= 0 {
				if err == nil {
					t.Errorf("Invalid timestamp %d should fail validation", status.Timestamp)
				}
			}
			
			if status.PlayerHP < 0 || status.PlayerHP > 100 {
				if err == nil {
					t.Errorf("Invalid player HP %d should fail validation", status.PlayerHP)
				}
			}
			
			if status.PlayerMP < 0 || status.PlayerMP > 100 {
				if err == nil {
					t.Errorf("Invalid player MP %d should fail validation", status.PlayerMP)
				}
			}
		})
	}
}

// Generator functions for property-based testing

func generateRandomMessageType() MessageType {
	types := []MessageType{TypePing, TypePong, TypeExecuteCommand, TypeChatLine, TypeStatusUpdate, TypeErrorReport}
	// Sometimes generate invalid types to test validation
	if rand.Float32() < 0.1 {
		return MessageType(rand.Intn(255))
	}
	return types[rand.Intn(len(types))]
}

func generateRandomBodyForType(msgType MessageType) interface{} {
	switch msgType {
	case TypePing, TypePong:
		return nil
	case TypeExecuteCommand:
		return generateRandomExecuteCommand()
	case TypeChatLine:
		return generateRandomChatLine()
	case TypeStatusUpdate:
		return generateRandomStatusUpdate()
	case TypeErrorReport:
		return generateRandomErrorReport()
	default:
		return nil
	}
}

func generateRandomExecuteCommand() *ExecuteCommand {
	commands := []string{"/ma \"Cure IV\" <t>", "/ma \"Stona\" player1", "/ma \"Protect\" <me>", "", "invalid"}
	targets := []string{"<t>", "<me>", "player1", "player2", ""}
	
	return &ExecuteCommand{
		Command:  commands[rand.Intn(len(commands))],
		Target:   targets[rand.Intn(len(targets))],
		Priority: rand.Intn(15) - 2, // -2 to 12, includes invalid values
		Timeout:  rand.Intn(10000) - 100, // -100 to 9899, includes negative values
		ID:       generateRandomString(8),
	}
}

func generateRandomChatLine() *ChatLine {
	senders := []string{"player1", "player2", "player3", ""}
	messages := []string{"stoned", "firebuffs", "help me", "cure please", ""}
	
	return &ChatLine{
		Mode:      uint32(rand.Intn(10)),
		Sender:    senders[rand.Intn(len(senders))],
		Message:   messages[rand.Intn(len(messages))],
		Timestamp: int64(rand.Intn(2000000000)) - 1000000000, // includes negative values
	}
}

func generateRandomStatusUpdate() *StatusUpdate {
	partySize := rand.Intn(7) // 0-6 party members
	members := make([]PartyMember, partySize)
	
	for i := 0; i < partySize; i++ {
		members[i] = generateRandomPartyMember()
	}
	
	return &StatusUpdate{
		Timestamp:    int64(rand.Intn(2000000000)) - 1000000000,
		PartyMembers: members,
		PlayerMP:     rand.Intn(150) - 25, // -25 to 124, includes invalid values
		PlayerHP:     rand.Intn(150) - 25,
		Zone:         generateRandomString(10),
	}
}

func generateRandomPartyMember() PartyMember {
	names := []string{"player1", "player2", "player3", ""}
	jobs := []string{"WHM", "BLM", "WAR", "THF", ""}
	
	return PartyMember{
		Name:          names[rand.Intn(len(names))],
		HPPercent:     rand.Intn(150) - 25, // includes invalid values
		MPPercent:     rand.Intn(150) - 25,
		StatusEffects: generateRandomStatusEffects(),
		Job:           jobs[rand.Intn(len(jobs))],
		Distance:      rand.Float32()*100 - 10, // includes negative values
		LastUpdate:    time.Now(),
	}
}

func generateRandomStatusEffects() []int {
	count := rand.Intn(5)
	effects := make([]int, count)
	for i := 0; i < count; i++ {
		effects[i] = rand.Intn(1000)
	}
	return effects
}

func generateRandomErrorReport() *ErrorReport {
	ids := []string{"cmd_123", "cmd_456", ""}
	errors := []string{"spell failed", "target not found", "insufficient MP", ""}
	
	return &ErrorReport{
		CommandID: ids[rand.Intn(len(ids))],
		Error:     errors[rand.Intn(len(errors))],
		Timestamp: int64(rand.Intn(2000000000)) - 1000000000,
	}
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func isValidMessageType(msgType MessageType) bool {
	switch msgType {
	case TypePing, TypePong, TypeExecuteCommand, TypeChatLine, TypeStatusUpdate, TypeErrorReport:
		return true
	default:
		return false
	}
}

func TestJSONSerialization(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_JSONSerialization", func(t *testing.T) {
			// Test ExecuteCommand serialization
			cmd := generateRandomExecuteCommand()
			data, err := MarshalExecuteCommand(cmd)
			if err != nil {
				t.Errorf("Failed to marshal ExecuteCommand: %v", err)
			}
			
			deserializedCmd, err := UnmarshalExecuteCommand(data)
			if err != nil {
				t.Errorf("Failed to unmarshal ExecuteCommand: %v", err)
			}
			
			if deserializedCmd.Command != cmd.Command {
				t.Errorf("Command mismatch: expected %s, got %s", cmd.Command, deserializedCmd.Command)
			}
			
			// Test ChatLine serialization
			chat := generateRandomChatLine()
			chatData, err := MarshalChatLine(chat)
			if err != nil {
				t.Errorf("Failed to marshal ChatLine: %v", err)
			}
			
			deserializedChat, err := UnmarshalChatLine(chatData)
			if err != nil {
				t.Errorf("Failed to unmarshal ChatLine: %v", err)
			}
			
			if deserializedChat.Sender != chat.Sender {
				t.Errorf("Sender mismatch: expected %s, got %s", chat.Sender, deserializedChat.Sender)
			}
			
			// Test StatusUpdate serialization
			status := generateRandomStatusUpdate()
			statusData, err := MarshalStatusUpdate(status)
			if err != nil {
				t.Errorf("Failed to marshal StatusUpdate: %v", err)
			}
			
			deserializedStatus, err := UnmarshalStatusUpdate(statusData)
			if err != nil {
				t.Errorf("Failed to unmarshal StatusUpdate: %v", err)
			}
			
			if deserializedStatus.Zone != status.Zone {
				t.Errorf("Zone mismatch: expected %s, got %s", status.Zone, deserializedStatus.Zone)
			}
		})
	}
}