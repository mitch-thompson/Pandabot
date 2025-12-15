package main

import (
	"PandaBot/internal/casting"
	"PandaBot/internal/entity"
	"fmt"
)

func main() {
	// Test case: Character with incorrect HP percentage (100%) but actual HP shows missing HP
	target := &entity.Entity{
		Name:      "TestPlayer",
		HPPercent: 100, // Incorrect percentage from Lua
		HPMax:     900,
		HPcurrent: 600, // Actual values show 300 missing HP
		MPPercent: 80,
		Job:       "WAR",
		JobLevel:  75,
	}
	
	availableMP := 500
	jobLevel := map[string]int{"WHM": 75}
	
	fmt.Printf("Test case: Target with incorrect HP percentage\n")
	fmt.Printf("Target: %s\n", target.Name)
	fmt.Printf("HP: %d/%d (reported as %d%%, but actually missing %d HP)\n", 
		target.HPcurrent, target.HPMax, target.HPPercent, target.HPMax-target.HPcurrent)
	fmt.Printf("Available MP: %d\n", availableMP)
	fmt.Printf("Job Level: WHM %d\n\n", jobLevel["WHM"])
	
	// Test the full casting engine flow
	fmt.Println("=== Testing casting engine with incorrect HP percentage ===")
	engine := casting.NewCastingEngine(nil)
	
	context := &casting.CastContext{
		CasterMP:        availableMP + 50, // Add MP reservation
		CasterJobLevels: jobLevel,
		CasterName:      "TestCaster",
		TargetEntity:    target,
		PartyMembers:    []*entity.Entity{target},
		PartySize:       1,
		MissingHP:       32, // This is the incorrect value from percentage calculation
	}
	
	option, err := engine.SelectOptimalCure(context)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else if option != nil {
		fmt.Printf("SUCCESS: Selected %s (heal: %d, cost: %d MP)\n", 
			option.SpellName, option.HealAmount, option.MPCost)
		
		coverage := float64(option.HealAmount) / float64(target.HPMax-target.HPcurrent) * 100
		fmt.Printf("Coverage: %.1f%% of actual missing HP (%d)\n", 
			coverage, target.HPMax-target.HPcurrent)
		
		if option.SpellName == "Cure III" {
			fmt.Println("✓ CORRECT: Selected Cure III for 300 missing HP")
		} else {
			fmt.Printf("✗ INCORRECT: Selected %s instead of Cure III\n", option.SpellName)
		}
	}
}