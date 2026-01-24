package registry

import (
	"PandaBot/internal/action"
	"PandaBot/internal/spell"
	"fmt"
	"sync"
)

var (
	spells     = make(map[string]*spell.Spell)
	spellsByID = make(map[uint16]*spell.Spell)
	spellMu    sync.RWMutex
)

func init() {
	initializeSpells()
}

func RegisterSpell(s *spell.Spell) {
	spellMu.Lock()
	defer spellMu.Unlock()
	spells[s.English] = s
	spellsByID[s.ID] = s
}

func GetSpell(name string) (*spell.Spell, error) {
	spellMu.RLock()
	defer spellMu.RUnlock()
	s, ok := spells[name]
	if !ok {
		return nil, fmt.Errorf("spell not found: %s", name)
	}
	return s, nil
}

func GetSpellByID(id uint16) (*spell.Spell, error) {
	spellMu.RLock()
	defer spellMu.RUnlock()
	s, ok := spellsByID[id]
	if !ok {
		return nil, fmt.Errorf("spell ID not found: %d", id)
	}
	return s, nil
}

func GetAllSpells() []*spell.Spell {
	spellMu.RLock()
	defer spellMu.RUnlock()
	all := make([]*spell.Spell, 0, len(spells))
	for _, s := range spells {
		all = append(all, s)
	}
	return all
}

func initializeSpells() {
	defaultSpells := []*spell.Spell{
		// Healing
		{English: "Cure", ID: 1, MPCost: 8, CastTime: 2.0, Type: spell.Healing, Element: spell.Light, Targets: action.TargetAlly, Priority: 1, HealAmount: 30, LevelReq: map[string]int{"WHM": 1, "RDM": 3, "PLD": 5, "SCH": 5}},
		{English: "Cure II", ID: 2, MPCost: 24, CastTime: 2.5, Type: spell.Healing, Element: spell.Light, Targets: action.TargetAlly, Priority: 2, HealAmount: 100, LevelReq: map[string]int{"WHM": 11, "RDM": 14, "PLD": 17, "SCH": 17}},
		{English: "Cure III", ID: 3, MPCost: 46, CastTime: 3.0, Type: spell.Healing, Element: spell.Light, Targets: action.TargetAlly, Priority: 3, HealAmount: 250, LevelReq: map[string]int{"WHM": 21, "RDM": 26, "PLD": 30, "SCH": 30}},
		{English: "Cure IV", ID: 4, MPCost: 88, CastTime: 3.5, Type: spell.Healing, Element: spell.Light, Targets: action.TargetAlly, Priority: 4, HealAmount: 480, LevelReq: map[string]int{"WHM": 41, "RDM": 48, "PLD": 55, "SCH": 55}},
		{English: "Cure V", ID: 5, MPCost: 135, CastTime: 4.0, Type: spell.Healing, Element: spell.Light, Targets: action.TargetAlly, Priority: 5, HealAmount: 780, LevelReq: map[string]int{"WHM": 61}},
		{English: "Cure VI", ID: 6, MPCost: 180, CastTime: 4.5, Type: spell.Healing, Element: spell.Light, Targets: action.TargetAlly, Priority: 6, HealAmount: 900, LevelReq: map[string]int{"WHM": 80}},

		// Curaga
		{English: "Curaga", ID: 7, MPCost: 60, CastTime: 3.0, Type: spell.Healing, Element: spell.Light, Targets: action.TargetSelf | action.TargetAoE, Priority: 7, HealAmount: 90, LevelReq: map[string]int{"WHM": 16}},
		{English: "Curaga II", ID: 8, MPCost: 120, CastTime: 3.5, Type: spell.Healing, Element: spell.Light, Targets: action.TargetSelf | action.TargetAoE, Priority: 8, HealAmount: 300, LevelReq: map[string]int{"WHM": 31}},
		{English: "Curaga III", ID: 9, MPCost: 180, CastTime: 4.0, Type: spell.Healing, Element: spell.Light, Targets: action.TargetSelf | action.TargetAoE, Priority: 9, HealAmount: 550, LevelReq: map[string]int{"WHM": 51}},
		{English: "Curaga IV", ID: 10, MPCost: 260, CastTime: 4.5, Type: spell.Healing, Element: spell.Light, Targets: action.TargetSelf | action.TargetAoE, Priority: 10, HealAmount: 800, LevelReq: map[string]int{"WHM": 71}},
		{English: "Curaga V", ID: 11, MPCost: 380, CastTime: 5.0, Type: spell.Healing, Element: spell.Light, Targets: action.TargetSelf | action.TargetAoE, Priority: 11, HealAmount: 1100, LevelReq: map[string]int{"WHM": 91}},

		// Status Removal
		{English: "Poisona", ID: 14, MPCost: 8, CastTime: 2.0, Type: spell.Healing, Element: spell.Light, Targets: action.TargetAlly, Priority: 1, LevelReq: map[string]int{"WHM": 6}},
		{English: "Paralyna", ID: 15, MPCost: 12, CastTime: 2.0, Type: spell.Healing, Element: spell.Light, Targets: action.TargetAlly, Priority: 2, LevelReq: map[string]int{"WHM": 9}},
		{English: "Blindna", ID: 16, MPCost: 16, CastTime: 2.0, Type: spell.Healing, Element: spell.Light, Targets: action.TargetAlly, Priority: 1, LevelReq: map[string]int{"WHM": 14}},
		{English: "Silena", ID: 17, MPCost: 24, CastTime: 2.0, Type: spell.Healing, Element: spell.Light, Targets: action.TargetAlly, Priority: 3, LevelReq: map[string]int{"WHM": 19}},
		{English: "Cursna", ID: 18, MPCost: 30, CastTime: 2.0, Type: spell.Healing, Element: spell.Light, Targets: action.TargetAlly, Priority: 2, LevelReq: map[string]int{"WHM": 20}},
		{English: "Viruna", ID: 19, MPCost: 36, CastTime: 2.0, Type: spell.Healing, Element: spell.Light, Targets: action.TargetAlly, Priority: 1, LevelReq: map[string]int{"WHM": 34}},
		{English: "Stona", ID: 20, MPCost: 40, CastTime: 2.0, Type: spell.Healing, Element: spell.Light, Targets: action.TargetAlly, Priority: 4, LevelReq: map[string]int{"WHM": 39}},
		{English: "Erase", ID: 21, MPCost: 32, CastTime: 2.0, Type: spell.Healing, Element: spell.Light, Targets: action.TargetAlly, Priority: 2, LevelReq: map[string]int{"WHM": 32}},

		// Buffs
		{English: "Protect", ID: 43, MPCost: 8, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetPartyMember, Priority: 1, LevelReq: map[string]int{"WHM": 7, "RDM": 7, "PLD": 10, "SCH": 10, "RUN": 20}},
		{English: "Protect II", ID: 44, MPCost: 18, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetPartyMember, Priority: 2, LevelReq: map[string]int{"WHM": 27, "RDM": 27, "PLD": 30, "SCH": 30, "RUN": 40}},
		{English: "Protect III", ID: 45, MPCost: 28, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetPartyMember, Priority: 3, LevelReq: map[string]int{"WHM": 47, "RDM": 47, "PLD": 50, "SCH": 50, "RUN": 60}},
		{English: "Protect IV", ID: 46, MPCost: 38, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetPartyMember, Priority: 4, LevelReq: map[string]int{"WHM": 63, "RDM": 63, "PLD": 70, "SCH": 66, "RUN": 80}},
		{English: "Protect V", ID: 47, MPCost: 48, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetPartyMember, Priority: 5, LevelReq: map[string]int{"WHM": 76, "RDM": 77, "PLD": 90, "SCH": 80}},

		{English: "Protectra", ID: 125, MPCost: 9, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 7}},
		{English: "Protectra II", ID: 126, MPCost: 20, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 2, LevelReq: map[string]int{"WHM": 27}},
		{English: "Protectra III", ID: 127, MPCost: 32, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 3, LevelReq: map[string]int{"WHM": 47}},
		{English: "Protectra IV", ID: 128, MPCost: 44, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 4, LevelReq: map[string]int{"WHM": 63}},
		{English: "Protectra V", ID: 129, MPCost: 56, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 5, LevelReq: map[string]int{"WHM": 75}},

		{English: "Shell", ID: 48, MPCost: 8, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetPartyMember, Priority: 1, LevelReq: map[string]int{"WHM": 17, "RDM": 17, "PLD": 20, "SCH": 20, "RUN": 10}},
		{English: "Shell II", ID: 49, MPCost: 18, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetPartyMember, Priority: 2, LevelReq: map[string]int{"WHM": 37, "RDM": 37, "PLD": 40, "SCH": 40, "RUN": 30}},
		{English: "Shell III", ID: 50, MPCost: 28, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetPartyMember, Priority: 3, LevelReq: map[string]int{"WHM": 57, "RDM": 57, "PLD": 60, "SCH": 60, "RUN": 50}},
		{English: "Shell IV", ID: 51, MPCost: 38, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetPartyMember, Priority: 4, LevelReq: map[string]int{"WHM": 68, "RDM": 68, "PLD": 80, "SCH": 71, "RUN": 70}},
		{English: "Shell V", ID: 52, MPCost: 48, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetPartyMember, Priority: 5, LevelReq: map[string]int{"WHM": 76, "RDM": 87, "SCH": 90, "RUN": 90}},

		{English: "Shellra", ID: 130, MPCost: 9, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 17}},
		{English: "Shellra II", ID: 131, MPCost: 20, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 2, LevelReq: map[string]int{"WHM": 37}},
		{English: "Shellra III", ID: 132, MPCost: 32, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 3, LevelReq: map[string]int{"WHM": 57}},
		{English: "Shellra IV", ID: 133, MPCost: 44, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 4, LevelReq: map[string]int{"WHM": 68}},
		{English: "Shellra V", ID: 134, MPCost: 56, CastTime: 3.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 5, LevelReq: map[string]int{"WHM": 75}},

		{English: "Barfira", ID: 64, MPCost: 6, CastTime: 1.0, Type: spell.Enhancing, Element: spell.Fire, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 9}},
		{English: "Barblizzara", ID: 65, MPCost: 6, CastTime: 1.0, Type: spell.Enhancing, Element: spell.Ice, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 14}},
		{English: "Baraera", ID: 66, MPCost: 6, CastTime: 1.0, Type: spell.Enhancing, Element: spell.Wind, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 19}},
		{English: "Barstonra", ID: 67, MPCost: 6, CastTime: 1.0, Type: spell.Enhancing, Element: spell.Earth, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 24}},
		{English: "Barthundra", ID: 68, MPCost: 6, CastTime: 1.0, Type: spell.Enhancing, Element: spell.Thunder, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 29}},
		{English: "Barwatera", ID: 69, MPCost: 6, CastTime: 1.0, Type: spell.Enhancing, Element: spell.Water, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 34}},
		{English: "Baramnesra", ID: 84, MPCost: 20, CastTime: 2.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 50}},
		{English: "Barvira", ID: 85, MPCost: 20, CastTime: 2.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 50}},
		{English: "Barsilencra", ID: 86, MPCost: 20, CastTime: 2.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 50}},
		{English: "Barparalyzra", ID: 87, MPCost: 20, CastTime: 2.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 50}},
		{English: "Barblindra", ID: 88, MPCost: 20, CastTime: 2.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 50}},
		{English: "Barpoisonra", ID: 89, MPCost: 20, CastTime: 2.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 50}},
		{English: "Barpetra", ID: 90, MPCost: 20, CastTime: 2.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 50}},
		{English: "Barsleepra", ID: 91, MPCost: 20, CastTime: 2.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 50}},

		{English: "Haste", ID: 57, MPCost: 40, CastTime: 2.0, Recast: 20.0, Type: spell.Enhancing, Element: spell.Wind, Targets: action.TargetPartyMember, Priority: 1, LevelReq: map[string]int{"WHM": 40, "RDM": 48, "BRD": 40, "SCH": 40, "GEO": 40, "RUN": 50}},
		{English: "Auspice", ID: 272, MPCost: 48, CastTime: 2.0, Recast: 10.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 55}},

		{English: "Reraise", ID: 113, MPCost: 150, CastTime: 8.0, Recast: 60.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 1, LevelReq: map[string]int{"WHM": 25, "SCH": 35}},
		{English: "Reraise II", ID: 129, MPCost: 150, CastTime: 8.0, Recast: 60.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 2, LevelReq: map[string]int{"WHM": 56, "SCH": 70}},
		{English: "Reraise III", ID: 141, MPCost: 150, CastTime: 8.0, Recast: 60.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetSelf, Priority: 3, LevelReq: map[string]int{"WHM": 70, "SCH": 91}},

		{English: "Regen", ID: 108, MPCost: 15, CastTime: 2.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetAlly, Priority: 1, LevelReq: map[string]int{"WHM": 21, "RDM": 44, "SCH": 20}},
		{English: "Regen II", ID: 109, MPCost: 36, CastTime: 2.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetAlly, Priority: 2, LevelReq: map[string]int{"WHM": 44, "SCH": 50}},
		{English: "Regen III", ID: 110, MPCost: 64, CastTime: 2.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetAlly, Priority: 3, LevelReq: map[string]int{"WHM": 66, "SCH": 80}},
		{English: "Regen IV", ID: 477, MPCost: 82, CastTime: 2.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetAlly, Priority: 4, LevelReq: map[string]int{"WHM": 86}},
		{English: "Regen V", ID: 478, MPCost: 100, CastTime: 2.0, Type: spell.Enhancing, Element: spell.Light, Targets: action.TargetAlly, Priority: 5, LevelReq: map[string]int{"WHM": 99}},

		// Blue Magic Healing
		{English: "Healing Breeze", ID: 581, MPCost: 55, CastTime: 4.5, Type: spell.BlueMagic, Element: spell.Wind, Targets: action.TargetSelf | action.TargetAoE, Priority: 1, HealAmount: 100, LevelReq: map[string]int{"BLU": 16}},
		{English: "Wild Carrot", ID: 591, MPCost: 37, CastTime: 3.0, Type: spell.BlueMagic, Element: spell.Light, Targets: action.TargetAlly, Priority: 1, HealAmount: 120, LevelReq: map[string]int{"BLU": 30}},
		{English: "Magic Fruit", ID: 630, MPCost: 72, CastTime: 4.0, Type: spell.BlueMagic, Element: spell.Light, Targets: action.TargetAlly, Priority: 2, HealAmount: 400, LevelReq: map[string]int{"BLU": 58}},
	}

	for _, s := range defaultSpells {
		RegisterSpell(s)
	}
}
