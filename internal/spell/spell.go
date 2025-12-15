package spell

type Spell struct {
	English    string
	ID         uint16 // Packet spell ID
	MPCost     uint16
	CastTime   float32
	Recast     float32
	LevelReq   map[string]int // e.g. {"WHM": 17, "RDM": 21}
	Type       SpellType
	Element    Element
	Targets    TargetFlags
	Priority   int // for auto-selection (higher = more important)
	IsAoE      bool
	HealAmount int // For healing spells
}

type SpellType uint8

const (
	Healing SpellType = iota
	Enhancing
	Enfeebling
	DarkMagic
	Summoning
	Ninjutsu
	Singing
	BlueMagic
)

type Element uint8

const (
	Fire Element = iota
	Ice
	Wind
	Earth
	Thunder
	Water
	Light
	Dark
)

type TargetFlags uint8

const (
	TargetSelf       TargetFlags = 1 << iota // Can only target self (area spells like Protectra, Shellra)
	TargetPartyMember                        // Can target any party member (single target spells like Protect, Shell)
	TargetPlayer                             // Can target any player (including non-party members)
	TargetEnemy                              // Can target enemies
	TargetAlly       = TargetSelf | TargetPartyMember | TargetPlayer // Legacy: any friendly target
	TargetAoE
	TargetCone
)
