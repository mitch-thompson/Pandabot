package casting

import (
	"fmt"
	"testing"
	"time"

	"PandaBot/internal/entity"
)

func TestCastingEngine_RequestCast(t *testing.T) {
	engine := NewCastingEngine(DefaultCastingConfig())

	// Test manual cast request
	request := &CastRequest{
		ID:        "test_cast_1",
		Type:      CastTypeManual,
		SpellName: "Cure",
		Target:    "TestPlayer",
		Priority:  5,
		Timeout:   10 * time.Second,
		Context: &CastContext{
			CasterMP:        100,
			CasterJobLevels: map[string]int{"WHM": 30},
			CasterName:      "TestCaster",
		},
	}

	err := engine.RequestCast(request)
	if err != nil {
		t.Fatalf("Failed to request cast: %v", err)
	}

	// Check that cast is active
	activeCasts := engine.GetActiveCasts()
	if len(activeCasts) != 1 {
		t.Fatalf("Expected 1 active cast, got %d", len(activeCasts))
	}

	if _, exists := activeCasts["test_cast_1"]; !exists {
		t.Fatalf("Cast request not found in active casts")
	}
}

func TestCastingEngine_CureSelection(t *testing.T) {
	// Use config with lower MP reservation for testing
	config := DefaultCastingConfig()
	config.MPReservation = 10 // Lower reservation for testing
	engine := NewCastingEngine(config)

	// Create test entity needing healing
	targetEntity := &entity.Entity{
		Name:      "TestPlayer",
		HPPercent: 30.0, // Low HP
		HPMax:     1000,
		HPcurrent: 300,
	}

	// Test cure cast request
	request := &CastRequest{
		ID:       "test_cure_1",
		Type:     CastTypeCure,
		Target:   "TestPlayer",
		Priority: 8,
		Context: &CastContext{
			CasterMP:        100,                       // Should be enough for Cure (8 MP) with 10 MP reservation
			CasterJobLevels: map[string]int{"WHM": 75}, // Higher level to access more spells
			CasterName:      "TestCaster",
			TargetEntity:    targetEntity,
		},
	}

	err := engine.RequestCast(request)
	if err != nil {
		t.Fatalf("Failed to request cure cast: %v", err)
	}

	// Check that a spell was selected
	activeCasts := engine.GetActiveCasts()
	if cast, exists := activeCasts["test_cure_1"]; exists {
		if cast.Request.SpellName == "" {
			t.Fatalf("No spell selected for cure cast")
		}
		t.Logf("Selected cure spell: %s", cast.Request.SpellName)
	} else {
		t.Fatalf("Cure cast request not found")
	}
}

func TestCastingEngine_BuffSelection(t *testing.T) {
	engine := NewCastingEngine(DefaultCastingConfig())

	// Test buff cast request
	request := &CastRequest{
		ID:       "test_buff_1",
		Type:     CastTypeBuff,
		Target:   "TestPlayer",
		Priority: 3,
		Context: &CastContext{
			CasterMP:        300,
			CasterJobLevels: map[string]int{"WHM": 60, "RDM": 30},
			CasterName:      "TestCaster",
			PartySize:       4,
			BuffType:        "firebuffs",
		},
	}

	err := engine.RequestCast(request)
	if err != nil {
		t.Fatalf("Failed to request buff cast: %v", err)
	}

	// Check that spell(s) were selected
	activeCasts := engine.GetActiveCasts()
	if cast, exists := activeCasts["test_buff_1"]; exists {
		if cast.Request.SpellName == "" {
			t.Fatalf("No spell selected for buff cast")
		}
		t.Logf("Selected buff spell: %s", cast.Request.SpellName)

		// Check if it's a sequence
		if cast.Request.Type == CastTypeSequence {
			t.Logf("Buff sequence: %v", cast.SpellsInSequence)
		}
	} else {
		t.Fatalf("Buff cast request not found")
	}
}

func TestCastingEngine_NaSpellSelection(t *testing.T) {
	engine := NewCastingEngine(DefaultCastingConfig())

	// Test na spell cast request
	request := &CastRequest{
		ID:       "test_na_1",
		Type:     CastTypeNa,
		Target:   "TestPlayer",
		Priority: 9,
		Context: &CastContext{
			CasterMP:        150,
			CasterJobLevels: map[string]int{"WHM": 40},
			CasterName:      "TestCaster",
			StatusEffects:   []int{4, 6}, // Paralysis and Silence
		},
	}

	err := engine.RequestCast(request)
	if err != nil {
		t.Fatalf("Failed to request na spell cast: %v", err)
	}

	// Check that a spell was selected
	activeCasts := engine.GetActiveCasts()
	if cast, exists := activeCasts["test_na_1"]; exists {
		if cast.Request.SpellName == "" {
			t.Fatalf("No spell selected for na cast")
		}
		t.Logf("Selected na spell: %s", cast.Request.SpellName)
	} else {
		t.Fatalf("Na cast request not found")
	}
}

func TestCastingEngine_CancelCast(t *testing.T) {
	engine := NewCastingEngine(DefaultCastingConfig())

	// Create a cast request
	request := &CastRequest{
		ID:        "test_cancel_1",
		Type:      CastTypeManual,
		SpellName: "Cure",
		Target:    "TestPlayer",
		Priority:  5,
		Context: &CastContext{
			CasterMP:        100,
			CasterJobLevels: map[string]int{"WHM": 30},
			CasterName:      "TestCaster",
		},
	}

	err := engine.RequestCast(request)
	if err != nil {
		t.Fatalf("Failed to request cast: %v", err)
	}

	// Cancel the cast
	err = engine.CancelCast("test_cancel_1")
	if err != nil {
		t.Fatalf("Failed to cancel cast: %v", err)
	}

	// Check that cast was cancelled
	activeCasts := engine.GetActiveCasts()
	if cast, exists := activeCasts["test_cancel_1"]; exists {
		if cast.State != CastStateCancelled {
			t.Fatalf("Cast was not cancelled, state: %v", cast.State)
		}
	}
}

func TestCastingHelper_ConvenienceMethods(t *testing.T) {
	// Use config with lower MP reservation for testing
	config := DefaultCastingConfig()
	config.MPReservation = 10
	engine := NewCastingEngine(config)
	clientManager := NewClientManager(engine)
	helper := NewCastingHelper(engine, clientManager)

	// Test cure casting
	requestID, err := helper.CastCureByDamage("TestPlayer", 500, 100, map[string]int{"WHM": 75}, 8, nil)
	if err != nil {
		t.Fatalf("Failed to cast cure: %v", err)
	}

	if requestID == "" {
		t.Fatalf("No request ID returned for cure cast")
	}

	// Test buff casting
	requestID, err = helper.CastBuffs("TestPlayer", "firebuffs", 300, map[string]int{"WHM": 60}, 4, 3)
	if err != nil {
		t.Fatalf("Failed to cast buffs: %v", err)
	}

	if requestID == "" {
		t.Fatalf("No request ID returned for buff cast")
	}

	// Test na spell casting
	requestID, err = helper.CastNaSpell("TestPlayer", []int{4, 6}, 150, map[string]int{"WHM": 75}, 9)
	if err != nil {
		t.Fatalf("Failed to cast na spell: %v", err)
	}

	if requestID == "" {
		t.Fatalf("No request ID returned for na spell cast")
	}

	// Check statistics
	stats := engine.GetStats()
	if stats["active_casts"].(int) == 0 {
		t.Fatalf("Expected active casts, got 0")
	}

	t.Logf("Casting engine stats: %+v", stats)
}

func TestCastingEngine_SequenceCasting(t *testing.T) {
	engine := NewCastingEngine(DefaultCastingConfig())

	// Test sequence casting with firebuffs
	request := &CastRequest{
		ID:       "test_sequence_1",
		Type:     CastTypeBuff,
		Target:   "TestCaster",
		Priority: 3,
		Context: &CastContext{
			CasterMP:        300,
			CasterJobLevels: map[string]int{"WHM": 60, "RDM": 30},
			CasterName:      "TestCaster",
			PartySize:       4,
			BuffType:        "firebuffs",
		},
	}

	err := engine.RequestCast(request)
	if err != nil {
		t.Fatalf("Failed to request buff sequence cast: %v", err)
	}

	// Check that it was converted to a sequence
	activeCasts := engine.GetActiveCasts()
	cast, exists := activeCasts["test_sequence_1"]
	if !exists {
		t.Fatalf("Sequence cast request not found")
	}

	if cast.Request.Type != CastTypeSequence {
		t.Fatalf("Expected CastTypeSequence, got %v", cast.Request.Type)
	}

	if len(cast.SpellsInSequence) == 0 {
		t.Fatalf("No spells in sequence")
	}

	expectedSpells := []string{"Protectra III", "Shellra III", "Barfira"}
	if len(cast.SpellsInSequence) != len(expectedSpells) {
		t.Fatalf("Expected %d spells in sequence, got %d", len(expectedSpells), len(cast.SpellsInSequence))
	}

	t.Logf("Sequence spells: %v", cast.SpellsInSequence)
	t.Logf("Current spell: %s (index %d)", cast.Request.SpellName, cast.CurrentSpellIndex)

	// Simulate completing each spell in the sequence
	for i := 0; i < len(expectedSpells); i++ {
		// Verify current spell
		activeCasts = engine.GetActiveCasts()
		currentCast, exists := activeCasts["test_sequence_1"]
		if !exists {
			if i == len(expectedSpells)-1 {
				// Last spell completed, cast should be removed
				t.Logf("Sequence completed successfully, cast removed from active casts")
				break
			} else {
				t.Fatalf("Cast disappeared unexpectedly at spell %d", i)
			}
		}

		if currentCast.CurrentSpellIndex != i {
			t.Fatalf("Expected spell index %d, got %d", i, currentCast.CurrentSpellIndex)
		}

		if currentCast.Request.SpellName != expectedSpells[i] {
			t.Fatalf("Expected spell %s, got %s", expectedSpells[i], currentCast.Request.SpellName)
		}

		t.Logf("Completing spell %d: %s", i+1, expectedSpells[i])

		// Simulate spell completion
		engine.NotifySpellComplete("test_sequence_1", true, "")

		// Small delay to allow goroutines to process
		time.Sleep(10 * time.Millisecond)
	}

	// Verify sequence is completely finished
	activeCasts = engine.GetActiveCasts()
	if _, exists := activeCasts["test_sequence_1"]; exists {
		t.Fatalf("Sequence should be completed and removed from active casts")
	}

	// Check cast history
	history := engine.GetCastHistory(10)
	if len(history) == 0 {
		t.Fatalf("Expected cast in history")
	}

	lastCast := history[len(history)-1]
	if lastCast.Request.ID != "test_sequence_1" {
		t.Fatalf("Expected last cast to be test_sequence_1, got %s", lastCast.Request.ID)
	}

	if lastCast.State != CastStateCompleted {
		t.Fatalf("Expected cast state to be completed, got %v", lastCast.State)
	}

	t.Logf("Sequence casting test completed successfully")
}

func TestCastingEngine_RealServerConditions(t *testing.T) {
	engine := NewCastingEngine(DefaultCastingConfig())

	// Test with low job levels to reproduce the issue
	request := &CastRequest{
		ID:       "buff_test_low_levels",
		Type:     CastTypeBuff,
		Target:   "Mysticminion",
		Priority: 3,
		Context: &CastContext{
			CasterMP:        300,
			CasterJobLevels: map[string]int{"WHM": 15}, // Very low level
			CasterName:      "Mysticminion",
			PartySize:       1,
			BuffType:        "firebuffs",
		},
	}

	err := engine.RequestCast(request)
	if err != nil {
		t.Fatalf("Failed to request buff cast: %v", err)
	}

	// Check what was selected
	activeCasts := engine.GetActiveCasts()
	cast, exists := activeCasts["buff_test_low_levels"]
	if !exists {
		t.Fatalf("Cast request not found")
	}

	t.Logf("Cast Type: %s", engine.castTypeToString(cast.Request.Type))
	t.Logf("Selected spell: %s", cast.Request.SpellName)
	if cast.Request.Type == CastTypeSequence {
		t.Logf("Sequence: %v", cast.SpellsInSequence)
	}

	// Test with empty job levels
	request2 := &CastRequest{
		ID:       "buff_test_empty_levels",
		Type:     CastTypeBuff,
		Target:   "Mysticminion",
		Priority: 3,
		Context: &CastContext{
			CasterMP:        300,
			CasterJobLevels: map[string]int{}, // Empty job levels
			CasterName:      "Mysticminion",
			PartySize:       1,
			BuffType:        "firebuffs",
		},
	}

	err2 := engine.RequestCast(request2)
	t.Logf("Empty job levels result: %v", err2)
}

func TestCastingEngine_QueueDebugLogging(t *testing.T) {
	engine := NewCastingEngine(DefaultCastingConfig())

	// Create multiple cast requests to see queue behavior
	requests := []*CastRequest{
		{
			ID:        "cure_request",
			Type:      CastTypeManual,
			SpellName: "Cure III",
			Target:    "Player1",
			Priority:  8,
			Context: &CastContext{
				CasterMP:        200,
				CasterJobLevels: map[string]int{"WHM": 50},
				CasterName:      "TestCaster",
			},
		},
		{
			ID:       "buff_request",
			Type:     CastTypeBuff,
			Target:   "TestCaster",
			Priority: 3,
			Context: &CastContext{
				CasterMP:        300,
				CasterJobLevels: map[string]int{"WHM": 60},
				CasterName:      "TestCaster",
				PartySize:       4,
				BuffType:        "firebuffs",
			},
		},
		{
			ID:        "na_request",
			Type:      CastTypeManual,
			SpellName: "Paralyna",
			Target:    "Player2",
			Priority:  9,
			Context: &CastContext{
				CasterMP:        150,
				CasterJobLevels: map[string]int{"WHM": 40},
				CasterName:      "TestCaster",
			},
		},
	}

	// Submit all requests
	for _, request := range requests {
		err := engine.RequestCast(request)
		if err != nil {
			t.Fatalf("Failed to submit request %s: %v", request.ID, err)
		}
		time.Sleep(10 * time.Millisecond) // Small delay to see queue changes
	}

	// Simulate completing the cure request
	time.Sleep(50 * time.Millisecond)
	engine.NotifySpellComplete("cure_request", true, "")

	// Simulate completing the na request
	time.Sleep(50 * time.Millisecond)
	engine.NotifySpellComplete("na_request", true, "")

	// Simulate completing the first spell in the buff sequence
	time.Sleep(50 * time.Millisecond)
	engine.NotifySpellComplete("buff_request", true, "")

	// Complete the second spell in the sequence
	time.Sleep(50 * time.Millisecond)
	engine.NotifySpellComplete("buff_request", true, "")

	// Complete the final spell in the sequence
	time.Sleep(50 * time.Millisecond)
	engine.NotifySpellComplete("buff_request", true, "")

	// Verify all casts are completed
	time.Sleep(100 * time.Millisecond)
	activeCasts := engine.GetActiveCasts()
	if len(activeCasts) != 0 {
		t.Fatalf("Expected 0 active casts, got %d", len(activeCasts))
	}

	t.Logf("Queue debug logging test completed successfully")
}

func TestCastingEngine_Configuration(t *testing.T) {
	// Test custom configuration
	config := &CastingConfig{
		DefaultTimeout:     15 * time.Second,
		MaxConcurrentCasts: 3,
		RetryAttempts:      1,
		RetryDelay:         1 * time.Second,
		MPReservation:      25,
	}

	engine := NewCastingEngine(config)

	// Test that configuration is applied
	if engine.config.MaxConcurrentCasts != 3 {
		t.Fatalf("Expected MaxConcurrentCasts=3, got %d", engine.config.MaxConcurrentCasts)
	}

	if engine.config.MPReservation != 25 {
		t.Fatalf("Expected MPReservation=25, got %d", engine.config.MPReservation)
	}

	// Test concurrent cast limit
	for i := 0; i < 5; i++ {
		request := &CastRequest{
			ID:        fmt.Sprintf("test_concurrent_%d", i),
			Type:      CastTypeManual,
			SpellName: "Cure",
			Target:    "TestPlayer",
			Priority:  5,
			Context: &CastContext{
				CasterMP:        100,
				CasterJobLevels: map[string]int{"WHM": 75},
				CasterName:      "TestCaster",
			},
		}

		err := engine.RequestCast(request)
		if i < 3 {
			// First 3 should succeed
			if err != nil {
				t.Fatalf("Cast %d should have succeeded: %v", i, err)
			}
		} else {
			// 4th and 5th should fail due to limit
			if err == nil {
				t.Fatalf("Cast %d should have failed due to concurrent limit", i)
			}
		}
	}
}
