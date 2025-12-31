package textParser

import (
	"PandaBot/internal/protocol"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Pre-compiled regex patterns for sender extraction
var (
	parensPattern   = regexp.MustCompile(`^\((?P<sender>[a-zA-Z]+)\)\s*(?P<msg>.*)$`)
	bracketsPattern = regexp.MustCompile(`^\[(?P<sender>[a-zA-Z]+)\]\s*(?P<msg>.*)$`)
	colonPattern    = regexp.MustCompile(`^(?P<sender>[^:]+):\s*(?P<msg>.*)$`)
)

// TriggerEvent represents a generic trigger event detected in chat
type TriggerEvent struct {
	TriggerType string // Type of trigger (e.g., "stoned", "firebuffs", "heal")
	Sender      string // Player who sent the trigger message
	Priority    int    // Priority level (1-10)
}

// TextParser analyzes chat messages for trigger words and creates generic trigger events
type TextParser struct {
	triggerMap map[string]int // Maps trigger words to priority levels
	mu         sync.RWMutex
}

// NewTextParser creates a new text parser with default trigger mappings
func NewTextParser() *TextParser {
	parser := &TextParser{
		triggerMap: make(map[string]int),
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
	tp.triggerMap["erase"] = 7
	tp.triggerMap["cursna"] = 8
	tp.triggerMap["cursed"] = 8
	tp.triggerMap["doom"] = 10
	tp.triggerMap["viruna"] = 7
	tp.triggerMap["diseased"] = 7
	tp.triggerMap["plagued"] = 7

	// Buff triggers
	tp.triggerMap["firebuffs"] = 3
	tp.triggerMap["waterbuffs"] = 3
	tp.triggerMap["thunderbuffs"] = 3
	tp.triggerMap["earthbuffs"] = 3
	tp.triggerMap["windbuffs"] = 3
	tp.triggerMap["icebuffs"] = 3
	tp.triggerMap["lightbuffs"] = 3
	tp.triggerMap["darkbuffs"] = 3
	tp.triggerMap["protect"] = 3
	tp.triggerMap["shell"] = 3
	tp.triggerMap["haste"] = 5
	tp.triggerMap["whmprep"] = 7
	tp.triggerMap["auspice"] = 4
	tp.triggerMap["reraise"] = 6
	tp.triggerMap["solace"] = 5
	tp.triggerMap["misery"] = 5
	tp.triggerMap["lightarts"] = 5
	tp.triggerMap["darkarts"] = 5

	// Healing triggers
	tp.triggerMap["heal"] = 9
	tp.triggerMap["cure"] = 9
	tp.triggerMap["help"] = 8

	// Control triggers
	tp.triggerMap["panda"] = 10
}

// ParseMessage analyzes a chat message and returns generic trigger events
func (tp *TextParser) ParseMessage(chatLine *protocol.ChatLine) ([]TriggerEvent, error) {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	if chatLine == nil {
		return nil, fmt.Errorf("chat line cannot be nil")
	}

	effectiveSender := chatLine.Sender
	effectiveMessage := chatLine.Message

	// If sender is "Unknown" (common in some party chat formats), try to extract it from the message
	if effectiveSender == "Unknown" || effectiveSender == "" {
		if extractedSender, extractedMsg, ok := tp.extractSender(effectiveMessage); ok {
			effectiveSender = extractedSender
			effectiveMessage = extractedMsg
		}
	}

	var events []TriggerEvent
	normalizedMessage := tp.normalizeString(effectiveMessage)

	// Check for trigger words in the message
	for trigger, priority := range tp.triggerMap {
		if tp.containsTrigger(normalizedMessage, trigger) {
			// Validate sender
			if effectiveSender == "" {
				continue
			}

			event := TriggerEvent{
				TriggerType: trigger,
				Sender:      effectiveSender,
				Priority:    priority,
			}
			events = append(events, event)
		}
	}

	return events, nil
}

// extractSender attempts to extract the sender name and actual message from a composite message
func (tp *TextParser) extractSender(message string) (string, string, bool) {
	patterns := []*regexp.Regexp{parensPattern, bracketsPattern, colonPattern}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(message)
		if matches != nil {
			senderIndex := pattern.SubexpIndex("sender")
			msgIndex := pattern.SubexpIndex("msg")

			if senderIndex != -1 && msgIndex != -1 {
				sender := strings.TrimSpace(matches[senderIndex])
				msg := matches[msgIndex]
				return sender, msg, true
			}
		}
	}

	return "", "", false
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
