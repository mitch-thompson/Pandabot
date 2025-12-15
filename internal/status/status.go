package status

type Status struct {
	ID        uint16
	Name      string
	Priority  int      // 100 = instance remove, 1 = ignore
	CanRemove []string // e.g. {"Ensuna", "Poisona", "Paralyna"}
	IconID    uint16
}

var RemovePriority = map[uint16]*Status{2: {
	Name: "Poison", Priority: 90, CanRemove: []string{"Poisona"}},
	7:  {Name: "Paralyze", Priority: 95, CanRemove: []string{"Paralyna"}},
	14: {Name: "Sleep", Priority: 100, CanRemove: []string{"Cure", "Curaga"}},
}
