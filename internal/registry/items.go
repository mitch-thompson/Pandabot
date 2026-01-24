package registry

import (
	"PandaBot/internal/action"
	"fmt"
	"sync"
)

type Item struct {
	English  string
	ID       uint16
	Priority int
	Targets  action.TargetFlags
}

func (i *Item) GetName() string                    { return i.English }
func (i *Item) GetID() uint16                      { return i.ID }
func (i *Item) GetActionType() action.ActionType   { return action.ActionTypeItem }
func (i *Item) GetPriority() int                   { return i.Priority }
func (i *Item) GetTargetFlags() action.TargetFlags { return i.Targets }

var (
	items     = make(map[string]*Item)
	itemsByID = make(map[uint16]*Item)
	itemMu    sync.RWMutex
)

func init() {
	initializeItems()
}

func RegisterItem(i *Item) {
	itemMu.Lock()
	defer itemMu.Unlock()
	items[i.English] = i
	itemsByID[i.ID] = i
}

func GetItem(name string) (*Item, error) {
	itemMu.RLock()
	defer itemMu.RUnlock()
	i, ok := items[name]
	if !ok {
		return nil, fmt.Errorf("item not found: %s", name)
	}
	return i, nil
}

func GetItemByID(id uint16) (*Item, error) {
	itemMu.RLock()
	defer itemMu.RUnlock()
	i, ok := itemsByID[id]
	if !ok {
		return nil, fmt.Errorf("item ID not found: %d", id)
	}
	return i, nil
}

func initializeItems() {
	defaultItems := []*Item{
		{English: "Echo Drops", ID: 4113, Priority: 10, Targets: action.TargetSelf},
		{English: "Remedy", ID: 4114, Priority: 10, Targets: action.TargetSelf},
	}

	for _, i := range defaultItems {
		RegisterItem(i)
	}
}
