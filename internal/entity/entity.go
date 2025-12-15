package entity

type StatusID uint16

type Entity struct {
	ServerID    uint32
	Name        string
	HPPercent   uint8
	MPPercent   uint8
	HPcurrent   uint32
	HPMax       uint32
	Status      uint32     // bitmask of active statuses
	Buffs       [32]uint16 //buff IDs (up to 32)
	Distance    float32
	Zone        uint16
	Job         string
	JobLevel    uint8
	SubJob      string
	SubJobLevel uint8
}

func (e *Entity) HasStatus(id StatusID) bool {
	//todo
	return false
}

func (e *Entity) NeedsCure(threshold int) bool {
	return e.HPPercent < uint8(threshold)
}
