package casting

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"PandaBot/internal/action"
	"PandaBot/internal/entity"
	"PandaBot/internal/protocol"
	"PandaBot/internal/statusMonitor"
)

// ServerClientAdapter adapts the existing server client to work with the casting engine
type ServerClientAdapter struct {
	conn       net.Conn
	playerName string
	lastSeen   time.Time
	mu         sync.RWMutex

	// Client state
	mp             int
	jobLevels      map[string]int
	knownSpells    map[string]bool
	knownAbilities map[string]bool
	isConnected    bool

	// Command tracking
	pendingCommands map[string]*ActionCommand
	commandMu       sync.RWMutex

	// Ready check tracking
	readyChecks   map[string]chan *protocol.ReadyResponse
	readyChecksMu sync.RWMutex

	// Ready for action tracking
	readyForActionChan chan struct{}

	// Status Monitor reference
	statusMonitor *statusMonitor.StatusMonitor

	// executionMu ensures only one command is being processed (checked or sent) at a time
	executionMu sync.Mutex
}

// NewServerClientAdapter creates a new adapter for an existing server client
func NewServerClientAdapter(conn net.Conn, playerName string) *ServerClientAdapter {
	return &ServerClientAdapter{
		conn:               conn,
		playerName:         playerName,
		lastSeen:           time.Now(),
		jobLevels:          make(map[string]int),
		knownSpells:        make(map[string]bool),
		knownAbilities:     make(map[string]bool),
		isConnected:        true,
		pendingCommands:    make(map[string]*ActionCommand),
		readyChecks:        make(map[string]chan *protocol.ReadyResponse),
		readyForActionChan: make(chan struct{}, 10), // Buffered to prevent blocking client loop
	}
}

// SendActionCommand implements ClientInterface
func (sca *ServerClientAdapter) SendActionCommand(command *ActionCommand) error {
	sca.commandMu.Lock()
	sca.pendingCommands[command.ID] = command
	sca.commandMu.Unlock()

	// Determine the correct command prefix based on action type
	commandPrefix := "/ma" // Default to magic
	switch command.ActionType {
	case action.ActionTypeAbility:
		commandPrefix = "/ja"
	case action.ActionTypeItem:
		commandPrefix = "/item"
	case action.ActionTypeSpell:
		commandPrefix = "/ma"
	}

	// Create execute command message using existing protocol
	// Ashita v4 expects a standard command string
	commandStr := fmt.Sprintf("%s \"%s\" %s", commandPrefix, command.ActionName, command.Target)

	commandMsg := &protocol.Message{
		Type: protocol.TypeExecuteCommand,
		Body: map[string]interface{}{
			"id":       command.ID,
			"command":  commandStr,
			"target":   command.Target,
			"priority": command.Priority,
			"timeout":  int(command.Timeout.Milliseconds()),
		},
	}

	msgBytes, err := json.Marshal(commandMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal command message: %v", err)
	}

	return sca.sendMessage(string(msgBytes))
}

// GetClientInfo implements ClientInterface
func (sca *ServerClientAdapter) GetClientInfo() *ClientInfo {
	sca.mu.RLock()
	defer sca.mu.RUnlock()

	// Copy job levels to avoid race conditions
	jobLevelsCopy := make(map[string]int)
	for job, level := range sca.jobLevels {
		jobLevelsCopy[job] = level
	}

	// Copy known spells/abilities to avoid race conditions
	knownSpellsCopy := make(map[string]bool)
	for s, v := range sca.knownSpells {
		knownSpellsCopy[s] = v
	}
	knownAbilitiesCopy := make(map[string]bool)
	for a, v := range sca.knownAbilities {
		knownAbilitiesCopy[a] = v
	}

	return &ClientInfo{
		PlayerName:     sca.playerName,
		MP:             sca.mp,
		JobLevels:      jobLevelsCopy,
		KnownSpells:    knownSpellsCopy,
		KnownAbilities: knownAbilitiesCopy,
		IsConnected:    sca.isConnected,
		LastSeen:       sca.lastSeen,
	}
}

// IsConnected implements ClientInterface
func (sca *ServerClientAdapter) GetStatusMonitor() *statusMonitor.StatusMonitor {
	return sca.statusMonitor
}

func (sca *ServerClientAdapter) IsConnected() bool {
	sca.mu.RLock()
	defer sca.mu.RUnlock()
	return sca.isConnected
}

// CheckReadyToCast implements ClientInterface
func (sca *ServerClientAdapter) CheckReadyToCast(commandID string) (bool, string, error) {
	// Deprecated: No longer polling client
	return true, "", nil
}

// WaitForReadyForAction implements ClientInterface
func (sca *ServerClientAdapter) WaitForReadyForAction(timeout time.Duration) error {
	select {
	case <-sca.readyForActionChan:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for ready for action signal")
	}
}

// LockExecution implements ClientInterface
func (sca *ServerClientAdapter) LockExecution() {
	sca.executionMu.Lock()
}

// UnlockExecution implements ClientInterface
func (sca *ServerClientAdapter) UnlockExecution() {
	sca.executionMu.Unlock()
}

// UpdateClientState updates the client's state information including known actions
func (sca *ServerClientAdapter) UpdateClientState(mp int, jobLevels map[string]int, knownSpells []string, knownAbilities []string) {
	sca.mu.Lock()
	defer sca.mu.Unlock()

	sca.mp = mp
	sca.jobLevels = make(map[string]int)
	for job, level := range jobLevels {
		sca.jobLevels[job] = level
	}

	// Update known spells
	sca.knownSpells = make(map[string]bool)
	for _, s := range knownSpells {
		sca.knownSpells[s] = true
	}

	// Update known abilities
	sca.knownAbilities = make(map[string]bool)
	for _, a := range knownAbilities {
		sca.knownAbilities[a] = true
	}

	sca.lastSeen = time.Now()
}

// SetConnected updates the connection status
func (sca *ServerClientAdapter) SetConnected(connected bool) {
	sca.mu.Lock()
	defer sca.mu.Unlock()
	sca.isConnected = connected
}

// SetPlayerName updates the player name
func (sca *ServerClientAdapter) SetPlayerName(playerName string) {
	sca.mu.Lock()
	defer sca.mu.Unlock()
	sca.playerName = playerName
}

// HandleSpellComplete handles spell completion notifications
func (sca *ServerClientAdapter) HandleSpellComplete(commandID string) {
	sca.commandMu.Lock()
	defer sca.commandMu.Unlock()

	if command, exists := sca.pendingCommands[commandID]; exists {
		delete(sca.pendingCommands, commandID)
		log.Printf("Action completed successfully: %s -> %s on %s",
			commandID, command.ActionName, command.Target)
	}
}

// HandleSpellFailed handles spell failure notifications
func (sca *ServerClientAdapter) HandleSpellFailed(commandID string, errorMsg string) {
	sca.commandMu.Lock()
	defer sca.commandMu.Unlock()

	if command, exists := sca.pendingCommands[commandID]; exists {
		delete(sca.pendingCommands, commandID)
		log.Printf("Action failed: %s -> %s on %s (error: %s)",
			commandID, command.ActionName, command.Target, errorMsg)
	}
}

// HandleReadyResponse handles ready check responses from the client
func (sca *ServerClientAdapter) HandleReadyResponse(resp *protocol.ReadyResponse) {
	sca.readyChecksMu.RLock()
	ch, exists := sca.readyChecks[resp.CommandID]
	sca.readyChecksMu.RUnlock()

	if exists {
		select {
		case ch <- resp:
		default:
			// Channel full, ignore
		}
	}
}

// GetPendingCommands returns currently pending commands
func (sca *ServerClientAdapter) GetPendingCommands() map[string]*ActionCommand {
	sca.commandMu.RLock()
	defer sca.commandMu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]*ActionCommand)
	for id, cmd := range sca.pendingCommands {
		result[id] = cmd
	}
	return result
}

// sendMessage sends a message to the client using length-prefixed protocol
func (sca *ServerClientAdapter) sendMessage(message string) error {
	// Proactively check if we are still connected
	if !sca.IsConnected() {
		return fmt.Errorf("cannot send message: client is disconnected")
	}

	// Create length prefix (4 bytes, big-endian)
	messageBytes := []byte(message)
	messageLength := uint32(len(messageBytes))

	lengthPrefix := []byte{
		byte(messageLength >> 24),
		byte(messageLength >> 16),
		byte(messageLength >> 8),
		byte(messageLength),
	}

	// Set a write deadline to avoid hanging on a half-closed connection
	sca.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	defer sca.conn.SetWriteDeadline(time.Time{})

	// Send length prefix + message
	_, err := sca.conn.Write(lengthPrefix)
	if err != nil {
		sca.SetConnected(false)
		return fmt.Errorf("error sending length prefix: %v (marking client as disconnected)", err)
	}

	_, err = sca.conn.Write(messageBytes)
	if err != nil {
		sca.SetConnected(false)
		return fmt.Errorf("error sending message: %v (marking client as disconnected)", err)
	}

	return nil
}

// CastingServerIntegration integrates the casting engine with the existing server
type CastingServerIntegration struct {
	engine           *CastingEngine
	clientManager    *ClientManager
	helper           *CastingHelper
	triggerProcessor *TriggerProcessor

	// Adapters for existing server clients
	clientAdapters map[net.Conn]*ServerClientAdapter
	adaptersMu     sync.RWMutex
}

// NewCastingServerIntegration creates a new server integration
func NewCastingServerIntegration() *CastingServerIntegration {
	engine := NewCastingEngine(DefaultCastingConfig())
	clientManager := NewClientManager(engine)
	helper := NewCastingHelper(engine, clientManager)
	triggerProcessor := NewTriggerProcessor(engine)

	// Set up the engine to use the client manager for execution
	engine.SetClientManager(clientManager)

	return &CastingServerIntegration{
		engine:           engine,
		clientManager:    clientManager,
		helper:           helper,
		triggerProcessor: triggerProcessor,
		clientAdapters:   make(map[net.Conn]*ServerClientAdapter),
	}
}

// RegisterClient registers a new client connection with the casting system
func (csi *CastingServerIntegration) RegisterClient(conn net.Conn, playerName string, sm *statusMonitor.StatusMonitor) {
	csi.adaptersMu.Lock()
	defer csi.adaptersMu.Unlock()

	adapter := NewServerClientAdapter(conn, playerName)
	adapter.statusMonitor = sm
	csi.clientAdapters[conn] = adapter

	// Use a stable ID that doesn't change even if playerName is updated
	clientID := fmt.Sprintf("client_%p", conn)
	csi.clientManager.RegisterClient(clientID, adapter)

	log.Printf("Registered client %s (ID: %s) with casting system", playerName, clientID)
}

// UnregisterClient removes a client from the casting system
func (csi *CastingServerIntegration) UnregisterClient(conn net.Conn) {
	csi.adaptersMu.Lock()
	defer csi.adaptersMu.Unlock()

	if adapter, exists := csi.clientAdapters[conn]; exists {
		clientID := fmt.Sprintf("client_%p", conn)
		csi.clientManager.UnregisterClient(clientID)
		delete(csi.clientAdapters, conn)

		log.Printf("Unregistered client %s (ID: %s) from casting system", adapter.playerName, clientID)
	}
}

// UpdateClientStatus updates client status information
// UpdateClientStatus updates the client's status in the casting system
func (csi *CastingServerIntegration) UpdateClientStatus(conn net.Conn, mp int, jobLevels map[string]int, knownSpells []string, knownAbilities []string) {
	csi.adaptersMu.RLock()
	defer csi.adaptersMu.RUnlock()

	if adapter, exists := csi.clientAdapters[conn]; exists {
		adapter.UpdateClientState(mp, jobLevels, knownSpells, knownAbilities)
	}
}

// UpdateClientPlayerName updates the client's player name
func (csi *CastingServerIntegration) UpdateClientPlayerName(conn net.Conn, playerName string) {
	csi.adaptersMu.RLock()
	defer csi.adaptersMu.RUnlock()

	if adapter, exists := csi.clientAdapters[conn]; exists {
		adapter.SetPlayerName(playerName)
	}
}

// UpdateActionComplete handles action completion from existing server
func (csi *CastingServerIntegration) UpdateActionComplete(conn net.Conn, commandID string) {
	csi.adaptersMu.RLock()
	defer csi.adaptersMu.RUnlock()

	if adapter, exists := csi.clientAdapters[conn]; exists {
		adapter.HandleSpellComplete(commandID)
		csi.clientManager.NotifyActionComplete(commandID, true, "")
	}
}

// UpdateActionFailed handles action failure from existing server
func (csi *CastingServerIntegration) UpdateActionFailed(conn net.Conn, commandID string, errorMsg string) {
	csi.adaptersMu.RLock()
	defer csi.adaptersMu.RUnlock()

	if adapter, exists := csi.clientAdapters[conn]; exists {
		adapter.HandleSpellFailed(commandID, errorMsg)
		csi.clientManager.NotifyActionComplete(commandID, false, errorMsg)
	}
}

// HandleReadyResponse handles ready check response from existing server
func (csi *CastingServerIntegration) HandleReadyResponse(conn net.Conn, resp *protocol.ReadyResponse) {
	csi.adaptersMu.RLock()
	defer csi.adaptersMu.RUnlock()

	if adapter, exists := csi.clientAdapters[conn]; exists {
		adapter.HandleReadyResponse(resp)
	}
}

// HandleReadyForAction handles ready for action signal from existing server
func (csi *CastingServerIntegration) HandleReadyForAction(conn net.Conn) {
	csi.adaptersMu.RLock()
	defer csi.adaptersMu.RUnlock()

	if adapter, exists := csi.clientAdapters[conn]; exists {
		select {
		case adapter.readyForActionChan <- struct{}{}:
			// Signal sent
		default:
			// Channel full, signal already pending
		}
	}
}

// GetCastingHelper returns the casting helper for convenient operations
func (csi *CastingServerIntegration) GetCastingHelper() *CastingHelper {
	return csi.helper
}

// GetCastingEngine returns the casting engine for advanced operations
func (csi *CastingServerIntegration) GetCastingEngine() *CastingEngine {
	return csi.engine
}

// GetClientManager returns the client manager
func (csi *CastingServerIntegration) GetClientManager() *ClientManager {
	return csi.clientManager
}

// GetTriggerProcessor returns the trigger processor
func (csi *CastingServerIntegration) GetTriggerProcessor() *TriggerProcessor {
	return csi.triggerProcessor
}

// ProcessTriggerEvent processes a trigger event using the centralized casting system
func (csi *CastingServerIntegration) ProcessTriggerEvent(triggerType string, sender string, priority int, partyMembers []*entity.Entity) []string {
	// Get a connected client to determine available MP and job levels
	connectedClients := csi.clientManager.GetConnectedClients()
	if len(connectedClients) == 0 {
		log.Printf("No connected clients available for casting")
		return nil
	}

	// Use the first available client's info for spell selection
	var clientInfo *ClientInfo
	for _, client := range connectedClients {
		clientInfo = client.GetClientInfo()
		break
	}

	if clientInfo == nil {
		log.Printf("No client info available for casting")
		return nil
	}

	log.Printf("[SERVER DEBUG] ProcessTriggerEvent: triggerType=%s, clientInfo.PlayerName=%s, clientInfo.MP=%d, clientInfo.JobLevels=%v",
		triggerType, clientInfo.PlayerName, clientInfo.MP, clientInfo.JobLevels)

	// Process the trigger event through the centralized trigger processor
	requestIDs, err := csi.triggerProcessor.ProcessTriggerEvent(
		triggerType,
		sender,
		priority,
		clientInfo.PlayerName,
		clientInfo.MP,
		clientInfo.JobLevels,
		partyMembers,
	)

	if err != nil {
		log.Printf("Failed to process trigger event %s from %s: %v", triggerType, sender, err)
		return nil
	}

	log.Printf("Processed trigger event %s from %s, generated %d casting requests", triggerType, sender, len(requestIDs))
	return requestIDs
}

// GetStats returns comprehensive casting system statistics
func (csi *CastingServerIntegration) GetStats() map[string]interface{} {
	engineStats := csi.engine.GetStats()
	clientStats := csi.clientManager.GetClientStats()

	csi.adaptersMu.RLock()
	adapterCount := len(csi.clientAdapters)
	csi.adaptersMu.RUnlock()

	return map[string]interface{}{
		"engine":   engineStats,
		"clients":  clientStats,
		"adapters": adapterCount,
	}
}
