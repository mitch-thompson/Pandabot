package statusMonitor

import (
	"math/rand"
	"testing"
	"time"
)

// Test data generators for property testing

// generateRandomPartyMember creates a random party member for testing
func generateRandomPartyMember() (string, int, int, int, int, []int) {
	names := []string{"Alice", "Bob", "Charlie", "Diana", "Eve", "Frank"}
	name := names[rand.Intn(len(names))]
	hpPercent := rand.Intn(101)
	mpPercent := rand.Intn(101)
	job := rand.Intn(22) + 1 // Jobs 1-22
	zone := rand.Intn(300) + 1
	
	// Generate random status effects
	var statusIDs []int
	statusCount := rand.Intn(4) // 0-3 status effects
	possibleStatuses := []int{2, 3, 4, 5, 6, 7, 8, 9, 28, 31}
	
	for i := 0; i < statusCount; i++ {
		statusID := possibleStatuses[rand.Intn(len(possibleStatuses))]
		// Avoid duplicates
		found := false
		for _, existing := range statusIDs {
			if existing == statusID {
				found = true
				break
			}
		}
		if !found {
			statusIDs = append(statusIDs, statusID)
		}
	}
	
	return name, hpPercent, mpPercent, job, zone, statusIDs
}

// Property test: Status monitoring detects health thresholds correctly
func TestStatusMonitoringHealthThresholds(t *testing.T) {
	for i := 0; i < 100; i++ {
		monitor := NewStatusMonitor()
		
		// Test with various HP levels
		testCases := []struct {
			hp       int
			expected string
		}{
			{10, "critical"},
			{25, "critical"},
			{30, "low"},
			{50, "low"},
			{60, "medium"},
			{75, "medium"},
			{80, "healthy"},
			{100, "healthy"},
		}
		
		for _, tc := range testCases {
			threshold := monitor.GetHealthThreshold(tc.hp)
			if threshold != tc.expected {
				t.Errorf("Iteration %d: HP %d should be %s, got %s", i, tc.hp, tc.expected, threshold)
			}
		}
	}
}

// Property test: Status effect severity detection
func TestStatusEffectSeverityDetection(t *testing.T) {
	for i := 0; i < 100; i++ {
		monitor := NewStatusMonitor()
		
		// Create member with various status effects
		name, _, _, job, zone, _ := generateRandomPartyMember()
		
		// Test different severity combinations
		testCases := []struct {
			statusIDs      []int
			shouldNeedCure bool
			expectedSeverity int
		}{
			{[]int{}, false, 0},                    // No status effects
			{[]int{3}, true, 2},                    // Poison (moderate)
			{[]int{7}, true, 4},                    // Petrification (critical)
			{[]int{2, 4}, true, 3},                 // Sleep + Paralysis
			{[]int{3, 5, 8}, true, 2},              // Multiple moderate effects
		}
		
		for _, tc := range testCases {
			monitor.UpdatePartyMember(name, 100, 100, job, zone, tc.statusIDs)
			member, exists := monitor.GetPartyMember(name)
			
			if !exists {
				t.Errorf("Iteration %d: Member should exist after update", i)
				continue
			}
			
			if member.NeedsStatusRemoval != tc.shouldNeedCure {
				t.Errorf("Iteration %d: Status %v should need cure: %v, got %v", 
					i, tc.statusIDs, tc.shouldNeedCure, member.NeedsStatusRemoval)
			}
			
			if tc.shouldNeedCure {
				mostSevere := monitor.GetMostSevereStatusEffect(member)
				if mostSevere == nil {
					t.Errorf("Iteration %d: Should have most severe effect for %v", i, tc.statusIDs)
				} else if mostSevere.Severity < tc.expectedSeverity {
					t.Errorf("Iteration %d: Expected severity >= %d, got %d", 
						i, tc.expectedSeverity, mostSevere.Severity)
				}
			}
		}
	}
}

// Property test: Action triggering based on party state
func TestActionTriggeringBasedOnPartyState(t *testing.T) {
	for i := 0; i < 100; i++ {
		monitor := NewStatusMonitor()
		
		// Add multiple party members with different needs
		memberCount := rand.Intn(5) + 1
		expectedActions := 0
		
		for j := 0; j < memberCount; j++ {
			name, hp, mp, job, zone, statusIDs := generateRandomPartyMember()
			monitor.UpdatePartyMember(name, hp, mp, job, zone, statusIDs)
			
			// Count expected actions
			if hp < monitor.healthThresholds.Medium {
				expectedActions++
			}
			
			// Check if any status effects need removal
			needsStatusRemoval := false
			for _, statusID := range statusIDs {
				if effect, exists := monitor.statusEffects[statusID]; exists && effect.Severity >= 2 {
					needsStatusRemoval = true
					break
				}
			}
			if needsStatusRemoval {
				expectedActions++
			}
		}
		
		actions := monitor.CheckForActions()
		
		if len(actions) != expectedActions {
			t.Errorf("Iteration %d: Expected %d actions, got %d", i, expectedActions, len(actions))
		}
		
		// Verify action types and priorities
		for _, action := range actions {
			if action.Type != "cure" && action.Type != "na_spell" {
				t.Errorf("Iteration %d: Invalid action type: %s", i, action.Type)
			}
			
			if action.Priority <= 0 {
				t.Errorf("Iteration %d: Action priority should be positive, got %d", i, action.Priority)
			}
			
			if action.Target == "" {
				t.Errorf("Iteration %d: Action should have a target", i)
			}
			
			if action.Reason == "" {
				t.Errorf("Iteration %d: Action should have a reason", i)
			}
		}
	}
}

// Property test: Priority calculation consistency
func TestPriorityCalculationConsistency(t *testing.T) {
	for i := 0; i < 100; i++ {
		monitor := NewStatusMonitor()
		
		// Test job priority consistency
		jobs := []int{1, 3, 7, 15} // WAR, WHM, PLD, SMN
		priorities := make(map[int]int)
		
		for _, job := range jobs {
			priority := monitor.calculateMemberPriority(job)
			priorities[job] = priority
			
			if priority <= 0 {
				t.Errorf("Iteration %d: Job %d priority should be positive, got %d", i, job, priority)
			}
		}
		
		// WHM (job 3) should have highest priority
		if priorities[3] <= priorities[1] {
			t.Errorf("Iteration %d: WHM should have higher priority than WAR", i)
		}
		
		// Test healing priority scaling
		name := "TestPlayer"
		job := 3 // WHM
		
		testHPs := []int{10, 30, 60, 90}
		lastPriority := 0
		
		for _, hp := range testHPs {
			monitor.UpdatePartyMember(name, hp, 100, job, 1, []int{})
			member, _ := monitor.GetPartyMember(name)
			
			if member.NeedsHealing {
				threshold := monitor.GetHealthThreshold(hp)
				priority := monitor.calculateHealingPriority(member, threshold)
				
				// Lower HP should generally have higher priority
				if hp < 50 && priority <= lastPriority && lastPriority > 0 {
					t.Errorf("Iteration %d: Lower HP (%d) should have higher priority than %d", 
						i, hp, lastPriority)
				}
				
				lastPriority = priority
			}
		}
	}
}

// Property test: Party member lifecycle management
func TestPartyMemberLifecycleManagement(t *testing.T) {
	for i := 0; i < 100; i++ {
		monitor := NewStatusMonitor()
		
		// Add multiple members
		memberNames := []string{"Alice", "Bob", "Charlie"}
		
		for _, name := range memberNames {
			_, hp, mp, job, zone, statusIDs := generateRandomPartyMember()
			monitor.UpdatePartyMember(name, hp, mp, job, zone, statusIDs)
		}
		
		if monitor.GetPartyCount() != len(memberNames) {
			t.Errorf("Iteration %d: Expected %d members, got %d", i, len(memberNames), monitor.GetPartyCount())
		}
		
		// Remove one member
		monitor.RemovePartyMember(memberNames[0])
		
		if monitor.GetPartyCount() != len(memberNames)-1 {
			t.Errorf("Iteration %d: Expected %d members after removal, got %d", 
				i, len(memberNames)-1, monitor.GetPartyCount())
		}
		
		// Verify removed member is gone
		_, exists := monitor.GetPartyMember(memberNames[0])
		if exists {
			t.Errorf("Iteration %d: Removed member should not exist", i)
		}
		
		// Verify other members still exist
		for j := 1; j < len(memberNames); j++ {
			_, exists := monitor.GetPartyMember(memberNames[j])
			if !exists {
				t.Errorf("Iteration %d: Member %s should still exist", i, memberNames[j])
			}
		}
	}
}

// Property test: Stale member cleanup
func TestStaleMemberCleanup(t *testing.T) {
	for i := 0; i < 100; i++ {
		monitor := NewStatusMonitor()
		
		// Add members with different timestamps
		currentTime := time.Now()
		
		// Fresh member
		monitor.UpdatePartyMember("Fresh", 100, 100, 1, 1, []int{})
		
		// Stale member (simulate old timestamp)
		monitor.UpdatePartyMember("Stale", 100, 100, 1, 1, []int{})
		staleMember, _ := monitor.GetPartyMember("Stale")
		staleMember.LastSeen = currentTime.Add(-2 * time.Hour)
		
		// Very stale member
		monitor.UpdatePartyMember("VeryStale", 100, 100, 1, 1, []int{})
		veryStale, _ := monitor.GetPartyMember("VeryStale")
		veryStale.LastSeen = currentTime.Add(-5 * time.Hour)
		
		initialCount := monitor.GetPartyCount()
		if initialCount != 3 {
			t.Errorf("Iteration %d: Expected 3 initial members, got %d", i, initialCount)
		}
		
		// Clean up members older than 1 hour
		removed := monitor.CleanupStaleMembers(1 * time.Hour)
		
		if removed != 2 {
			t.Errorf("Iteration %d: Expected to remove 2 stale members, removed %d", i, removed)
		}
		
		if monitor.GetPartyCount() != 1 {
			t.Errorf("Iteration %d: Expected 1 member after cleanup, got %d", i, monitor.GetPartyCount())
		}
		
		// Verify fresh member still exists
		_, exists := monitor.GetPartyMember("Fresh")
		if !exists {
			t.Errorf("Iteration %d: Fresh member should still exist", i)
		}
	}
}

// Property test: Health threshold configuration
func TestHealthThresholdConfiguration(t *testing.T) {
	for i := 0; i < 100; i++ {
		monitor := NewStatusMonitor()
		
		// Test custom thresholds
		critical := rand.Intn(20) + 5  // 5-24
		low := critical + rand.Intn(20) + 10  // critical + 10-29
		medium := low + rand.Intn(20) + 10    // low + 10-29
		
		monitor.SetHealthThresholds(critical, low, medium)
		thresholds := monitor.GetHealthThresholds()
		
		if thresholds.Critical != critical {
			t.Errorf("Iteration %d: Critical threshold should be %d, got %d", i, critical, thresholds.Critical)
		}
		
		if thresholds.Low != low {
			t.Errorf("Iteration %d: Low threshold should be %d, got %d", i, low, thresholds.Low)
		}
		
		if thresholds.Medium != medium {
			t.Errorf("Iteration %d: Medium threshold should be %d, got %d", i, medium, thresholds.Medium)
		}
		
		// Test threshold logic with custom values
		testHP := critical - 1
		threshold := monitor.GetHealthThreshold(testHP)
		if threshold != "critical" {
			t.Errorf("Iteration %d: HP %d should be critical with threshold %d", i, testHP, critical)
		}
		
		testHP = (critical + low) / 2
		threshold = monitor.GetHealthThreshold(testHP)
		if threshold != "low" {
			t.Errorf("Iteration %d: HP %d should be low with thresholds %d-%d", i, testHP, critical, low)
		}
	}
}

// Benchmark status monitoring operations
func BenchmarkStatusMonitoringOperations(b *testing.B) {
	monitor := NewStatusMonitor()
	
	// Pre-populate with some members
	for i := 0; i < 6; i++ {
		name, hp, mp, job, zone, statusIDs := generateRandomPartyMember()
		monitor.UpdatePartyMember(name, hp, mp, job, zone, statusIDs)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate typical operations
		name, hp, mp, job, zone, statusIDs := generateRandomPartyMember()
		monitor.UpdatePartyMember(name, hp, mp, job, zone, statusIDs)
		
		actions := monitor.CheckForActions()
		_ = actions // Use the result
		
		if i%100 == 0 {
			monitor.CleanupStaleMembers(1 * time.Hour)
		}
	}
}