package casting

import (
	"testing"
)

func TestTargetResolution(t *testing.T) {
	engine := NewCastingEngine(DefaultCastingConfig())
	
	context := &CastContext{
		CasterName: "TestCaster",
		CasterMP:   300,
		CasterJobLevels: map[string]int{"WHM": 75},
	}
	
	// Test self-targeting spells (area spells)
	testCases := []struct {
		spellName      string
		originalTarget string
		expectedTarget string
		description    string
	}{
		{
			spellName:      "Protectra III",
			originalTarget: "SomePlayer",
			expectedTarget: "TestCaster",
			description:    "Area spell should target caster",
		},
		{
			spellName:      "Shellra II",
			originalTarget: "AnotherPlayer",
			expectedTarget: "TestCaster",
			description:    "Area spell should target caster",
		},
		{
			spellName:      "Curaga",
			originalTarget: "PartyMember",
			expectedTarget: "TestCaster",
			description:    "Curaga should target caster",
		},
		{
			spellName:      "Cure III",
			originalTarget: "InjuredPlayer",
			expectedTarget: "InjuredPlayer",
			description:    "Single-target cure should target original target",
		},
		{
			spellName:      "Paralyna",
			originalTarget: "ParalyzedPlayer",
			expectedTarget: "ParalyzedPlayer",
			description:    "Na spell should target original target",
		},
		{
			spellName:      "Barfira",
			originalTarget: "SomePlayer",
			expectedTarget: "TestCaster",
			description:    "Bar spell should target caster (self-only)",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			resolvedTarget, err := engine.resolveSpellTarget(tc.spellName, tc.originalTarget, context)
			if err != nil {
				t.Fatalf("Failed to resolve target for %s: %v", tc.spellName, err)
			}
			
			if resolvedTarget != tc.expectedTarget {
				t.Errorf("Target resolution failed for %s: expected %s, got %s", 
					tc.spellName, tc.expectedTarget, resolvedTarget)
			}
			
			t.Logf("✓ %s: %s -> %s", tc.spellName, tc.originalTarget, resolvedTarget)
		})
	}
}

func TestTargetResolutionFallback(t *testing.T) {
	engine := NewCastingEngine(DefaultCastingConfig())
	
	// Test with no caster name (should fallback to "me")
	context := &CastContext{
		CasterName: "", // Empty caster name
		CasterMP:   300,
		CasterJobLevels: map[string]int{"WHM": 75},
	}
	
	resolvedTarget, err := engine.resolveSpellTarget("Protectra III", "SomePlayer", context)
	if err != nil {
		t.Fatalf("Failed to resolve target: %v", err)
	}
	
	if resolvedTarget != "me" {
		t.Errorf("Expected fallback to 'me', got %s", resolvedTarget)
	}
	
	t.Logf("✓ Fallback working: empty caster name -> 'me'")
}

func TestTargetResolutionWithActualSpellData(t *testing.T) {
	engine := NewCastingEngine(DefaultCastingConfig())
	
	context := &CastContext{
		CasterName: "TestCaster",
		CasterMP:   300,
		CasterJobLevels: map[string]int{"WHM": 75},
	}
	
	// Test with actual cure spell that has proper target flags
	resolvedTarget, err := engine.resolveSpellTarget("Cure", "InjuredPlayer", context)
	if err != nil {
		t.Fatalf("Failed to resolve target for Cure: %v", err)
	}
	
	// Cure should target the original target since it's TargetAlly (not TargetSelf)
	if resolvedTarget != "InjuredPlayer" {
		t.Errorf("Cure should target original target, got %s", resolvedTarget)
	}
	
	t.Logf("✓ Cure spell targeting: InjuredPlayer -> %s", resolvedTarget)
}