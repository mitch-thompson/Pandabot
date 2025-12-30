package cureSelector

import (
	"PandaBot/internal/entity"
	"fmt"
	"testing"
)

func TestUrgencyWeightedCureSelection(t *testing.T) {
	selector := NewCureSelector()
	jobLevel := map[string]int{"WHM": 75}
	availableMP := 500

	t.Run("Scenario 1: 1 critical + 5 near-full -> picks single-target", func(t *testing.T) {
		// 1 critical (800 missing) + 5 near-full (100 missing each)
		critical := &entity.Entity{
			Name:      "Critical",
			HPMax:     1000,
			HPcurrent: 200,
			HPPercent: 20,
		}

		party := []*entity.Entity{critical}
		for i := 0; i < 5; i++ {
			party = append(party, &entity.Entity{
				Name:      fmt.Sprintf("NearFull%d", i),
				HPMax:     1000,
				HPcurrent: 900,
				HPPercent: 90,
			})
		}

		// Currently SelectOptimalCure doesn't take party, so we simulate the call
		// we expect to have after refactoring.
		// For now, let's see what ShouldUseCuraga says.
		shouldUse, _, _ := selector.ShouldUseCuraga(party, availableMP, jobLevel)
		if shouldUse {
			t.Errorf("Should NOT prefer Curaga when only one member is critical")
		}
	})

	t.Run("Scenario 2: 4 critical + 2 near-full -> picks Curaga", func(t *testing.T) {
		// 4 critical (800 missing each) + 2 near-full
		party := []*entity.Entity{}
		for i := 0; i < 4; i++ {
			party = append(party, &entity.Entity{
				Name:      fmt.Sprintf("Critical%d", i),
				HPMax:     1000,
				HPcurrent: 200,
				HPPercent: 20,
			})
		}
		for i := 0; i < 2; i++ {
			party = append(party, &entity.Entity{
				Name:      fmt.Sprintf("NearFull%d", i),
				HPMax:     1000,
				HPcurrent: 900,
				HPPercent: 90,
			})
		}

		shouldUse, _, _ := selector.ShouldUseCuraga(party, availableMP, jobLevel)
		if !shouldUse {
			t.Errorf("Should prefer Curaga when 4 members are critical")
		}
	})
}
