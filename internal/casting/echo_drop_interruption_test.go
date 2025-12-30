package casting

import (
	"testing"
)

func TestCastingEngine_QueueInterruption(t *testing.T) {
	engine := NewCastingEngine(DefaultCastingConfig())

	// 1. Add some low priority casts
	err := engine.RequestCast(&CastRequest{
		ID:        "low_1",
		Type:      CastTypeManual,
		SpellName: "Protect",
		Target:    "Player1",
		Priority:  3,
		Context: &CastContext{
			CasterMP:        100,
			CasterJobLevels: map[string]int{"WHM": 30},
		},
	})
	if err != nil {
		t.Fatalf("Failed to request low priority cast: %v", err)
	}

	err = engine.RequestCast(&CastRequest{
		ID:        "low_2",
		Type:      CastTypeManual,
		SpellName: "Shell",
		Target:    "Player1",
		Priority:  3,
		Context: &CastContext{
			CasterMP:        100,
			CasterJobLevels: map[string]int{"WHM": 30},
		},
	})
	if err != nil {
		t.Fatalf("Failed to request low priority cast: %v", err)
	}

	// Verify they are in active casts
	activeCasts := engine.GetActiveCasts()
	if len(activeCasts) != 2 {
		t.Fatalf("Expected 2 active casts, got %d", len(activeCasts))
	}

	// 2. Add a priority 10 cast (Echo Drop)
	err = engine.RequestCast(&CastRequest{
		ID:        "high_priority",
		Type:      CastTypeItem,
		SpellName: "Echo Drop",
		Target:    "<me>",
		Priority:  10,
		Context: &CastContext{
			CasterName: "TestCaster",
		},
	})
	if err != nil {
		t.Fatalf("Failed to request high priority cast: %v", err)
	}

	// 3. Verify queue was interrupted/cleared
	activeCasts = engine.GetActiveCasts()

	// Should only have "high_priority" now (or others should be cancelled)
	// Actually RequestCast deletes from map if priority 10
	if _, exists := activeCasts["low_1"]; exists {
		t.Errorf("low_1 should have been cancelled and removed from activeCasts")
	}
	if _, exists := activeCasts["low_2"]; exists {
		t.Errorf("low_2 should have been cancelled and removed from activeCasts")
	}
	if _, exists := activeCasts["high_priority"]; !exists {
		t.Errorf("high_priority should be in activeCasts")
	}

	if len(activeCasts) != 1 {
		t.Errorf("Expected 1 active cast, got %d", len(activeCasts))
	}
}

func TestCastingHelper_UseEchoDrop(t *testing.T) {
	engine := NewCastingEngine(DefaultCastingConfig())
	clientManager := NewClientManager(engine)
	helper := NewCastingHelper(engine, clientManager)

	requestID, err := helper.UseEchoDrop(10)
	if err != nil {
		t.Fatalf("Failed to use echo drop: %v", err)
	}

	activeCasts := engine.GetActiveCasts()
	if cast, exists := activeCasts[requestID]; exists {
		if cast.Request.Type != CastTypeItem {
			t.Errorf("Expected CastTypeItem, got %v", cast.Request.Type)
		}
		if cast.Request.SpellName != "Echo Drop" {
			t.Errorf("Expected Echo Drop, got %s", cast.Request.SpellName)
		}
		if cast.Request.Priority != 10 {
			t.Errorf("Expected priority 10, got %d", cast.Request.Priority)
		}
	} else {
		t.Errorf("Echo Drop cast request not found in active casts")
	}
}
