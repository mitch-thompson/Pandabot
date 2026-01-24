package casting

import (
	"PandaBot/internal/action"
	"fmt"
	"time"
)

// ClientInterface defines how the casting engine communicates with game clients
type ClientInterface interface {
	// SendActionCommand sends an action command to the client
	SendActionCommand(command *ActionCommand) error

	// GetClientInfo returns information about the client's current state
	GetClientInfo() *ClientInfo

	// IsConnected returns whether the client is currently connected
	IsConnected() bool

	// CheckReadyToCast asks the client if it's ready to cast a spell
	CheckReadyToCast(commandID string) (bool, string, error)

	// WaitForReadyForAction blocks until the client signals it is ready for an action
	WaitForReadyForAction(timeout time.Duration) error

	// LockExecution locks the client for command execution (CheckReady + Send)
	LockExecution()
	// UnlockExecution unlocks the client
	UnlockExecution()
}

// ActionCommand represents an action command to send to a client
type ActionCommand struct {
	ID         string
	ActionName string
	ActionID   uint16
	ActionType action.ActionType
	Target     string
	Priority   int
	Timeout    time.Duration
}

// ClientInfo contains information about a client's current state
type ClientInfo struct {
	PlayerName     string
	MP             int
	JobLevels      map[string]int
	KnownSpells    map[string]bool
	KnownAbilities map[string]bool
	IsConnected    bool
	LastSeen       time.Time
}

// ClientManager manages multiple game clients for the casting engine
type ClientManager struct {
	clients map[string]ClientInterface
	engine  *CastingEngine
}

// NewClientManager creates a new client manager
func NewClientManager(engine *CastingEngine) *ClientManager {
	return &ClientManager{
		clients: make(map[string]ClientInterface),
		engine:  engine,
	}
}

// RegisterClient registers a new client with the casting engine
func (cm *ClientManager) RegisterClient(clientID string, client ClientInterface) {
	cm.clients[clientID] = client
}

// UnregisterClient removes a client from the casting engine
func (cm *ClientManager) UnregisterClient(clientID string) {
	delete(cm.clients, clientID)
}

// GetClient returns a client by ID
func (cm *ClientManager) GetClient(clientID string) (ClientInterface, bool) {
	client, exists := cm.clients[clientID]
	return client, exists
}

// GetConnectedClients returns all currently connected clients
func (cm *ClientManager) GetConnectedClients() map[string]ClientInterface {
	connected := make(map[string]ClientInterface)
	for id, client := range cm.clients {
		if client.IsConnected() {
			connected[id] = client
		}
	}
	return connected
}

// ExecuteCastRequest executes a cast request using available clients
func (cm *ClientManager) ExecuteCastRequest(request *CastRequest) error {
	if request.Action == nil {
		return fmt.Errorf("action is required for cast request")
	}

	// Find a suitable client to execute the cast
	client, err := cm.selectClientForCast(request)
	if err != nil {
		return err
	}

	// Wait for client to be ready for action
	timeout := 10 * time.Second
	if request.Timeout > 0 {
		timeout = request.Timeout
	}

	err = client.WaitForReadyForAction(timeout)
	if err != nil {
		return fmt.Errorf("client not ready for action within timeout: %v", err)
	}

	// Lock the client for execution to prevent multiple commands being sent at once
	client.LockExecution()
	defer client.UnlockExecution()

	// Get client info to determine caster name for target resolution
	clientInfo := client.GetClientInfo()

	// Update request context with caster name if not already set
	if request.Context != nil && request.Context.CasterName == "" {
		request.Context.CasterName = clientInfo.PlayerName
	}

	// Resolve the correct target for this action using the casting engine
	resolvedTarget, err := cm.engine.resolveActionTarget(request.Action.GetName(), request.Target, request.Context)
	if err != nil {
		return fmt.Errorf("failed to resolve target: %v", err)
	}

	// Create action command with resolved target
	command := &ActionCommand{
		ID:         request.ID,
		ActionName: request.Action.GetName(),
		ActionID:   request.Action.GetID(),
		ActionType: request.Action.GetActionType(),
		Target:     resolvedTarget,
		Priority:   request.Priority,
		Timeout:    request.Timeout,
	}

	// Send command to client
	return client.SendActionCommand(command)
}

// selectClientForCast selects the best client to execute a cast request
func (cm *ClientManager) selectClientForCast(request *CastRequest) (ClientInterface, error) {
	connectedClients := cm.GetConnectedClients()
	if len(connectedClients) == 0 {
		return nil, fmt.Errorf("no connected clients available")
	}

	// For now, select the first available client
	// In a more sophisticated implementation, this could consider:
	// - Client MP levels
	// - Job levels and spell availability
	// - Current casting load
	// - Geographic proximity to target

	for _, client := range connectedClients {
		info := client.GetClientInfo()

		// Basic checks
		if info.MP < 10 { // Minimum MP threshold
			continue
		}

		// Check if client has the required job levels for the spell
		if request.Context != nil && len(request.Context.CasterJobLevels) > 0 {
			hasRequiredJob := false
			for job, level := range info.JobLevels {
				if requiredLevel, exists := request.Context.CasterJobLevels[job]; exists {
					if level >= requiredLevel {
						hasRequiredJob = true
						break
					}
				}
			}
			if !hasRequiredJob {
				continue
			}
		}

		return client, nil
	}

	return nil, fmt.Errorf("no suitable client found for cast request")
}

// BroadcastCastRequest sends a cast request to all connected clients
func (cm *ClientManager) BroadcastCastRequest(request *CastRequest) []error {
	if request.Action == nil {
		return []error{fmt.Errorf("action is required for broadcast request")}
	}

	connectedClients := cm.GetConnectedClients()
	var errors []error

	for clientID, client := range connectedClients {
		command := &ActionCommand{
			ID:         fmt.Sprintf("%s_%s", request.ID, clientID),
			ActionName: request.Action.GetName(),
			ActionID:   request.Action.GetID(),
			ActionType: request.Action.GetActionType(),
			Target:     request.Target,
			Priority:   request.Priority,
			Timeout:    request.Timeout,
		}

		if err := client.SendActionCommand(command); err != nil {
			errors = append(errors, fmt.Errorf("client %s: %v", clientID, err))
		}
	}

	return errors
}

// NotifyActionComplete notifies the casting engine that an action has completed
func (cm *ClientManager) NotifyActionComplete(commandID string, success bool, errorMsg string) {
	// Notify the casting engine to handle sequence progression and completion
	cm.engine.NotifyActionComplete(commandID, success, errorMsg)
}

// GetClientStats returns statistics about managed clients
func (cm *ClientManager) GetClientStats() map[string]interface{} {
	totalClients := len(cm.clients)
	connectedCount := 0

	for _, client := range cm.clients {
		if client.IsConnected() {
			connectedCount++
		}
	}

	return map[string]interface{}{
		"total_clients":     totalClients,
		"connected_clients": connectedCount,
		"disconnected":      totalClients - connectedCount,
	}
}

// GetClientInfoByPlayerName returns information about a client by player name
func (cm *ClientManager) GetClientInfoByPlayerName(playerName string) *ClientInfo {
	for _, client := range cm.clients {
		info := client.GetClientInfo()
		if info.PlayerName == playerName {
			return info
		}
	}
	return nil
}

// GetFirstClientInfo returns information about the first available client
func (cm *ClientManager) GetFirstClientInfo() *ClientInfo {
	for _, client := range cm.clients {
		if client.IsConnected() {
			return client.GetClientInfo()
		}
	}
	return nil
}
