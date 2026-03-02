package protocol

import (
	"encoding/json"
	"fmt"

	"PandaBot/internal/job"
)

type MessageType uint8

const (
	TypePing            MessageType = 1
	TypePong            MessageType = 2
	TypeExecuteCommand  MessageType = 10 // Go -> Ashita
	TypeChatLine        MessageType = 20 // Ashita -> Go (parsed chat)
	TypeStatusUpdate    MessageType = 21 // Ashita -> Go (party status)
	TypeErrorReport     MessageType = 30 // Ashita -> Go (command execution errors)
	TypeActionComplete  MessageType = 31 // Ashita -> Go (action completion notification)
	TypeActionFailed    MessageType = 32 // Ashita -> Go (action failure notification)
	TypeReadyToCast     MessageType = 40 // Go -> Ashita (check if ready)
	TypeReadyResponse   MessageType = 41 // Ashita -> Go (ready status)
	TypeReadyForAction  MessageType = 42 // Ashita -> Go (client is ready for next action)
	TypeStratagemUpdate MessageType = 50 // Ashita -> Go (stratagem recast info)
)

type ReadyForAction struct {
	PlayerName string `json:"player_name"`
	Timestamp  int64  `json:"timestamp"`
}

type ReadyToCast struct {
	CommandID string `json:"command_id"`
}

type ReadyResponse struct {
	CommandID string `json:"command_id"`
	IsReady   bool   `json:"is_ready"`
	Reason    string `json:"reason"` // Reason if not ready (e.g., "moving", "casting")
}

type Message struct {
	Type MessageType `json:"type"`
	Body any         `json:"body"`
}

// ExecuteCommand represents a command sent from server to client
type ExecuteCommand struct {
	Command   string `json:"command"`   // "/ma \"Cure IV\" <t>"
	Target    string `json:"target"`    // Player name or <t>, <me>
	Priority  int    `json:"priority"`  // Execution priority (1-100)
	Timeout   int    `json:"timeout"`   // Max execution time (ms)
	ID        string `json:"id"`        // Unique command ID for tracking
	Timestamp int64  `json:"timestamp"` // When the action was first queued
}

// ChatLine represents a chat message from client to server
type ChatLine struct {
	Mode      uint32 `json:"mode"`      // Chat channel type
	Sender    string `json:"sender"`    // Player name
	Message   string `json:"message"`   // Chat content
	Timestamp int64  `json:"timestamp"` // Message timestamp
}

// StatusUpdate represents party status information from client to server
type StatusUpdate struct {
	Timestamp      int64          `json:"timestamp"`
	PartyMembers   []PartyMember  `json:"party_members"`
	PlayerMP       int            `json:"player_mp"`
	PlayerHP       int            `json:"player_hp"`
	PlayerStatus   []int          `json:"player_status"` // Player's own status effects
	EchoDropCount  int            `json:"echo_drop_count"`
	Zone           string         `json:"zone"`
	JobLevels      map[string]int `json:"job_levels"` // Job name -> level mapping
	KnownSpells    []string       `json:"known_spells"`
	KnownAbilities []string       `json:"known_abilities"`
}

// PartyMember represents a single party member's status
type PartyMember struct {
	Name          string  `json:"name"`
	HPPercent     int     `json:"hp_percent"`
	MPPercent     int     `json:"mp_percent"`
	HPActual      int     `json:"hp_actual"` // Current HP value from Ashita v4
	HPMax         int     `json:"hp_max"`    // Max HP value from Ashita v4
	MPActual      int     `json:"mp_actual"` // Current MP value from Ashita v4
	MPMax         int     `json:"mp_max"`    // Max MP value from Ashita v4
	StatusEffects []int   `json:"status_effects"`
	Job           string  `json:"job"`
	Distance      float32 `json:"distance"`
	LastUpdate    int64   `json:"last_update"` // Unix timestamp
}

// ErrorReport represents command execution errors from client to server
type ErrorReport struct {
	CommandID string `json:"command_id"`
	Error     string `json:"error"`
	Timestamp int64  `json:"timestamp"`
}

// ActionComplete represents action completion notification from client to server
type ActionComplete struct {
	CommandID string `json:"command_id"`
	Timestamp int64  `json:"timestamp"`
}

// ActionFailed represents action failure notification from client to server
type ActionFailed struct {
	CommandID string `json:"command_id"`
	Error     string `json:"error"`
	Timestamp int64  `json:"timestamp"`
}

// StratagemUpdate represents stratagem recast information from client to server
type StratagemUpdate struct {
	Timer     int   `json:"timer"` // Stratagem recast timer in seconds
	Level     int   `json:"level"` // SCH job level
	Timestamp int64 `json:"timestamp"`
}

// Validation functions

// ValidateMessage validates a protocol message
func ValidateMessage(msg *Message) error {
	if msg == nil {
		return fmt.Errorf("message cannot be nil")
	}

	switch msg.Type {
	case TypePing, TypePong:
		// No body validation needed
		return nil
	case TypeExecuteCommand:
		return ValidateExecuteCommand(msg.Body)
	case TypeChatLine:
		return ValidateChatLine(msg.Body)
	case TypeStatusUpdate:
		return ValidateStatusUpdate(msg.Body)
	case TypeReadyForAction:
		return nil
	case TypeReadyToCast:
		return nil // Body is ReadyToCast struct
	case TypeReadyResponse:
		return nil // Body is ReadyResponse struct
	case TypeErrorReport:
		return ValidateErrorReport(msg.Body)
	case TypeActionComplete:
		return ValidateActionComplete(msg.Body)
	case TypeActionFailed:
		return ValidateActionFailed(msg.Body)
	default:
		return fmt.Errorf("unknown message type: %d", msg.Type)
	}
}

// ValidateExecuteCommand validates an ExecuteCommand message body
func ValidateExecuteCommand(body any) error {
	cmd, ok := body.(*ExecuteCommand)
	if !ok {
		return fmt.Errorf("invalid ExecuteCommand body type")
	}

	if cmd.Command == "" {
		return fmt.Errorf("command cannot be empty")
	}

	if cmd.Priority < 1 || cmd.Priority > 10 {
		return fmt.Errorf("priority must be between 1 and 10, got %d", cmd.Priority)
	}

	if cmd.Timeout < 0 {
		return fmt.Errorf("timeout cannot be negative")
	}

	return nil
}

// ValidateChatLine validates a ChatLine message body
func ValidateChatLine(body any) error {
	chat, ok := body.(*ChatLine)
	if !ok {
		return fmt.Errorf("invalid ChatLine body type")
	}

	if chat.Sender == "" {
		return fmt.Errorf("sender cannot be empty")
	}

	if chat.Message == "" {
		return fmt.Errorf("message cannot be empty")
	}

	if chat.Timestamp <= 0 {
		return fmt.Errorf("timestamp must be positive")
	}

	return nil
}

// ValidateStatusUpdate validates a StatusUpdate message body
func ValidateStatusUpdate(body any) error {
	status, ok := body.(*StatusUpdate)
	if !ok {
		return fmt.Errorf("invalid StatusUpdate body type")
	}

	if status.Timestamp <= 0 {
		return fmt.Errorf("timestamp must be positive")
	}

	if status.PlayerHP < 0 {
		return fmt.Errorf("player HP cannot be negative, got %d", status.PlayerHP)
	}

	if status.PlayerMP < 0 {
		return fmt.Errorf("player MP cannot be negative, got %d", status.PlayerMP)
	}

	for i, member := range status.PartyMembers {
		if err := ValidatePartyMember(&member); err != nil {
			return fmt.Errorf("party member %d validation failed: %w", i, err)
		}
	}

	return nil
}

// ValidatePartyMember validates a PartyMember
func ValidatePartyMember(member *PartyMember) error {
	if member.Name == "" {
		return fmt.Errorf("party member name cannot be empty")
	}

	if member.HPPercent < 0 || member.HPPercent > 100 {
		return fmt.Errorf("HP percent must be between 0 and 100, got %d", member.HPPercent)
	}

	if member.MPPercent < 0 || member.MPPercent > 100 {
		return fmt.Errorf("MP percent must be between 0 and 100, got %d", member.MPPercent)
	}

	if member.Distance < 0 {
		return fmt.Errorf("distance cannot be negative")
	}

	return nil
}

// ValidateErrorReport validates an ErrorReport message body
func ValidateErrorReport(body any) error {
	report, ok := body.(*ErrorReport)
	if !ok {
		return fmt.Errorf("invalid ErrorReport body type")
	}

	if report.CommandID == "" {
		return fmt.Errorf("command ID cannot be empty")
	}

	if report.Error == "" {
		return fmt.Errorf("error message cannot be empty")
	}

	if report.Timestamp <= 0 {
		return fmt.Errorf("timestamp must be positive")
	}

	return nil
}

// ValidateActionComplete validates an ActionComplete message body
func ValidateActionComplete(body any) error {
	complete, ok := body.(*ActionComplete)
	if !ok {
		return fmt.Errorf("invalid ActionComplete body type")
	}

	if complete.CommandID == "" {
		return fmt.Errorf("command ID cannot be empty")
	}

	if complete.Timestamp <= 0 {
		return fmt.Errorf("timestamp must be positive")
	}

	return nil
}

// ValidateActionFailed validates an ActionFailed message body
func ValidateActionFailed(body any) error {
	failed, ok := body.(*ActionFailed)
	if !ok {
		return fmt.Errorf("invalid ActionFailed body type")
	}

	if failed.CommandID == "" {
		return fmt.Errorf("command ID cannot be empty")
	}

	if failed.Error == "" {
		return fmt.Errorf("error message cannot be empty")
	}

	if failed.Timestamp <= 0 {
		return fmt.Errorf("timestamp must be positive")
	}

	return nil
}

// JSON serialization helpers

// MarshalMessage serializes a message to JSON
func MarshalMessage(msg *Message) ([]byte, error) {
	return json.Marshal(msg)
}

// UnmarshalMessage deserializes a message from JSON
func UnmarshalMessage(data []byte) (*Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// MarshalExecuteCommand serializes an ExecuteCommand to JSON
func UnmarshalReadyToCast(data []byte) (*ReadyToCast, error) {
	var body ReadyToCast
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, err
	}
	return &body, nil
}

func UnmarshalReadyResponse(data []byte) (*ReadyResponse, error) {
	var body ReadyResponse
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, err
	}
	return &body, nil
}

func MarshalReadyForAction(body *ReadyForAction) ([]byte, error) {
	return json.Marshal(body)
}

func UnmarshalReadyForAction(data []byte) (*ReadyForAction, error) {
	var body ReadyForAction
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, err
	}
	return &body, nil
}

func MarshalReadyToCast(body *ReadyToCast) ([]byte, error) {
	return json.Marshal(body)
}

func MarshalReadyResponse(body *ReadyResponse) ([]byte, error) {
	return json.Marshal(body)
}

func MarshalExecuteCommand(cmd *ExecuteCommand) ([]byte, error) {
	return json.Marshal(cmd)
}

// UnmarshalExecuteCommand deserializes an ExecuteCommand from JSON
func UnmarshalExecuteCommand(data []byte) (*ExecuteCommand, error) {
	var cmd ExecuteCommand
	err := json.Unmarshal(data, &cmd)
	if err != nil {
		return nil, err
	}
	return &cmd, nil
}

// MarshalChatLine serializes a ChatLine to JSON
func MarshalChatLine(chat *ChatLine) ([]byte, error) {
	return json.Marshal(chat)
}

// UnmarshalChatLine deserializes a ChatLine from JSON
func UnmarshalChatLine(data []byte) (*ChatLine, error) {
	var chat ChatLine
	err := json.Unmarshal(data, &chat)
	if err != nil {
		return nil, err
	}
	return &chat, nil
}

// MarshalStatusUpdate serializes a StatusUpdate to JSON
func MarshalStatusUpdate(status *StatusUpdate) ([]byte, error) {
	return json.Marshal(status)
}

// partyMemberRaw is used for unmarshaling JSON where job comes as an integer
type partyMemberRaw struct {
	Name          string  `json:"name"`
	HPPercent     int     `json:"hp_percent"`
	MPPercent     int     `json:"mp_percent"`
	HPActual      int     `json:"hp_actual"` // Actual HP value from Ashita v4
	HPMax         int     `json:"hp_max"`    // Max HP value from Ashita v4
	MPActual      int     `json:"mp_actual"` // Actual MP value from Ashita v4
	MPMax         int     `json:"mp_max"`    // Max MP value from Ashita v4
	StatusEffects []int   `json:"status_effects"`
	Job           int     `json:"job"` // Job ID from client
	Distance      float32 `json:"distance"`
	LastUpdate    int64   `json:"last_update"`
}

// statusUpdateRaw is used for unmarshaling JSON with raw party member data
type statusUpdateRaw struct {
	Timestamp      int64            `json:"timestamp"`
	PartyMembers   []partyMemberRaw `json:"party_members"`
	PlayerMP       int              `json:"player_mp"`
	PlayerHP       int              `json:"player_hp"`
	PlayerStatus   []int            `json:"player_status"`
	EchoDropCount  int              `json:"echo_drop_count"`
	JobLevels      map[string]int   `json:"job_levels"`
	KnownSpells    []string         `json:"known_spells"`
	KnownAbilities []string         `json:"known_abilities"`
	Zone           int              `json:"zone"` // Zone ID from client - TODO: convert to zone name
}

// UnmarshalStatusUpdate deserializes a StatusUpdate from JSON
func UnmarshalStatusUpdate(data []byte) (*StatusUpdate, error) {
	var rawStatus statusUpdateRaw
	err := json.Unmarshal(data, &rawStatus)
	if err != nil {
		return nil, err
	}

	// Convert raw status to proper StatusUpdate with job name conversion
	status := &StatusUpdate{
		Timestamp:      rawStatus.Timestamp,
		PlayerMP:       rawStatus.PlayerMP,
		PlayerHP:       rawStatus.PlayerHP,
		PlayerStatus:   rawStatus.PlayerStatus,
		EchoDropCount:  rawStatus.EchoDropCount,
		JobLevels:      rawStatus.JobLevels,
		KnownSpells:    rawStatus.KnownSpells,
		KnownAbilities: rawStatus.KnownAbilities,
		Zone:           fmt.Sprintf("Zone_%d", rawStatus.Zone), // TODO: convert zone ID to zone name
	}

	// Convert party members with job ID to job name conversion
	for _, rawMember := range rawStatus.PartyMembers {
		member := PartyMember{
			Name:          rawMember.Name,
			HPPercent:     rawMember.HPPercent,
			MPPercent:     rawMember.MPPercent,
			HPActual:      rawMember.HPActual,
			HPMax:         rawMember.HPMax, // Max HP from Ashita v4
			MPActual:      rawMember.MPActual,
			MPMax:         rawMember.MPMax, // Max MP from Ashita v4
			StatusEffects: rawMember.StatusEffects,
			Job:           getJobName(rawMember.Job),
			Distance:      rawMember.Distance,
			LastUpdate:    rawMember.LastUpdate,
		}
		status.PartyMembers = append(status.PartyMembers, member)
	}

	return status, nil
}

// getJobName converts a job ID to job name using the job package
func getJobName(jobID int) string {
	return job.GetJobName(jobID)
}
