package casting

import (
	"PandaBot/internal/entity"
	"log"
	"testing"
	"time"
)

type MockClient struct {
	CapturedRequests []*CastRequest
}

func (m *MockClient) SendSpellCommand(command *SpellCommand) error {
	m.CapturedRequests = append(m.CapturedRequests, &CastRequest{
		SpellName: command.Spell,
		Target:    command.Target,
	})
	return nil
}
func (m *MockClient) GetClientInfo() *ClientInfo {
	return &ClientInfo{
		PlayerName:  "BotCaster",
		IsConnected: true,
		MP:          1000,
		JobLevels:   map[string]int{"WHM": 75},
	}
}
func (m *MockClient) IsConnected() bool                                       { return true }
func (m *MockClient) CheckReadyToCast(commandID string) (bool, string, error) { return true, "", nil }
func (m *MockClient) WaitForReadyForAction(timeout time.Duration) error       { return nil }
func (m *MockClient) LockExecution()                                          {}
func (m *MockClient) UnlockExecution()                                        {}

func TestReproduction_FirebuffsTargeting(t *testing.T) {
	// Setup engine
	config := DefaultCastingConfig()
	config.SequenceDelay = 10 * time.Millisecond
	engine := NewCastingEngine(config)

	// Mock client capture
	mockClient := &MockClient{CapturedRequests: make([]*CastRequest, 0)}
	cm := NewClientManager(engine)
	cm.RegisterClient("test", mockClient)
	engine.SetClientManager(cm)

	// Trigger "firebuffs"
	tp := NewTriggerProcessor(engine)

	sender := "TargetPlayer" // Original sender
	casterName := "BotCaster"
	casterMP := 1000
	casterJobLevels := map[string]int{"WHM": 75}
	// Party size > 1 to ensure area spells are selected
	partyMembers := []*entity.Entity{
		{Name: "BotCaster", Job: "WHM", JobLevel: 75},
		{Name: "TargetPlayer", Job: "WAR", JobLevel: 75},
	}

	requestIDs, err := tp.ProcessTriggerEvent(
		"firebuffs",
		sender,
		3,
		casterName,
		casterMP,
		casterJobLevels,
		partyMembers,
	)

	if err != nil {
		t.Fatalf("ProcessTriggerEvent failed: %v", err)
	}

	if len(requestIDs) != 1 {
		t.Fatalf("Expected 1 request ID (for the sequence), got %d", len(requestIDs))
	}

	requestID := requestIDs[0]

	// Start processing the first spell (Protectra V)
	engine.mu.RLock()
	activeCast, ok := engine.activeCasts[requestID]
	engine.mu.RUnlock()
	if !ok {
		t.Fatalf("Request not found in active casts")
	}

	// 1. Execute first spell
	activeCast.Request.Target = sender
	success, err := engine.executeCast(activeCast)
	if !success || err != nil {
		t.Fatalf("First executeCast failed: %v", err)
	}

	if len(mockClient.CapturedRequests) != 1 {
		t.Fatalf("Expected 1 captured request, got %d", len(mockClient.CapturedRequests))
	}

	if mockClient.CapturedRequests[0].SpellName != "Protectra V" {
		t.Errorf("Expected first spell to be Protectra V, got %s", mockClient.CapturedRequests[0].SpellName)
	}

	if mockClient.CapturedRequests[0].Target != casterName {
		t.Errorf("Expected first spell target to be %s, got %s", casterName, mockClient.CapturedRequests[0].Target)
	}

	// 2. Complete first spell and advance sequence
	engine.NotifySpellComplete(requestID, true, "")

	// Wait for sequence delay and processing (which calls executeCast again)
	// engine.NotifySpellComplete starts a goroutine that waits 500ms then calls processCast, which calls executeCast
	time.Sleep(700 * time.Millisecond)

	// We expect multiple requests because of how the test is set up
	// and because we added more spells to the sequence.
	if len(mockClient.CapturedRequests) < 2 {
		t.Fatalf("Expected at least 2 captured requests, got %d", len(mockClient.CapturedRequests))
	}

	if mockClient.CapturedRequests[1].SpellName != "Shellra V" {
		t.Errorf("Expected second spell to be Shellra V, got %s", mockClient.CapturedRequests[1].SpellName)
	}

	// THIS IS THE CRITICAL CHECK
	if mockClient.CapturedRequests[1].Target != casterName {
		t.Errorf("Expected second spell target to be %s, got %s. This confirms the issue!", casterName, mockClient.CapturedRequests[1].Target)
	} else {
		log.Printf("Second spell target correctly resolved to %s", mockClient.CapturedRequests[1].Target)
	}
}
