package textParser

import (
	"PandaBot/internal/protocol"
	"fmt"
	"strings"
	"sync"
)

// TriggerEvent represents a generic trigger event detected in chat
type TriggerEvent struct {
	TriggerType string // Type of trigger (e.g., "stoned", "firebuffs", "heal")
	Sender      string // Player who sent the trigger message
	Priority    int    // Priority level (1-10)
}

// TextParser analyzes chat messages for trigger words and creates generic trigger events
type TextParser struct {
	triggerMap       map[string]int // Maps trigger words to priority levels
	authorizedUsers  map[string]bool
	mu               sync.RWMutex
}

// NewTextParser creates a new text parser with default trigger mappings
func NewTextParser() *TextParser {
	parser := &TextParser{
		triggerMap:      make(map[string]int),
		authorizedUsers: make(map[string]bool),
	}
	
	// Initialize default trigger mappings
	parser.initializeDefaultTriggers()
	
	return parser
}

// initializeDefaultTriggers sets up the default trigger word mappings
func (tp *TextParser) initializeDefaultTriggers() {
	// Status removal triggers
	tp.triggerMap["stoned"] = 8
	tp.triggerMap["paralyzed"] = 7
	tp.triggerMap["silenced"] = 6
	tp.triggerMap["poisoned"] = 5
	tp.triggerMap["blinded"] = 4
	
	// Buff triggers
	tp.triggerMap["firebuffs"] = 3
	tp.triggerMap["waterbuffs"] = 3
	tp.triggerMap["thunderbuffs"] = 3
	tp.triggerMap["earthbuffs"] = 3
	tp.triggerMap["windbuffs"] = 3
	tp.triggerMap["icebuffs"] = 3
	tp.triggerMap["lightbuffs"] = 3
	tp.triggerMap["darkbuffs"] = 3
	
	// Healing triggers
	tp.triggerMap["heal"] = 9
	tp.triggerMap["cure"] = 9
	tp.triggerMap["help"] = 8
}

// ParseMessage analyzes a chat message and returns generic trigger events
func (tp *TextParser) ParseMessage(chatLine *protocol.ChatLine) ([]TriggerEvent, error) {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	
	if chatLine == nil {
		return nil, fmt.Errorf("chat line cannot be nil")
	}
	
	// TODO: Re-enable authorization check once user management is implemented
	// Check if sender is authorized
	// if !tp.IsAuthorized(chatLine.Sender) {
	//	return nil, fmt.Errorf("unauthorized sender: %s", chatLine.Sender)
	// }
	
	var events []TriggerEvent
	normalizedMessage := tp.normalizeString(chatLine.Message)
	
	// Check for trigger words in the message
	for trigger, priority := range tp.triggerMap {
		if tp.containsTrigger(normalizedMessage, trigger) {
			event := TriggerEvent{
				TriggerType: trigger,
				Sender:      chatLine.Sender,
				Priority:    priority,
			}
			events = append(events, event)
		}
	}
	
	return events, nil
}

// AddTrigger adds or updates a trigger word mapping
func (tp *TextParser) AddTrigger(trigger string, priority int) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	
	tp.triggerMap[tp.normalizeString(trigger)] = priority
}

// RemoveTrigger removes a trigger word mapping
func (tp *TextParser) RemoveTrigger(trigger string) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	
	delete(tp.triggerMap, tp.normalizeString(trigger))
}

// AddAuthorizedUser adds a user to the authorized users list
func (tp *TextParser) AddAuthorizedUser(username string) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	
	tp.authorizedUsers[strings.ToLower(username)] = true
}

// RemoveAuthorizedUser removes a user from the authorized users list
func (tp *TextParser) RemoveAuthorizedUser(username string) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	
	delete(tp.authorizedUsers, strings.ToLower(username))
}

// IsAuthorized checks if a user is authorized to trigger actions
func (tp *TextParser) IsAuthorized(username string) bool {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	
	// If no users are specifically authorized, allow all users
	if len(tp.authorizedUsers) == 0 {
		return true
	}
	
	return tp.authorizedUsers[strings.ToLower(username)]
}

// GetTriggers returns a copy of all trigger mappings
func (tp *TextParser) GetTriggers() map[string]int {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	
	triggers := make(map[string]int)
	for k, v := range tp.triggerMap {
		triggers[k] = v
	}
	return triggers
}

// normalizeString normalizes a string for comparison
func (tp *TextParser) normalizeString(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// containsTrigger checks if a normalized message contains a trigger word
func (tp *TextParser) containsTrigger(normalizedMessage, trigger string) bool {
	normalizedTrigger := tp.normalizeString(trigger)
	
	// Check for exact word match (not just substring)
	words := strings.Fields(normalizedMessage)
	for _, word := range words {
		if word == normalizedTrigger {
			return true
		}
	}
	
	// Also check for substring match for compound words
	return strings.Contains(normalizedMessage, normalizedTrigger)
}
