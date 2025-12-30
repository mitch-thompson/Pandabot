package casting

import (
	"PandaBot/internal/entity"
	"strings"
	"testing"
)

type fakeEngine struct {
	lastRequest *CastRequest
}

func (e *fakeEngine) RequestCast(request *CastRequest) error {
	e.lastRequest = request
	return nil
}

func TestTriggerProcessor_EraseTrigger(t *testing.T) {
	// Setup trigger processor with a fake engine
	engine := &fakeEngine{}
	tp := NewTriggerProcessor(engine)

	// Mock data
	sender := "TestPlayer"
	priority := 7
	casterName := "PandaBot"
	casterMP := 500
	casterJobLevels := map[string]int{"WHM": 75}
	partyMembers := []*entity.Entity{
		{Name: sender},
	}
	partyMembers[0].Buffs[0] = 13 // 13 is Slow, which Erase removes

	// Process "erase" trigger
	requestIDs, err := tp.ProcessTriggerEvent("erase", sender, priority, casterName, casterMP, casterJobLevels, partyMembers)
	if err != nil {
		t.Fatalf("Unexpected error processing erase trigger: %v", err)
	}

	if len(requestIDs) != 1 {
		t.Fatalf("Expected 1 request ID, got %d", len(requestIDs))
	}

	if !strings.HasPrefix(requestIDs[0], "na_") {
		t.Errorf("Expected request ID to start with 'na_', got %s", requestIDs[0])
	}
}
