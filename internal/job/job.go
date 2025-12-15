package job

import (
	"PandaBot/internal/ability"
	"PandaBot/internal/spell"
)

type Job struct {
	ID        uint8
	Name      string
	Spells    []spell.Spell
	Abilities []ability.JobAbility
	//Traits    []Trait
}

// JobIDToName maps FFXI job IDs to job abbreviations
var JobIDToName = map[int]string{
	0:  "NON", // None
	1:  "WAR", // Warrior
	2:  "MNK", // Monk
	3:  "WHM", // White Mage
	4:  "BLM", // Black Mage
	5:  "RDM", // Red Mage
	6:  "THF", // Thief
	7:  "PLD", // Paladin
	8:  "DRK", // Dark Knight
	9:  "BST", // Beastmaster
	10: "BRD", // Bard
	11: "RNG", // Ranger
	12: "SAM", // Samurai
	13: "NIN", // Ninja
	14: "DRG", // Dragoon
	15: "SMN", // Summoner
	16: "BLU", // Blue Mage
	17: "COR", // Corsair
	18: "PUP", // Puppetmaster
	19: "DNC", // Dancer
	20: "SCH", // Scholar
	21: "GEO", // Geomancer
	22: "RUN", // Rune Fencer
}

// GetJobName returns the job name for a given job ID
func GetJobName(jobID int) string {
	if name, exists := JobIDToName[jobID]; exists {
		return name
	}
	return "UNK" // Unknown job
}

// GetJobID returns the job ID for a given job name
func GetJobID(jobName string) int {
	for id, name := range JobIDToName {
		if name == jobName {
			return id
		}
	}
	return 0 // Unknown job
}

//var AllJobs = map[string]*Job{
//	"WHM": &whmJob,
//	"RDM": &rdmJob,
//	"SCH": &schJob,
//}

func newJob(
	id uint8,
	name string,
	Spells []spell.Spell,
	Abilities []ability.JobAbility,
) *Job {
	return &Job{ID: id, Name: name}
}
