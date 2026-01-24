package action

type ActionType uint8

const (
	ActionTypeSpell ActionType = iota
	ActionTypeAbility
	ActionTypeItem
)

type TargetFlags uint8

const (
	TargetSelf TargetFlags = 1 << iota
	TargetPartyMember
	TargetPlayer
	TargetEnemy
	TargetAlly = TargetSelf | TargetPartyMember | TargetPlayer
	TargetAoE
	TargetCone
)

type Actionable interface {
	GetName() string
	GetID() uint16
	GetActionType() ActionType
	GetPriority() int
	GetTargetFlags() TargetFlags
}
