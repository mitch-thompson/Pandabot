package casting

import (
	"PandaBot/internal/entity"
	"testing"
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
