package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"PandaBot/internal/autoActionService"
	"PandaBot/internal/buffSelector"
	"PandaBot/internal/casting"
	"PandaBot/internal/cureSelector"
	"PandaBot/internal/job"
	"PandaBot/internal/naSelector"
	"PandaBot/internal/prioritizer"
	"PandaBot/internal/protocol"
	"PandaBot/internal/statusMonitor"
	"PandaBot/internal/textParser"
	"PandaBot/internal/triggerService"
)

// Server represents the main PandaBot server
type Server struct {
	listener     net.Listener
	clients      map[net.Conn]*Client
	clientsMutex sync.RWMutex

	// Core components
	textParser    *textParser.TextParser
	prioritizer   *prioritizer.SpellPrioritizer
	cureSelector  *cureSelector.CureSelector
	naSelector    *naSelector.NaSpellSelector
	buffSelector  *buffSelector.BuffSelector
	statusMonitor *statusMonitor.StatusMonitor

	// Centralized casting system
	castingSystem *casting.CastingServerIntegration

	// Services
	triggerService    *triggerService.TriggerService
	autoActionService *autoActionService.AutoActionService

	// Configuration
	config *Config

	// State
	running  bool
	stopChan chan struct{}
}

const MaxCommandQueueSize = 100

// CommandState represents the state of a command
type CommandState int

const (
	CommandQueued CommandState = iota
	CommandInProgress
	CommandCompleted
	CommandFailed
)

// QueuedCommand represents a command in the queue
type QueuedCommand struct {
	ID          string
	Command     string
	Target      string
	Priority    int
	Timeout     time.Duration
	State       CommandState
	QueuedAt    time.Time
	SentAt      *time.Time
	CompletedAt *time.Time
	Error       string
}

// Client represents a connected Lua addon client
type Client struct {
	conn       net.Conn
	reader     *bufio.Reader
	writer     *bufio.Writer
	lastSeen   time.Time
	playerName string // Name of the player running this client

	// Command queue management
	commandQueue   []*QueuedCommand
	currentCommand *QueuedCommand
	queueMutex     sync.RWMutex
}

// Config holds server configuration
type Config struct {
	Port                 int
	StatusUpdateInterval time.Duration
	ClientTimeout        time.Duration
	MaxClients           int
	HealthThresholds     statusMonitor.HealthThresholds
	LogLevel             string
}

// NewServer creates a new PandaBot server
func NewServer(config *Config) *Server {
	castingSystem := casting.NewCastingServerIntegration()

	return &Server{
		clients:           make(map[net.Conn]*Client),
		textParser:        textParser.NewTextParser(),
		prioritizer:       prioritizer.NewSpellPrioritizer(),
		cureSelector:      cureSelector.NewCureSelector(),
		naSelector:        naSelector.NewNaSpellSelector(),
		buffSelector:      buffSelector.NewBuffSelector(),
		statusMonitor:     statusMonitor.NewStatusMonitor(),
		castingSystem:     castingSystem,
		triggerService:    triggerService.NewTriggerService(castingSystem),
		autoActionService: autoActionService.NewAutoActionService(castingSystem),
		config:            config,
		stopChan:          make(chan struct{}),
	}
}

// DefaultConfig returns a default server configuration
func DefaultConfig() *Config {
	return &Config{
		Port:                 31337,
		StatusUpdateInterval: 5 * time.Second,
		ClientTimeout:        30 * time.Second,
		MaxClients:           10,
		HealthThresholds: statusMonitor.HealthThresholds{
			Critical: 25,
			Low:      50,
			Medium:   75,
		},
		LogLevel: "INFO",
	}
}

// Start starts the server
func (s *Server) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.config.Port))
	if err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	s.running = true
	log.Printf("PandaBot server started on port %d", s.config.Port)

	// Configure components
	s.statusMonitor.SetHealthThresholds(
		s.config.HealthThresholds.Critical,
		s.config.HealthThresholds.Low,
		s.config.HealthThresholds.Medium,
	)

	// Start background routines
	go s.acceptConnections()
	go s.monitorClients()
	go s.processStatusUpdates()
	go s.processCommandQueues()

	return nil
}

// Stop stops the server
func (s *Server) Stop() error {
	if !s.running {
		return nil
	}

	s.running = false
	close(s.stopChan)

	if s.listener != nil {
		s.listener.Close()
	}

	// Close all client connections
	s.clientsMutex.Lock()
	for conn := range s.clients {
		conn.Close()
	}
	s.clientsMutex.Unlock()

	log.Println("PandaBot server stopped")
	return nil
}

// acceptConnections handles incoming client connections
func (s *Server) acceptConnections() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				log.Printf("Error accepting connection: %v", err)
			}
			continue
		}

		// Check client limit
		s.clientsMutex.RLock()
		clientCount := len(s.clients)
		s.clientsMutex.RUnlock()

		if clientCount >= s.config.MaxClients {
			log.Printf("Max clients reached, rejecting connection from %s", conn.RemoteAddr())
			conn.Close()
			continue
		}

		client := &Client{
			conn:         conn,
			reader:       bufio.NewReader(conn),
			writer:       bufio.NewWriter(conn),
			lastSeen:     time.Now(),
			commandQueue: make([]*QueuedCommand, 0),
		}

		s.clientsMutex.Lock()
		s.clients[conn] = client
		s.clientsMutex.Unlock()

		log.Printf("Client connected from %s", conn.RemoteAddr())

		// Register with casting system (will be updated with player name later)
		s.castingSystem.RegisterClient(conn, "")

		go s.handleClient(client)
	}
}

// handleClient handles communication with a single client
func (s *Server) handleClient(client *Client) {
	defer func() {
		client.conn.Close()
		s.clientsMutex.Lock()
		delete(s.clients, client.conn)
		s.clientsMutex.Unlock()

		// Unregister from casting system
		s.castingSystem.UnregisterClient(client.conn)

		log.Printf("Client disconnected: %s", client.conn.RemoteAddr())
	}()

	for s.running {
		// Set read timeout
		client.conn.SetReadDeadline(time.Now().Add(s.config.ClientTimeout))

		// Read length prefix (4 bytes)
		lengthBuf := make([]byte, 4)
		_, err := io.ReadFull(client.conn, lengthBuf)
		if err != nil {
			if s.running {
				log.Printf("Error reading length from client %s: %v", client.conn.RemoteAddr(), err)
			}
			break
		}

		// Parse message length (big-endian uint32)
		messageLength := uint32(lengthBuf[0])<<24 | uint32(lengthBuf[1])<<16 | uint32(lengthBuf[2])<<8 | uint32(lengthBuf[3])

		// Validate message length
		if messageLength > 1024*1024 { // 1MB limit
			log.Printf("Message too large from client %s: %d bytes", client.conn.RemoteAddr(), messageLength)
			break
		}

		// Read message data
		messageBuf := make([]byte, messageLength)
		_, err = io.ReadFull(client.conn, messageBuf)
		if err != nil {
			if s.running {
				log.Printf("Error reading message from client %s: %v", client.conn.RemoteAddr(), err)
			}
			break
		}

		client.lastSeen = time.Now()
		message := string(messageBuf)

		if message == "" {
			continue
		}

		s.processClientMessage(client, message)
	}
}

// processClientMessage processes a message from a client
func (s *Server) processClientMessage(client *Client, message string) {
	// Try to parse as JSON first (new protocol)
	if strings.HasPrefix(message, "{") {
		s.processJSONMessage(client, message)
		return
	}

	// Fall back to pipe-delimited protocol (legacy)
	parts := strings.Split(message, "|")
	if len(parts) == 0 {
		return
	}

	messageType := parts[0]

	switch messageType {
	case "STATUS":
		s.handleStatusUpdate(client, parts)
	case "CHAT":
		s.handleChatMessage(client, parts)
	case "SUCCESS":
		s.handleCommandSuccess(client, parts)
	case "ERROR":
		s.handleCommandError(client, parts)
	case "SPELL_COMPLETE":
		s.handleSpellComplete(client, parts)
	case "SPELL_FAILED":
		s.handleSpellFailed(client, parts)
	case "HEARTBEAT":
		s.handleHeartbeat(client, parts)
	case "PONG":
		// Client responded to ping
		log.Printf("Received pong from %s", client.conn.RemoteAddr())
	default:
		log.Printf("Unknown message type from %s: %s", client.conn.RemoteAddr(), messageType)
	}
}

// handleStatusUpdate processes a status update from the client
func (s *Server) handleStatusUpdate(client *Client, parts []string) {
	if len(parts) < 3 {
		log.Printf("Invalid status update from %s", client.conn.RemoteAddr())
		return
	}

	// Parse timestamp
	timestamp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		log.Printf("Invalid timestamp in status update: %v", err)
		return
	}

	// Process each party member
	for i := 2; i < len(parts); i++ {
		memberData := strings.Split(parts[i], ":")
		if len(memberData) < 6 {
			continue
		}

		name := memberData[0]
		hpPercent, _ := strconv.Atoi(memberData[1])
		mpPercent, _ := strconv.Atoi(memberData[2])
		job, _ := strconv.Atoi(memberData[3])
		zone, _ := strconv.Atoi(memberData[4])

		// Parse status effects
		var statusIDs []int
		if len(memberData) > 5 && memberData[5] != "" {
			statusStrs := strings.Split(memberData[5], ",")
			for _, statusStr := range statusStrs {
				if statusID, err := strconv.Atoi(statusStr); err == nil {
					statusIDs = append(statusIDs, statusID)
				}
			}
		}

		// Update status monitor
		s.statusMonitor.UpdatePartyMember(name, hpPercent, mpPercent, job, zone, statusIDs)
	}

	log.Printf("Status update processed from %s (timestamp: %d)", client.conn.RemoteAddr(), timestamp)
}

// handleChatMessage processes a chat message from the client
func (s *Server) handleChatMessage(client *Client, parts []string) {
	if len(parts) < 4 {
		log.Printf("Invalid chat message from %s", client.conn.RemoteAddr())
		return
	}

	mode, _ := strconv.Atoi(parts[1])
	sender := parts[2]
	message := parts[3]

	// Create ChatLine for parsing
	chatLine := &protocol.ChatLine{
		Mode:      uint32(mode),
		Sender:    sender,
		Message:   message,
		Timestamp: time.Now().Unix(),
	}

	// Parse the message for trigger events (no casting logic here)
	triggerEvents, err := s.textParser.ParseMessage(chatLine)

	if err != nil {
		log.Printf("Failed to parse message from %s: %v", sender, err)
		return
	}

	if len(triggerEvents) > 0 {
		log.Printf("Chat triggers detected from %s: %v", sender, triggerEvents)

		// Route trigger events to centralized casting system
		s.triggerService.RouteTriggerEvents(triggerEvents, s.statusMonitor)
	}
}

// handleCommandSuccess processes a command success report
func (s *Server) handleCommandSuccess(client *Client, parts []string) {
	if len(parts) >= 3 {
		commandID := parts[1]
		command := parts[2]
		log.Printf("Command %s executed successfully: %s", commandID, command)
	}
}

// handleCommandError processes a command error report
func (s *Server) handleCommandError(client *Client, parts []string) {
	if len(parts) >= 3 {
		commandID := parts[1]
		error := parts[2]
		log.Printf("Command %s failed: %s", commandID, error)
	}
}

// handleHeartbeat processes a heartbeat from the client
func (s *Server) handleHeartbeat(client *Client, parts []string) {
	// Send heartbeat acknowledgment
	s.sendMessageToClient(client, "HEARTBEAT_ACK")
}

// handleSpellComplete processes a spell completion notification
func (s *Server) handleSpellComplete(client *Client, parts []string) {
	if len(parts) < 2 {
		log.Printf("Invalid spell complete message from %s", client.conn.RemoteAddr())
		return
	}

	commandID := parts[1]

	client.queueMutex.Lock()
	defer client.queueMutex.Unlock()

	if client.currentCommand != nil && client.currentCommand.ID == commandID {
		client.currentCommand.State = CommandCompleted
		now := time.Now()
		client.currentCommand.CompletedAt = &now

		log.Printf("Command %s completed successfully: %s", commandID, client.currentCommand.Command)

		// Clear current command and process next in queue
		client.currentCommand = nil
		go s.processCommandQueue(client)
	} else {
		log.Printf("Received completion for unknown command %s from %s", commandID, client.conn.RemoteAddr())
	}
}

// handleSpellFailed processes a spell failure notification
func (s *Server) handleSpellFailed(client *Client, parts []string) {
	if len(parts) < 3 {
		log.Printf("Invalid spell failed message from %s", client.conn.RemoteAddr())
		return
	}

	commandID := parts[1]
	errorMsg := parts[2]

	client.queueMutex.Lock()
	defer client.queueMutex.Unlock()

	if client.currentCommand != nil && client.currentCommand.ID == commandID {
		client.currentCommand.State = CommandFailed
		client.currentCommand.Error = errorMsg
		now := time.Now()
		client.currentCommand.CompletedAt = &now

		log.Printf("Command %s failed: %s (error: %s)", commandID, client.currentCommand.Command, errorMsg)

		// Clear current command and process next in queue
		client.currentCommand = nil
		go s.processCommandQueue(client)
	} else {
		log.Printf("Received failure for unknown command %s from %s", commandID, client.conn.RemoteAddr())
	}
}

// processJSONMessage processes a JSON message from the client
func (s *Server) processJSONMessage(client *Client, messageStr string) {
	var msg protocol.Message
	if err := json.Unmarshal([]byte(messageStr), &msg); err != nil {
		log.Printf("Failed to parse JSON message from %s: %v", client.conn.RemoteAddr(), err)
		return
	}

	switch msg.Type {
	case protocol.TypePong:
		log.Printf("Received pong from %s", client.conn.RemoteAddr())

	case protocol.TypeChatLine:
		s.handleJSONChatMessage(client, &msg)

	case protocol.TypeStatusUpdate:
		s.handleJSONStatusUpdate(client, &msg)

	case protocol.TypeSpellComplete:
		s.handleJSONSpellComplete(client, &msg)

	case protocol.TypeSpellFailed:
		s.handleJSONSpellFailed(client, &msg)

	case protocol.TypeErrorReport:
		s.handleJSONErrorReport(client, &msg)

	default:
		log.Printf("Unknown JSON message type from %s: %d", client.conn.RemoteAddr(), msg.Type)
	}
}

// handleJSONSpellComplete processes a JSON spell completion notification
func (s *Server) handleJSONSpellComplete(client *Client, msg *protocol.Message) {
	bodyBytes, err := json.Marshal(msg.Body)
	if err != nil {
		log.Printf("Failed to marshal spell complete body: %v", err)
		return
	}

	var complete protocol.SpellComplete
	if err := json.Unmarshal(bodyBytes, &complete); err != nil {
		log.Printf("Failed to unmarshal spell complete: %v", err)
		return
	}

	// Fallback for different field names if CommandID is empty
	if complete.CommandID == "" {
		var bodyMap map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &bodyMap); err == nil {
			if id, ok := bodyMap["id"].(string); ok {
				complete.CommandID = id
			} else if id, ok := bodyMap["command_id"].(string); ok {
				complete.CommandID = id
			}
		}
	}

	// Notify centralized casting system
	s.castingSystem.HandleSpellComplete(client.conn, complete.CommandID)

	client.queueMutex.Lock()
	defer client.queueMutex.Unlock()

	if client.currentCommand != nil && client.currentCommand.ID == complete.CommandID {
		client.currentCommand.State = CommandCompleted
		now := time.Now()
		client.currentCommand.CompletedAt = &now

		log.Printf("Command %s completed successfully: %s", complete.CommandID, client.currentCommand.Command)

		// Debug: Print queue state before processing next command
		s.logQueueState(client, "before processing next command after completion")

		// Clear current command and process next in queue
		client.currentCommand = nil
		go s.processCommandQueue(client)
	} else {
		log.Printf("Received completion for unknown command %s from %s", complete.CommandID, client.conn.RemoteAddr())
	}
}

// handleJSONSpellFailed processes a JSON spell failure notification
func (s *Server) handleJSONSpellFailed(client *Client, msg *protocol.Message) {
	bodyBytes, err := json.Marshal(msg.Body)
	if err != nil {
		log.Printf("Failed to marshal spell failed body: %v", err)
		return
	}

	var failed protocol.SpellFailed
	if err := json.Unmarshal(bodyBytes, &failed); err != nil {
		log.Printf("Failed to unmarshal spell failed: %v", err)
		return
	}

	// Fallback for different field names if CommandID is empty
	if failed.CommandID == "" {
		var bodyMap map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &bodyMap); err == nil {
			if id, ok := bodyMap["id"].(string); ok {
				failed.CommandID = id
			} else if id, ok := bodyMap["command_id"].(string); ok {
				failed.CommandID = id
			}
			if errMsg, ok := bodyMap["error"].(string); ok {
				failed.Error = errMsg
			}
		}
	}

	// Notify centralized casting system
	s.castingSystem.HandleSpellFailed(client.conn, failed.CommandID, failed.Error)

	client.queueMutex.Lock()
	defer client.queueMutex.Unlock()

	if client.currentCommand != nil && client.currentCommand.ID == failed.CommandID {
		client.currentCommand.State = CommandFailed
		client.currentCommand.Error = failed.Error
		now := time.Now()
		client.currentCommand.CompletedAt = &now

		log.Printf("Command %s failed: %s (error: %s)", failed.CommandID, client.currentCommand.Command, failed.Error)

		// Clear current command and process next in queue
		client.currentCommand = nil
		go s.processCommandQueue(client)
	} else {
		log.Printf("Received failure for unknown command %s from %s", failed.CommandID, client.conn.RemoteAddr())
	}
}

// handleJSONChatMessage processes a JSON chat message
func (s *Server) handleJSONChatMessage(client *Client, msg *protocol.Message) {
	bodyBytes, err := json.Marshal(msg.Body)
	if err != nil {
		log.Printf("Failed to marshal chat body: %v", err)
		return
	}

	var chat protocol.ChatLine
	if err := json.Unmarshal(bodyBytes, &chat); err != nil {
		log.Printf("Failed to unmarshal chat message: %v", err)
		return
	}

	// Parse the message for trigger events (no casting logic here)
	triggerEvents, err := s.textParser.ParseMessage(&chat)

	if err != nil {
		log.Printf("Failed to parse JSON chat message from %s: %v", chat.Sender, err)
		return
	}

	if len(triggerEvents) > 0 {
		log.Printf("Chat triggers detected from %s: %v", chat.Sender, triggerEvents)

		// Route trigger events to centralized casting system
		s.triggerService.RouteTriggerEvents(triggerEvents, s.statusMonitor)
	}
}

// validateQueuedActions performs Queue Garbage Collection (GC)
func (s *Server) validateQueuedActions(client *Client) {
	client.queueMutex.Lock()
	defer client.queueMutex.Unlock()

	if len(client.commandQueue) == 0 {
		return
	}

	validQueue := make([]*QueuedCommand, 0, len(client.commandQueue))
	for _, cmd := range client.commandQueue {
		if s.isCommandStillNecessary(cmd) {
			validQueue = append(validQueue, cmd)
		} else {
			log.Printf("Queue GC: Removing unnecessary command %s: %s for %s", cmd.ID, cmd.Command, cmd.Target)
		}
	}

	if len(validQueue) != len(client.commandQueue) {
		client.commandQueue = validQueue
		log.Printf("Queue GC: Cleaned up %d commands for client %s", len(client.commandQueue)-len(validQueue), client.conn.RemoteAddr())
	}
}

// isCommandStillNecessary checks if a queued command is still needed based on game state
func (s *Server) isCommandStillNecessary(cmd *QueuedCommand) bool {
	// Self-recovery items are always necessary until used or silence wears off
	if cmd.Priority >= 100 {
		return true
	}

	targetName := cmd.Target
	if targetName == "" || targetName == "<t>" || targetName == "<me>" {
		// If we can't resolve the target name easily, keep it to be safe
		// In a better implementation, we'd resolve <me> to the client's player name
		return true
	}

	member, exists := s.statusMonitor.GetPartyMember(targetName)
	if !exists {
		// Target no longer in party
		return false
	}

	// Check for healing commands
	if strings.Contains(strings.ToLower(cmd.Command), "cure") || strings.Contains(strings.ToLower(cmd.Command), "curaga") {
		// If member health is above 90%, most heals are unnecessary
		// This is a simple heuristic, can be refined based on heal power
		if member.HPPercent > 90 {
			return false
		}
	}

	// Check for status removal commands
	// This would require mapping command to status effect
	// For now, let's keep them unless the member is dead
	if member.HPPercent == 0 {
		// Don't try to heal or buff dead people unless it's a raise (not implemented yet)
		return false
	}

	return true
}

// handleJSONStatusUpdate processes a JSON status update
func (s *Server) handleJSONStatusUpdate(client *Client, msg *protocol.Message) {
	bodyBytes, err := json.Marshal(msg.Body)
	if err != nil {
		log.Printf("Failed to marshal status body: %v", err)
		return
	}

	status, err := protocol.UnmarshalStatusUpdate(bodyBytes)
	if err != nil {
		log.Printf("Failed to unmarshal status update: %v", err)
		return
	}

	// Log all status update information for debugging
	log.Printf("[STATUS DEBUG] Received status update from %s:", client.conn.RemoteAddr())
	log.Printf("[STATUS DEBUG]   Timestamp: %d", status.Timestamp)
	log.Printf("[STATUS DEBUG]   Player HP: %d, Player MP: %d", status.PlayerHP, status.PlayerMP)
	log.Printf("[STATUS DEBUG]   Zone: %s", status.Zone)
	log.Printf("[STATUS DEBUG]   Job Levels: %v", status.JobLevels)
	log.Printf("[STATUS DEBUG]   Party Members (%d):", len(status.PartyMembers))

	// Process each party member
	for i, member := range status.PartyMembers {
		log.Printf("[STATUS DEBUG]     [%d] %s: HP=%d%% (%d/%d), MP=%d%% (%d/%d), Job=%s, Status=%v",
			i, member.Name, member.HPPercent, member.HPActual, member.HPMax,
			member.MPPercent, member.MPActual, member.MPMax, member.Job, member.StatusEffects)

		// First party member (index 0) is always the player themselves
		if i == 0 && member.Name != "" {
			client.playerName = member.Name
			log.Printf("Updated player name for client %s: %s", client.conn.RemoteAddr(), client.playerName)

			// Update casting system with player name
			s.castingSystem.UpdateClientPlayerName(client.conn, client.playerName)

			// Update casting system with player info
			// Use actual MP from status update, not percentage
			actualMP := status.PlayerMP

			// Use job levels from status update if available, otherwise use defaults
			jobLevels := status.JobLevels
			if jobLevels == nil || len(jobLevels) == 0 {
				log.Printf("[SERVER DEBUG] No job levels provided in status update")
				jobLevels = make(map[string]int)
			}

			log.Printf("[SERVER DEBUG] Using actualMP=%d (was using MPPercent=%d)", actualMP, member.MPPercent)
			s.castingSystem.UpdateClientStatus(client.conn, actualMP, jobLevels)

			// Update player status and echo drop count in status monitor
			s.statusMonitor.UpdatePlayerStatus(client.playerName, status.PlayerStatus, status.EchoDropCount)
		}

		s.statusMonitor.UpdatePartyMemberWithMaxValues(
			member.Name,
			member.HPPercent,
			member.MPPercent,
			member.HPActual,
			member.MPActual,
			member.HPMax,
			member.MPMax,
			getJobIDFromName(member.Job), // Convert job name to ID
			0,                            // Zone not used in this context
			member.StatusEffects,
		)
	}

	log.Printf("JSON status update processed from %s (timestamp: %d)", client.conn.RemoteAddr(), status.Timestamp)

	// Trigger Queue Garbage Collection after status update
	s.validateQueuedActions(client)
}

// handleJSONErrorReport processes a JSON error report
func (s *Server) handleJSONErrorReport(client *Client, msg *protocol.Message) {
	bodyBytes, err := json.Marshal(msg.Body)
	if err != nil {
		log.Printf("Failed to marshal error report body: %v", err)
		return
	}

	var report protocol.ErrorReport
	if err := json.Unmarshal(bodyBytes, &report); err != nil {
		log.Printf("Failed to unmarshal error report: %v", err)
		return
	}

	// Fallback for different field names if CommandID is empty
	if report.CommandID == "" {
		var bodyMap map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &bodyMap); err == nil {
			if id, ok := bodyMap["id"].(string); ok {
				report.CommandID = id
			} else if id, ok := bodyMap["command_id"].(string); ok {
				report.CommandID = id
			}
		}
	}

	log.Printf("Command %s failed: %s", report.CommandID, report.Error)
}

// queueCommandForClient adds a command to the client's queue
func (s *Server) queueCommandForClient(client *Client, command string, target string, priority int) {
	client.queueMutex.Lock()
	defer client.queueMutex.Unlock()

	commandID := fmt.Sprintf("cmd_%d", time.Now().UnixNano())
	now := time.Now()
	queuedCmd := &QueuedCommand{
		ID:       commandID,
		Command:  command,
		Target:   target,
		Priority: priority,
		Timeout:  30 * time.Second, // Default timeout
		State:    CommandQueued,
		QueuedAt: now,
	}

	// Priority 100 Preemption: Place at the very front
	if priority >= 100 {
		log.Printf("Priority 100 detected, placing command %s at the front of the queue", commandID)
		client.commandQueue = append([]*QueuedCommand{queuedCmd}, client.commandQueue...)

		// If there is a current command that is NOT priority 100, we could potentially
		// mark it as interrupted, but for now we'll just let it finish and put this next.
		// Requirement 10.4 says "interrupt any current casting queue", which we interpret
		// as clearing the queue (handled by insertion) and prioritizing this action.
	} else {
		// Insert command in priority order (higher priority first)
		inserted := false
		for i, existingCmd := range client.commandQueue {
			if priority > existingCmd.Priority {
				client.commandQueue = append(client.commandQueue[:i], append([]*QueuedCommand{queuedCmd}, client.commandQueue[i:]...)...)
				inserted = true
				break
			}
		}

		if !inserted {
			client.commandQueue = append(client.commandQueue, queuedCmd)
		}
	}

	// Enforce MaxCommandQueueSize (Requirement 1.8)
	if len(client.commandQueue) > MaxCommandQueueSize {
		log.Printf("Queue for client %s exceeded limit, removing lowest priority item", client.conn.RemoteAddr())
		// The lowest priority item is always at the end due to our insertion logic
		client.commandQueue = client.commandQueue[:MaxCommandQueueSize]
	}

	log.Printf("Queued command %s for client %s (priority %d): %s", commandID, client.conn.RemoteAddr(), priority, command)

	// Debug: Print current queue state
	s.logQueueState(client, "after queueing")

	// Only try to send the next command if no command is currently in progress
	if client.currentCommand == nil {
		// Use a goroutine to avoid deadlock since we're already holding the mutex
		go s.processCommandQueue(client)
	}
}

// processCommandQueue sends the next command if possible
func (s *Server) processCommandQueue(client *Client) {
	client.queueMutex.Lock()
	defer client.queueMutex.Unlock()

	// If there's already a command in progress, wait for completion
	if client.currentCommand != nil {
		sentAt := client.currentCommand.SentAt
		if sentAt != nil && time.Since(*sentAt) > client.currentCommand.Timeout {
			log.Printf("Command %s timed out after %v, marking as failed", client.currentCommand.ID, client.currentCommand.Timeout)
			client.currentCommand.State = CommandFailed
			client.currentCommand.Error = "Command timed out"
			now := time.Now()
			client.currentCommand.CompletedAt = &now
			client.currentCommand = nil
		} else {
			return // Still waiting for current command to complete
		}
	}

	// Find next command to send
	if len(client.commandQueue) == 0 {
		return // No commands queued
	}

	// Get the highest priority command
	nextCmd := client.commandQueue[0]
	client.commandQueue = client.commandQueue[1:]

	// Send the command using JSON protocol
	commandMsg := &protocol.Message{
		Type: protocol.TypeExecuteCommand,
		Body: protocol.ExecuteCommand{
			ID:        nextCmd.ID,
			Command:   nextCmd.Command,
			Target:    nextCmd.Target,
			Priority:  nextCmd.Priority,
			Timeout:   int(nextCmd.Timeout.Milliseconds()),
			Timestamp: nextCmd.QueuedAt.Unix(),
		},
	}

	msgBytes, err := json.Marshal(commandMsg)
	if err != nil {
		log.Printf("Failed to marshal command message: %v", err)
		return
	}

	s.sendMessageToClient(client, string(msgBytes))

	// Mark as in progress
	nextCmd.State = CommandInProgress
	now := time.Now()
	nextCmd.SentAt = &now
	client.currentCommand = nextCmd

	log.Printf("Sent command %s to client %s: %s", nextCmd.ID, client.conn.RemoteAddr(), nextCmd.Command)

	// Debug: Print queue state after sending command
	s.logQueueState(client, "after sending command")
}

// sendCommandToClient is now a wrapper for backward compatibility
func (s *Server) sendCommandToClient(client *Client, command string, priority int) {
	s.queueCommandForClient(client, command, "", priority)
}

// sendMessageToClient sends a message to a specific client using length-prefixed protocol
func (s *Server) sendMessageToClient(client *Client, message string) {
	// Create length prefix (4 bytes, big-endian)
	messageBytes := []byte(message)
	messageLength := uint32(len(messageBytes))

	lengthPrefix := []byte{
		byte(messageLength >> 24),
		byte(messageLength >> 16),
		byte(messageLength >> 8),
		byte(messageLength),
	}

	// Send length prefix + message
	_, err := client.conn.Write(lengthPrefix)
	if err != nil {
		log.Printf("Error sending length prefix to client %s: %v", client.conn.RemoteAddr(), err)
		return
	}

	_, err = client.conn.Write(messageBytes)
	if err != nil {
		log.Printf("Error sending message to client %s: %v", client.conn.RemoteAddr(), err)
		return
	}
}

// broadcastMessage sends a message to all connected clients
func (s *Server) broadcastMessage(message string) {
	s.clientsMutex.RLock()
	defer s.clientsMutex.RUnlock()

	for _, client := range s.clients {
		s.sendMessageToClient(client, message)
	}
}

// monitorClients monitors client connections and removes stale ones
func (s *Server) monitorClients() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanupStaleClients()
		case <-s.stopChan:
			return
		}
	}
}

// cleanupStaleClients removes clients that haven't been seen recently
func (s *Server) cleanupStaleClients() {
	cutoff := time.Now().Add(-s.config.ClientTimeout * 2)

	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	for conn, client := range s.clients {
		if client.lastSeen.Before(cutoff) {
			log.Printf("Removing stale client: %s", conn.RemoteAddr())
			conn.Close()
			delete(s.clients, conn)
		}
	}
}

// processStatusUpdates periodically checks for actions based on status monitoring
func (s *Server) processStatusUpdates() {
	ticker := time.NewTicker(s.config.StatusUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkForAutomaticActions()
		case <-s.stopChan:
			return
		}
	}
}

// processCommandQueues periodically processes command queues and handles timeouts
func (s *Server) processCommandQueues() {
	ticker := time.NewTicker(1 * time.Second) // Check every second
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.clientsMutex.RLock()
			for _, client := range s.clients {
				// We still need s.processCommandQueue(client) to handle timeouts
				s.processCommandQueue(client)
				// Periodically validate the queue even if no status update received
				s.validateQueuedActions(client)
			}
			s.clientsMutex.RUnlock()
		case <-s.stopChan:
			return
		}
	}
}

// checkForAutomaticActions checks for actions based on current party status
func (s *Server) checkForAutomaticActions() {
	s.autoActionService.ProcessAutomaticActions(s.statusMonitor)
}

// logQueueState logs the current state of a client's command queue for debugging
func (s *Server) logQueueState(client *Client, context string) {
	currentCmd := "none"
	if client.currentCommand != nil {
		currentCmd = fmt.Sprintf("%s (priority %d, state %v)",
			client.currentCommand.Command,
			client.currentCommand.Priority,
			client.currentCommand.State)
	}

	queueInfo := make([]string, len(client.commandQueue))
	for i, cmd := range client.commandQueue {
		queueInfo[i] = fmt.Sprintf("[%d] %s (priority %d)", i, cmd.Command, cmd.Priority)
	}

	log.Printf("DEBUG Queue state %s for client %s:", context, client.conn.RemoteAddr())
	log.Printf("  Current command: %s", currentCmd)
	log.Printf("  Queue length: %d", len(client.commandQueue))
	if len(queueInfo) > 0 {
		log.Printf("  Queued commands: %v", queueInfo)
	} else {
		log.Printf("  Queued commands: (empty)")
	}
}

// GetStats returns server statistics
func (s *Server) GetStats() map[string]interface{} {
	s.clientsMutex.RLock()
	clientCount := len(s.clients)
	s.clientsMutex.RUnlock()

	// Get casting system statistics
	castingStats := s.castingSystem.GetStats()

	return map[string]interface{}{
		"running":     s.running,
		"clients":     clientCount,
		"party_count": s.statusMonitor.GetPartyCount(),
		"last_update": s.statusMonitor.GetLastUpdateTime(),
		"casting":     castingStats,
	}
}

// getJobIDFromName converts a job name to job ID for backward compatibility
func getJobIDFromName(jobName string) int {
	return job.GetJobID(jobName)
}

// getJobNameFromID converts a job ID to job name
func getJobNameFromID(jobID int) string {
	return job.GetJobName(jobID)
}

// GetCastingSystem returns the centralized casting system for external access
func (s *Server) GetCastingSystem() *casting.CastingServerIntegration {
	return s.castingSystem
}
