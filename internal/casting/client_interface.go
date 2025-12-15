package casting

import (
	"fmt"
	"time"
)

// ClientInterface defines how the casting engine communicates with game clients
type ClientInterface interface {
	// SendSpellCommand sends a spell casting command to the client
	SendSpellCommand(command *SpellCommand) error
	
	// GetClientInfo returns information about the client's current state
	GetClientInfo() *ClientInfo
	
	// IsConnected returns whether the client is currently connected
	IsConnected() bool
}

// SpellCommand represents a spell casting command to send to a client
type SpellCommand struct {
	ID       string
	Spell    string
	Target   string
	Priority int
	Timeout  time.Duration
}

// ClientInfo contains information about a client's current state
type ClientInfo struct {
	PlayerName  string
	MP          int
	JobLevels   map[string]int
	IsConnected bool
	LastSeen    time.Time
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
	// Find a suitable client to execute the cast
	client, err := cm.selectClientForCast(request)
	if err != nil {
		return err
	}
	
	// Get client info to determine caster name for target resolution
	clientInfo := client.GetClientInfo()
	
	// Update request context with caster name if not already set
	if request.Context != nil && request.Context.CasterName == "" {
		request.Context.CasterName = clientInfo.PlayerName
	}
	
	// Resolve the correct target for this spell using the casting engine
	resolvedTarget, err := cm.engine.resolveSpellTarget(request.SpellName, request.Target, request.Context)
	if err != nil {
		return fmt.Errorf("failed to resolve target: %v", err)
	}
	
	// Create spell command with resolved target
	command := &SpellCommand{
		ID:       request.ID,
		Spell:    request.SpellName,
		Target:   resolvedTarget,
		Priority: request.Priority,
		Timeout:  request.Timeout,
	}
	
	// Send command to client
	return client.SendSpellCommand(command)
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
	connectedClients := cm.GetConnectedClients()
	var errors []error
	
	for clientID, client := range connectedClients {
		command := &SpellCommand{
			ID:       fmt.Sprintf("%s_%s", request.ID, clientID),
			Spell:    request.SpellName,
			Target:   request.Target,
			Priority: request.Priority,
			Timeout:  request.Timeout,
		}
		
		if err := client.SendSpellCommand(command); err != nil {
			errors = append(errors, fmt.Errorf("client %s: %v", clientID, err))
		}
	}
	
	return errors
}

// NotifySpellComplete notifies the casting engine that a spell has completed
func (cm *ClientManager) NotifySpellComplete(commandID string, success bool, error string) {
	// Notify the casting engine to handle sequence progression and completion
	cm.engine.NotifySpellComplete(commandID, success, error)
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