package casting

import (
	"testing"
	"time"
)

func TestProtectShellResolution(t *testing.T) {
	// Initialize engine
	config := &CastingConfig{
		DefaultTimeout:     30 * time.Second,
		MaxConcurrentCasts: 1,
		RetryAttempts:      3,
		RetryDelay:         1 * time.Second,
		MPReservation:      10,
	}
	ce := NewCastingEngine(config)

	casterJobLevels := map[string]int{"WHM": 75}
	context := &CastContext{
		CasterMP:        500,
		CasterJobLevels: casterJobLevels,
		CasterName:      "Caster",
		PartySize:       1,
	}

	t.Run("ResolveProtect", func(t *testing.T) {
		request := &CastRequest{
			ID:       "test_protect",
			Type:     CastTypeProtect,
			Target:   "Sender",
			Priority: 3,
			Context:  context,
		}
		activeCast := &ActiveCast{Request: request}

		err := ce.resolveSpellSelection(activeCast)
		if err != nil {
			t.Fatalf("Failed to resolve Protect: %v", err)
		}

		if request.SpellName != "Protectra V" {
			t.Errorf("Expected Protectra V for WHM75, got %s", request.SpellName)
		}
	})

	t.Run("ResolveShell", func(t *testing.T) {
		request := &CastRequest{
			ID:       "test_shell",
			Type:     CastTypeShell,
			Target:   "Sender",
			Priority: 3,
			Context:  context,
		}
		activeCast := &ActiveCast{Request: request}

		err := ce.resolveSpellSelection(activeCast)
		if err != nil {
			t.Fatalf("Failed to resolve Shell: %v", err)
		}

		if request.SpellName != "Shellra V" {
			t.Errorf("Expected Shellra V for WHM75, got %s", request.SpellName)
		}
	})
}
