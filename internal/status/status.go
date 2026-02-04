package status

type StatusInfo struct {
	ID       int
	Name     string
	Severity int    // 1=minor, 2=moderate, 3=severe, 4=critical, 10=urgent
	NaSpell  string // The "na" spell to cure it
	IsBuff   bool   // True if it's a positive status effect
}

var Registry = map[int]StatusInfo{
	2:   {ID: 2, Name: "Sleep", Severity: 6, NaSpell: "Cure"},
	3:   {ID: 3, Name: "Poison", Severity: 2, NaSpell: "Poisona"},
	4:   {ID: 4, Name: "Paralysis", Severity: 3, NaSpell: "Paralyna"},
	5:   {ID: 5, Name: "Blindness", Severity: 2, NaSpell: "Blindna"},
	6:   {ID: 6, Name: "Silence", Severity: 3, NaSpell: "Silena"},
	7:   {ID: 7, Name: "Petrification", Severity: 4, NaSpell: "Stona"},
	8:   {ID: 8, Name: "Disease", Severity: 2, NaSpell: "Viruna"},
	9:   {ID: 9, Name: "Curse", Severity: 3, NaSpell: "Cursna"},
	10:  {ID: 10, Name: "Silence", Severity: 3, NaSpell: "Silena"}, // Duplicate for some FFXI versions?
	11:  {ID: 11, Name: "Bind", Severity: 2, NaSpell: "Erase"},
	12:  {ID: 12, Name: "Weight", Severity: 2, NaSpell: "Erase"},
	13:  {ID: 13, Name: "Slow", Severity: 2, NaSpell: "Erase"},
	14:  {ID: 14, Name: "Charm", Severity: 4, NaSpell: "Sleep"},
	17:  {ID: 17, Name: "Attack Down", Severity: 2, NaSpell: "Erase"},
	18:  {ID: 18, Name: "Evasion Down", Severity: 2, NaSpell: "Erase"},
	19:  {ID: 19, Name: "Defense Down", Severity: 2, NaSpell: "Erase"},
	20:  {ID: 20, Name: "Magic Def. Down", Severity: 2, NaSpell: "Erase"},
	21:  {ID: 21, Name: "Magic Atk. Down", Severity: 2, NaSpell: "Erase"},
	28:  {ID: 28, Name: "Terror", Severity: 4, NaSpell: ""},
	31:  {ID: 31, Name: "Plague", Severity: 3, NaSpell: "Viruna"},
	33:  {ID: 33, Name: "Haste", Severity: 1, NaSpell: "Haste", IsBuff: true},
	40:  {ID: 40, Name: "Protect", Severity: 1, NaSpell: "Protect", IsBuff: true},
	41:  {ID: 41, Name: "Shell", Severity: 1, NaSpell: "Shell", IsBuff: true},
	42:  {ID: 42, Name: "Regen", Severity: 1, NaSpell: "Regen", IsBuff: true},
	100: {ID: 100, Name: "Barfire", Severity: 1, NaSpell: "Barfira", IsBuff: true},
	101: {ID: 101, Name: "Barblizzard", Severity: 1, NaSpell: "Barblizzara", IsBuff: true},
	102: {ID: 102, Name: "Baraero", Severity: 1, NaSpell: "Baraera", IsBuff: true},
	103: {ID: 103, Name: "Barstone", Severity: 1, NaSpell: "Barstonra", IsBuff: true},
	104: {ID: 104, Name: "Barthunder", Severity: 1, NaSpell: "Barthundra", IsBuff: true},
	105: {ID: 105, Name: "Barwater", Severity: 1, NaSpell: "Barwatera", IsBuff: true},
	113: {ID: 113, Name: "Reraise", Severity: 1, NaSpell: "Reraise", IsBuff: true},
	129: {ID: 129, Name: "Reraise II", Severity: 1, NaSpell: "Reraise", IsBuff: true},
	141: {ID: 141, Name: "Reraise III", Severity: 1, NaSpell: "Reraise", IsBuff: true},
	173: {ID: 173, Name: "Ability", Severity: 0, NaSpell: "None", IsBuff: true},
	272: {ID: 272, Name: "Auspice", Severity: 1, NaSpell: "Auspice", IsBuff: true},
	358: {ID: 358, Name: "Light Arts", Severity: 1, NaSpell: "Light Arts", IsBuff: true},
	359: {ID: 359, Name: "Dark Arts", Severity: 1, NaSpell: "Dark Arts", IsBuff: true},
	417: {ID: 417, Name: "Afflatus Solace", Severity: 1, NaSpell: "Afflatus Solace", IsBuff: true},
	418: {ID: 418, Name: "Afflatus Misery", Severity: 1, NaSpell: "Afflatus Misery", IsBuff: true},
}
