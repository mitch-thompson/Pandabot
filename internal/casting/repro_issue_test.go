package casting

import (
	"PandaBot/internal/entity"
	"testing"
	"time"
)

func TestReproduction_ParalyzedTargetUnknown(t *testing.T) {
	// Setup engine and processor
	engine := NewCastingEngine(DefaultCastingConfig())
	tp := NewTriggerProcessor(engine)

	// Define test parameters
	triggerType := "paralyzed"
	sender := "TargetPlayer" // The person who is paralyzed
	priority := 7
	casterName := "BotCaster"
	casterMP := 100
	casterJobLevels := map[string]int{"WHM": 75}

	// Party members - empty for now to trigger the "entity not found" path
	partyMembers := []*entity.Entity{}

	// Process trigger event
	requestIDs, err := tp.ProcessTriggerEvent(
		triggerType,
		sender,
		priority,
		casterName,
		casterMP,
		casterJobLevels,
		partyMembers,
	)

	if err != nil {
		t.Fatalf("ProcessTriggerEvent failed: %v", err)
	}

	if len(requestIDs) == 0 {
		t.Fatalf("No request IDs returned")
	}

	// Verify the request
	engine.mu.RLock()
	defer engine.mu.RUnlock()

	request, ok := engine.activeCasts[requestIDs[0]]
	if !ok {
		t.Fatalf("Request %s not found in active casts", requestIDs[0])
	}

	if request.Request.Target != sender {
		t.Errorf("Expected target %s, got %s", sender, request.Request.Target)
	}

	// Also check if it's "Unknown" as reported in the issue
	if request.Request.Target == "Unknown" {
		t.Errorf("Target is 'Unknown', which matches the issue description")
	}
}

func TestReproduction_SequenceCasting(t *testing.T) {
	// Setup engine
	config := DefaultCastingConfig()
	config.SequenceDelay = 10 * time.Millisecond
	engine := NewCastingEngine(config)

	// Mock client manager
	cm := NewClientManager(engine)
	engine.SetClientManager(cm)

	// Request a sequence
	request := &CastRequest{
		Type:      CastTypeSequence,
		SpellName: "Protectra V", // First spell
		Target:    "Mysticminion",
		Priority:  1,
		Context: &CastContext{
			CasterName: "BotCaster",
			CasterMP:   1000,
		},
	}

	// Manually set up the active cast since we're testing NotifySpellComplete
	requestID := "buff_123"
	activeCast := &ActiveCast{
		Request:           request,
		State:             CastStateInProgress,
		SpellsInSequence:  []string{"Protectra V", "Shellra V", "Barthundra"},
		CurrentSpellIndex: 0,
		StartTime:         time.Now(),
	}

	engine.mu.Lock()
	engine.activeCasts[requestID] = activeCast
	engine.mu.Unlock()

	// 1. Complete first spell
	engine.NotifySpellComplete(requestID, true, "")

	// Check if advanced to second spell
	engine.mu.RLock()
	if activeCast.CurrentSpellIndex != 1 {
		t.Errorf("Expected CurrentSpellIndex 1, got %d", activeCast.CurrentSpellIndex)
	}
	if activeCast.Request.SpellName != "Shellra V" {
		t.Errorf("Expected SpellName 'Shellra V', got '%s'", activeCast.Request.SpellName)
	}
	if activeCast.State != CastStatePending {
		t.Errorf("Expected State CastStatePending, got %v", activeCast.State)
	}
	engine.mu.RUnlock()

	// Wait for sequence delay and processing
	time.Sleep(50 * time.Millisecond)

	// 2. Complete second spell
	engine.NotifySpellComplete(requestID, true, "")

	engine.mu.RLock()
	if activeCast.CurrentSpellIndex != 2 {
		t.Errorf("Expected CurrentSpellIndex 2, got %d", activeCast.CurrentSpellIndex)
	}
	if activeCast.Request.SpellName != "Barthundra" {
		t.Errorf("Expected SpellName 'Barthundra', got '%s'", activeCast.Request.SpellName)
	}
	engine.mu.RUnlock()

	// 3. Complete third (final) spell
	engine.NotifySpellComplete(requestID, true, "")

	engine.mu.RLock()
	if activeCast.State != CastStateCompleted {
		t.Errorf("Expected State CastStateCompleted, got %v", activeCast.State)
	}
	engine.mu.RUnlock()
}
