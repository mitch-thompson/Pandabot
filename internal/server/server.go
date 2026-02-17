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
	"PandaBot/internal/config"
	"PandaBot/internal/cureSelector"
	"PandaBot/internal/job"
	"PandaBot/internal/naSelector"
	"PandaBot/internal/player"
	"PandaBot/internal/prioritizer"
	"PandaBot/internal/protocol"
	"PandaBot/internal/registry"
	"PandaBot/internal/spell"
	"PandaBot/internal/statusMonitor"
	"PandaBot/internal/textParser"
	"PandaBot/internal/triggerService"
	"PandaBot/internal/zone"
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
	Player        *player.Player

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
	conn         net.Conn
	reader       *bufio.Reader
	writer       *bufio.Writer
	lastSeen     time.Time
	playerName   string // Name of the player running this client
	currentZone  string // Current zone ID for the client
	DisableCures bool   // If true, cures and na spells are disabled
	PLSource     string // Name of the player who sent "power level" for this character
	PLTarget     string // Name of the player who received "power level" for this character

	// Command queue management
	commandQueue   []*QueuedCommand
	currentCommand *QueuedCommand
	queueMutex     sync.RWMutex
	statusMonitor  *statusMonitor.StatusMonitor
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
	p := &player.Player{
		SpellRecast:   make(map[uint16]time.Time),
		AbilityRecast: make(map[uint16]time.Time),
	}
	castingSystem.GetCastingEngine().Player = p

	aas := autoActionService.NewAutoActionService(castingSystem)
	s := &Server{
		clients:           make(map[net.Conn]*Client),
		textParser:        textParser.NewTextParser(),
		prioritizer:       prioritizer.NewSpellPrioritizer(),
		cureSelector:      cureSelector.NewCureSelector(),
		naSelector:        naSelector.NewNaSpellSelector(),
		buffSelector:      buffSelector.NewBuffSelector(),
		statusMonitor:     statusMonitor.NewStatusMonitor(),
		Player:            p,
		castingSystem:     castingSystem,
		triggerService:    triggerService.NewTriggerService(castingSystem),
		autoActionService: aas,
		config:            config,
		stopChan:          make(chan struct{}),
	}
	return s
}

// DefaultConfig returns a default server configuration
func DefaultConfig() *Config {
	cfg := config.Get()
	return &Config{
		Port:                 cfg.Port,
		StatusUpdateInterval: 5 * time.Second,
		ClientTimeout:        30 * time.Second,
		MaxClients:           10,
		HealthThresholds: statusMonitor.HealthThresholds{
			Critical: cfg.HealthThresholds.Critical,
			Low:      cfg.HealthThresholds.Low,
			Medium:   cfg.CureThreshold,
		},
		LogLevel: "INFO",
	}
}

func (s *Server) UpdateFromConfig() {
	cfg := config.Get()
	s.statusMonitor.SetHealthThresholds(
		cfg.HealthThresholds.Critical,
		cfg.HealthThresholds.Low,
		cfg.CureThreshold,
	)

	// Ensure casting engine has player reference
	if s.castingSystem != nil && s.castingSystem.GetCastingEngine() != nil {
		s.castingSystem.GetCastingEngine().Player = s.Player
	}

	// Update auto action service with current defaults
	if s.autoActionService != nil {
	}

	// Update cure selector with current defaults
	if s.cureSelector != nil {
		s.cureSelector.SetConfig(cureSelector.Config{
			CuragaThreshold: cfg.CuragaThreshold,
			IsPowerleveling: cfg.IsPowerleveling,
		})
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
	s.UpdateFromConfig()

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
			conn:          conn,
			reader:        bufio.NewReader(conn),
			writer:        bufio.NewWriter(conn),
			lastSeen:      time.Now(),
			commandQueue:  make([]*QueuedCommand, 0),
			statusMonitor: statusMonitor.NewStatusMonitor(),
		}

		s.clientsMutex.Lock()
		s.clients[conn] = client
		s.clientsMutex.Unlock()

		log.Printf("Client connected from %s", conn.RemoteAddr())

		// Register with casting system (will be updated with player name later)
		s.castingSystem.RegisterClient(conn, "", client.statusMonitor)

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

	// Process player (parts[2])
	playerData := strings.Split(parts[2], ":")
	if len(playerData) >= 8 {
		processPartyMember(s, client, playerData, true, true) // true for isPlayer, player is always main party
	}

	// Process party members (parts[3], semicolon-separated)
	if len(parts) > 3 {
		memberStrs := strings.Split(parts[3], ";")
		for i, memberStr := range memberStrs {
			if memberStr == "" {
				continue
			}
			memberData := strings.Split(memberStr, ":")
			if len(memberData) >= 8 {
				// Based on issue description: p0-p5 (main party), p6-p17 (alliance)
				// The player (p0) is sent separately in parts[2].
				// The others (p1-p17) are sent in parts[3].
				// So index 0-4 in memberStrs are likely p1-p5.
				inMainParty := i < 5
				processPartyMember(s, client, memberData, false, inMainParty)
			}
		}
	}

	log.Printf("Status update processed from %s (timestamp: %d)", client.conn.RemoteAddr(), timestamp)
}

// Helper function to process a party member (player or other)
func processPartyMember(s *Server, client *Client, memberData []string, isPlayer bool, inMainParty bool) {
	name := memberData[0]
	hpPercent, _ := strconv.Atoi(memberData[1])
	mpPercent, _ := strconv.Atoi(memberData[2])
	hpActual, _ := strconv.Atoi(memberData[3])
	mpActual, _ := strconv.Atoi(memberData[4])
	job, _ := strconv.Atoi(memberData[5])
	zone, _ := strconv.Atoi(memberData[6])

	// Compute max values
	hpMax := 0
	if hpPercent > 0 {
		hpMax = hpActual * 100 / hpPercent
	} else {
		hpMax = hpActual // Typically 0 when dead
	}

	mpMax := 0
	if mpPercent > 0 {
		mpMax = mpActual * 100 / mpPercent
	} else {
		mpMax = mpActual
	}

	// Parse status effects
	var statusIDs []int
	if memberData[7] != "" {
		statusStrs := strings.Split(memberData[7], ",")
		for _, statusStr := range statusStrs {
			if statusID, err := strconv.Atoi(statusStr); err == nil {
				statusIDs = append(statusIDs, statusID)
			}
		}
	}

	// Update status monitor with max values
	s.statusMonitor.UpdatePartyMemberWithMaxValues(
		name,
		hpPercent,
		mpPercent,
		hpActual,
		mpActual,
		hpMax,
		mpMax,
		job,
		zone,
		statusIDs,
		inMainParty,
	)

	// If this is the player, update client info
	if isPlayer && name != "" {
		client.playerName = name
		client.currentZone = fmt.Sprintf("%d", zone)
		s.castingSystem.UpdateClientPlayerName(client.conn, name)
		// Update MP and job levels (assuming job levels from status or default)
		jobLevels := map[string]int{getJobNameFromID(job): 75} // Placeholder; update if levels available
		s.castingSystem.UpdateClientStatus(client.conn, mpActual, jobLevels, nil, nil)
	}
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
		log.Printf("Chat triggers detected from %s (msg: %s): %v", sender, message, triggerEvents)

		// Route trigger events to centralized casting system
		// Note: handleChatMessage is legacy and doesn't have a direct client context easily available
		// but we can try to find the client if needed. For now, using false and empty PL.
		s.triggerService.RouteTriggerEvents(triggerEvents, s.statusMonitor, false, "", "")
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
		go s.processCommandQueue(client, false)
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
		go s.processCommandQueue(client, false)
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
		body, ok := msg.Body.(map[string]interface{})
		if !ok {
			log.Printf("Invalid body type for status update from %s: expected map[string]interface{}", client.conn.RemoteAddr())
			return
		}
		s.handleJSONStatusUpdate(client, body)

	case protocol.TypeActionComplete:
		s.handleJSONActionComplete(client, &msg)

	case protocol.TypeActionFailed:
		s.handleJSONActionFailed(client, &msg)

	case protocol.TypeReadyResponse:
		s.handleJSONReadyResponse(client, &msg)

	case protocol.TypeReadyForAction:
		s.handleJSONReadyForAction(client, &msg)

	case protocol.TypeErrorReport:
		s.handleJSONErrorReport(client, &msg)

	default:
		log.Printf("Unknown JSON message type from %s: %d", client.conn.RemoteAddr(), msg.Type)
	}
}

// handleJSONActionComplete processes a JSON action completion notification
func (s *Server) handleJSONActionComplete(client *Client, msg *protocol.Message) {
	bodyBytes, err := json.Marshal(msg.Body)
	if err != nil {
		log.Printf("Failed to marshal action complete body: %v", err)
		return
	}

	var complete protocol.ActionComplete
	if err := json.Unmarshal(bodyBytes, &complete); err != nil {
		log.Printf("Failed to unmarshal action complete: %v", err)
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
	s.castingSystem.UpdateActionComplete(client.conn, complete.CommandID)

	// If it was a buff, we should check if we can clear a desired buff
	client.queueMutex.Lock()
	defer client.queueMutex.Unlock()

	if client.currentCommand != nil && client.currentCommand.ID == complete.CommandID {
		// If it's a spell command, try to clear the corresponding desired buff
		cmd := client.currentCommand.Command
		if strings.HasPrefix(cmd, "/ma \"") {
			// Extract spell name
			endIndex := strings.Index(cmd[5:], "\"")
			if endIndex != -1 {
				spellName := cmd[5 : 5+endIndex]
				log.Printf("Spell %s completed, clearing desired buff if present", spellName)
				s.statusMonitor.ClearDesiredBuffBySpell(spellName)
			}
		}

		client.currentCommand.State = CommandCompleted
		now := time.Now()
		client.currentCommand.CompletedAt = &now

		log.Printf("Command %s completed successfully: %s", complete.CommandID, client.currentCommand.Command)

		// Clear current command and process next in queue
		client.currentCommand = nil
		go s.processCommandQueue(client, false)
	} else {
		log.Printf("Received completion for unknown command %s from %s", complete.CommandID, client.conn.RemoteAddr())
	}
}

// handleJSONActionFailed processes a JSON action failure notification
func (s *Server) handleJSONActionFailed(client *Client, msg *protocol.Message) {
	bodyBytes, err := json.Marshal(msg.Body)
	if err != nil {
		log.Printf("Failed to marshal action failed body: %v", err)
		return
	}

	var failed protocol.ActionFailed
	if err := json.Unmarshal(bodyBytes, &failed); err != nil {
		log.Printf("Failed to unmarshal action failed: %v", err)
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
	s.castingSystem.UpdateActionFailed(client.conn, failed.CommandID, failed.Error)

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
		go s.processCommandQueue(client, false)
	} else {
		log.Printf("Received failure for unknown command %s from %s", failed.CommandID, client.conn.RemoteAddr())
	}
}

// handleJSONReadyForAction processes a signal that the client is ready for a new action
func (s *Server) handleJSONReadyForAction(client *Client, msg *protocol.Message) {
	// Use mutex for thread-safe access to currentCommand
	client.queueMutex.Lock()
	if client.currentCommand != nil {
		client.queueMutex.Unlock()
		// log.Printf("Client %s is already executing a command, skipping decision tree", client.playerName)
		return
	}
	client.queueMutex.Unlock()

	// First notify the casting system (for potential pending manual requests)
	s.castingSystem.HandleReadyForAction(client.conn)

	// Now check the decision tree for the next automatic action
	playerName := client.playerName
	if playerName == "" {
		// Try to find player name from status monitor if not set on client
		for name, pm := range s.statusMonitor.GetAllPartyMembers() {
			// Check if this member's zone matches the client's current zone
			// client.currentZone is a string, pm.Zone is an int
			zoneStr := fmt.Sprintf("%d", pm.Zone)
			if (client.currentZone == zoneStr || client.currentZone == "Zone_"+zoneStr) && pm.HPMax > 0 {
				playerName = name
				client.playerName = name // Cache it
				break
			}
		}
	}

	if playerName != "" {
		command, reason, err := s.autoActionService.DecideNextAction(playerName, s.statusMonitor, client.DisableCures, client.PLSource, client.PLTarget)
		if err != nil {
			log.Printf("Decision tree error for %s: %v", playerName, err)
		} else if command != nil {
			log.Printf("Decision tree for %s: %s (Reason: %s)", playerName, command.Command, reason)

			// Wrap in JSON ExecuteCommand message
			commandID := fmt.Sprintf("auto_%d", time.Now().UnixNano())
			executeMsg := protocol.Message{
				Type: protocol.TypeExecuteCommand,
				Body: protocol.ExecuteCommand{
					Command:   command.Command,
					Target:    command.Target,
					Priority:  command.Priority,
					ID:        commandID,
					Timestamp: time.Now().Unix(),
				},
			}

			// Use a custom encoder to avoid HTML escaping of < and >
			var buf strings.Builder
			encoder := json.NewEncoder(&buf)
			encoder.SetEscapeHTML(false)
			err = encoder.Encode(executeMsg)

			if err != nil {
				log.Printf("Failed to marshal auto command: %v", err)
			} else {
				// Record this as the current command so we don't spam and can track completion
				client.queueMutex.Lock()
				now := time.Now()
				client.currentCommand = &QueuedCommand{
					ID:       commandID,
					Command:  command.Command,
					Target:   command.Target,
					Priority: command.Priority,
					State:    CommandInProgress,
					QueuedAt: now,
					SentAt:   &now,
				}
				client.queueMutex.Unlock()

				s.sendMessageToClient(client, strings.TrimSpace(buf.String()))
				return
			}
		}
	}

	// If decision tree has nothing, fallback to old queue for now
	// but the goal is to deprecate it.
	s.processCommandQueue(client, false)
}

// handleJSONReadyResponse processes a JSON ready response notification
func (s *Server) handleJSONReadyResponse(client *Client, msg *protocol.Message) {
	bodyBytes, err := json.Marshal(msg.Body)
	if err != nil {
		log.Printf("Failed to marshal ready response body: %v", err)
		return
	}

	var resp protocol.ReadyResponse
	if err := json.Unmarshal(bodyBytes, &resp); err != nil {
		log.Printf("Failed to unmarshal ready response: %v", err)
		return
	}

	// Notify centralized casting system
	s.castingSystem.HandleReadyResponse(client.conn, &resp)
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
		log.Printf("Chat triggers detected from %s (msg: %s): %v", chat.Sender, chat.Message, triggerEvents)

		// Route trigger events to centralized casting system
		s.triggerService.RouteTriggerEvents(triggerEvents, s.statusMonitor, client.DisableCures, client.PLSource, client.PLTarget)
	}

	// Power Leveling Mode detection
	// mode 13/14 are typically incoming tells
	// mode 12 is party chat
	// some versions of Ashita/FFXI might use other modes for tells
	isAllowedMode := chat.Mode == 13 || chat.Mode == 14 || chat.Mode == 3 || chat.Mode == 4 || chat.Mode == 12

	if strings.Contains(strings.ToLower(chat.Message), "power level") {
		// Log for debugging
		log.Printf("[PL DEBUG] Power level command received. Mode: %d, Sender: %s, Message: %s", chat.Mode, chat.Sender, chat.Message)

		if isAllowedMode {
			// Only enable if both are connected (implied since we received the tell/message from sender and we are here)
			// Use the effective sender from textParser if available
			effectiveSender := chat.Sender

			// If textParser extracted a sender (e.g. from a formatted string), use that
			if len(triggerEvents) > 0 {
				effectiveSender = triggerEvents[0].Sender
			}

			client.PLSource = effectiveSender
			client.PLTarget = client.playerName
			log.Printf("POWER LEVELING MODE ENABLED for %s: %s is power leveling %s", client.playerName, client.PLTarget, client.PLSource)
		} else {
			log.Printf("[PL DEBUG] Power level command ignored because mode %d is not a tell or party message", chat.Mode)
		}
	}

	if strings.Contains(strings.ToLower(chat.Message), "stop pl") {
		// Try to match sender against PL source/target using case-insensitive comparison
		// If textParser extracted a sender, we should check that too
		matchFound := false
		if strings.EqualFold(chat.Sender, client.PLSource) || strings.EqualFold(chat.Sender, client.PLTarget) {
			matchFound = true
		} else if len(triggerEvents) > 0 {
			effectiveSender := triggerEvents[0].Sender
			if strings.EqualFold(effectiveSender, client.PLSource) || strings.EqualFold(effectiveSender, client.PLTarget) {
				matchFound = true
			}
		}

		if matchFound {
			client.PLSource = ""
			client.PLTarget = ""
			log.Printf("POWER LEVELING MODE DISABLED for %s by %s", client.playerName, chat.Sender)
		}
	}

	if strings.Contains(strings.ToLower(chat.Message), "disable cures") {
		client.DisableCures = true
		log.Printf("CURES DISABLED for %s by %s", client.playerName, chat.Sender)
	}

	if strings.Contains(strings.ToLower(chat.Message), "enable cures") {
		client.DisableCures = false
		log.Printf("CURES ENABLED for %s by %s", client.playerName, chat.Sender)
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
		if s.isCommandStillNecessary(client, cmd) {
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
func (s *Server) isCommandStillNecessary(client *Client, cmd *QueuedCommand) bool {
	// Self-recovery items are always necessary until used or silence wears off
	if cmd.Priority >= 100 {
		return true
	}

	// Check if in restricted zone
	if zone.IsRestricted(client.currentZone) {
		log.Printf("[ZONE] Casting restricted in %s, removing command %s", client.currentZone, cmd.Command)
		return false
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

	// Check if the command is for a buff the target already has
	lowerCmd := strings.ToLower(cmd.Command)

	// Use centralized buff to status mapping
	buffChecks := statusMonitor.GetBuffToStatusMap()

	for substr, statusID := range buffChecks {
		if strings.Contains(lowerCmd, substr) {
			// Check if target already has this status effect
			for _, currentStatusID := range member.StatusIDs {
				if currentStatusID == statusID {
					log.Printf("Queue GC: Target %s already has buff %s (status %d), removing command", targetName, substr, statusID)
					return false
				}
			}
			break
		}
	}

	return true
}

// handleJSONStatusUpdate processes a JSON status update from the client
func (s *Server) handleJSONStatusUpdate(client *Client, body map[string]interface{}) {
	// log.Printf("[DEBUG] Received JSON status update")
	/*
		bodyJSON, _ := json.Marshal(body)
		log.Printf("[DEBUG] JSON Status Body: %s", string(bodyJSON))
	*/
	// Parse player name and zone
	playerName, ok := body["PlayerName"].(string)
	if !ok {
		log.Printf("Invalid player name in status update from %s", client.conn.RemoteAddr())
		return
	}
	zoneFloat, ok := body["Zone"].(float64)
	if !ok {
		log.Printf("Invalid zone in status update from %s", client.conn.RemoteAddr())
		return
	}
	zone := int(zoneFloat)

	// Update client info
	client.playerName = playerName
	client.currentZone = fmt.Sprintf("Zone_%d", zone)
	s.castingSystem.UpdateClientPlayerName(client.conn, playerName)

	// Parse members
	members, ok := body["Members"].([]interface{})
	if !ok {
		log.Printf("Invalid members array in status update from %s", client.conn.RemoteAddr())
		return
	}

	for _, m := range members {
		member, ok := m.(map[string]interface{})
		if !ok {
			continue
		}

		name, ok := member["Name"].(string)
		if !ok {
			continue
		}

		hpPercentFloat, _ := member["HPPercent"].(float64)
		mpPercentFloat, _ := member["MPPercent"].(float64)
		hpActualFloat, _ := member["HPActual"].(float64)
		mpActualFloat, _ := member["MPActual"].(float64)
		jobFloat, _ := member["Job"].(float64)
		jobLevelFloat, _ := member["JobLevel"].(float64)
		memberZoneFloat, _ := member["Zone"].(float64)

		hpPercent := int(hpPercentFloat)
		mpPercent := int(mpPercentFloat)
		hpActual := int(hpActualFloat)
		mpActual := int(mpActualFloat)
		job := int(jobFloat)
		jobLevel := int(jobLevelFloat)
		memberZone := int(memberZoneFloat)

		// Calculate max values
		hpMax := 0
		if hpPercent > 0 {
			hpMax = hpActual * 100 / hpPercent
		} else {
			hpMax = hpActual
		}
		mpMax := 0
		if mpPercent > 0 {
			mpMax = mpActual * 100 / mpPercent
		} else {
			mpMax = mpActual
		}

		// Parse status effects
		var statusIDs []int
		effects, ok := member["StatusEffects"].([]interface{})
		if ok {
			for _, eff := range effects {
				effFloat, ok := eff.(float64)
				if ok {
					statusIDs = append(statusIDs, int(effFloat))
				}
			}
		}

		// Update status monitor
		s.statusMonitor.UpdatePartyMemberWithMaxValues(
			name,
			hpPercent,
			mpPercent,
			hpActual,
			mpActual,
			hpMax,
			mpMax,
			job,
			memberZone,
			statusIDs,
			true, // Currently assume true for JSON updates
		)

		// Update local per-client status monitor
		if client.statusMonitor != nil {
			client.statusMonitor.UpdatePartyMemberWithMaxValues(
				name,
				hpPercent,
				mpPercent,
				hpActual,
				mpActual,
				hpMax,
				mpMax,
				job,
				memberZone,
				statusIDs,
				true,
			)
		}

		// If this is the player, update additional info
		if name == playerName {
			// Update player object available spells
			if spellsData, ok := body["KnownSpells"].([]interface{}); ok {
				if s.Player.AvailableSpells == nil {
					s.Player.AvailableSpells = make(map[string]*spell.Spell)
				}
				for _, sp := range spellsData {
					if spellName, ok := sp.(string); ok {
						if spellObj, err := registry.GetSpell(spellName); err == nil {
							s.Player.AvailableSpells[spellName] = spellObj
						}
					}
				}
			}

			// Update MP and job levels
			jobName := getJobNameFromID(job)
			jobLevels := map[string]int{jobName: jobLevel}
			if jobLevel <= 0 {
				jobLevels[jobName] = 75 // Fallback if level not provided
			}

			// Add subjob if present
			if subJob, ok := body["SubJob"].(float64); ok && subJob > 0 {
				subJobName := getJobNameFromID(int(subJob))
				subJobLevel := 0
				if sjl, ok := body["SubJobLevel"].(float64); ok {
					subJobLevel = int(sjl)
				}
				if subJobLevel > 0 {
					jobLevels[subJobName] = subJobLevel
				}
			}

			// Extract known spells and abilities if present
			var knownSpells []string
			if spells, ok := body["KnownSpells"].([]interface{}); ok {
				for _, s := range spells {
					if name, ok := s.(string); ok {
						knownSpells = append(knownSpells, name)
					}
				}
			}

			// Handle SpellRecasts if present
			if recasts, ok := body["SpellRecasts"].(map[string]interface{}); ok {
				for idStr, remainingStr := range recasts {
					id, err := strconv.ParseUint(idStr, 10, 16)
					if err != nil {
						continue
					}
					// Lua might send this as a string or a number depending on how it's encoded
					var remaining float64
					switch v := remainingStr.(type) {
					case string:
						remaining, _ = strconv.ParseFloat(v, 64)
					case float64:
						remaining = v
					}

					if remaining > 0 {
						readyAt := time.Now().Add(time.Duration(remaining * float64(time.Second)))
						s.Player.SetSpellRecast(uint16(id), readyAt)
						// log.Printf("[DEBUG] Recast update for spell %d: %v remaining (ready at %v)", id, remaining, readyAt)
					} else {
						s.Player.SetSpellRecast(uint16(id), time.Time{}) // Clear it
					}
				}
			}

			// Handle AbilityRecasts if present
			if recasts, ok := body["AbilityRecasts"].(map[string]interface{}); ok {
				for idStr, remainingStr := range recasts {
					id, err := strconv.ParseUint(idStr, 10, 16)
					if err != nil {
						continue
					}
					var remaining float64
					switch v := remainingStr.(type) {
					case string:
						remaining, _ = strconv.ParseFloat(v, 64)
					case float64:
						remaining = v
					}

					if remaining > 0 {
						readyAt := time.Now().Add(time.Duration(remaining * float64(time.Second)))
						s.Player.SetAbilityRecast(uint16(id), readyAt)
					} else {
						s.Player.SetAbilityRecast(uint16(id), time.Time{})
					}
				}
			}

			var knownAbilities []string
			if abilities, ok := body["KnownAbilities"].([]interface{}); ok {
				for _, a := range abilities {
					if name, ok := a.(string); ok {
						knownAbilities = append(knownAbilities, name)
					}
				}
			}

			s.castingSystem.UpdateClientStatus(client.conn, mpActual, jobLevels, knownSpells, knownAbilities)
		}
	}
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
		go s.processCommandQueue(client, false)
	}
}

// processCommandQueue sends the next command if possible.
// fromTicker is true if called from the background ticker (used for rate limiting).
func (s *Server) processCommandQueue(client *Client, fromTicker bool) {
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
			// After timeout, we can proceed to pull next command
		} else {
			return // Still waiting for current command to complete
		}
	} else if fromTicker {
		// If called from ticker and nothing is in progress, DON'T pull a new command.
		// This implements rate limiting - we only pull when receiving completion/failure or when a new command is queued.
		return
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

// processCommandQueues periodically handles timeouts and validates queues
func (s *Server) processCommandQueues() {
	ticker := time.NewTicker(1 * time.Second) // Check every second
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.clientsMutex.RLock()
			for _, client := range s.clients {
				// We only call processCommandQueue here to handle TIMEOUTS
				// It will return early if there's a command in progress that hasn't timed out.
				// It won't pull a new command if one is already in progress.
				s.processCommandQueue(client, true)

				// Periodically validate the queue even if no status update received
				s.validateQueuedActions(client)
			}
			s.clientsMutex.RUnlock()

			// Periodically validate the centralized casting system's active casts
			s.validateCastingEngineCasts()
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

// validateCastingEngineCasts performs GC on the centralized casting engine's active casts
func (s *Server) validateCastingEngineCasts() {
	activeCasts := s.castingSystem.GetCastingEngine().GetActiveCasts()
	if len(activeCasts) == 0 {
		return
	}

	for id, activeCast := range activeCasts {
		// Only consider pending casts for removal due to redundancy
		// In-progress casts are already being cast by the client
		if activeCast.State != casting.CastStatePending {
			continue
		}

		targetName := activeCast.Request.Target
		if targetName == "" || targetName == "<me>" || targetName == "<t>" {
			continue
		}

		member, exists := s.statusMonitor.GetPartyMember(targetName)
		if !exists {
			continue
		}

		if activeCast.Request.Action == nil {
			continue
		}
		actionName := activeCast.Request.Action.GetName()

		// Map of spell names to status IDs for redundancy check
		// Use centralized buff to status mapping
		buffChecks := statusMonitor.GetBuffToStatusMap()

		// Normalize spell name (remove level suffixes like V)
		baseSpellName := actionName
		if idx := strings.LastIndex(actionName, " "); idx != -1 {
			// Check if the last part is a Roman numeral (I, II, III, IV, V)
			lastPart := actionName[idx+1:]
			isRoman := true
			for _, char := range lastPart {
				if char != 'I' && char != 'V' && char != 'X' {
					isRoman = false
					break
				}
			}
			if isRoman {
				baseSpellName = actionName[:idx]
			}
		}

		if statusID, ok := buffChecks[strings.ToLower(baseSpellName)]; ok {
			for _, currentStatusID := range member.StatusIDs {
				if currentStatusID == statusID {
					log.Printf("Casting Engine GC: Target %s already has buff %s (status %d), cancelling cast %s", targetName, actionName, statusID, id)
					s.castingSystem.GetCastingEngine().CancelCast(id)
					break
				}
			}
		}
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

func (s *Server) GetStatusMonitor() *statusMonitor.StatusMonitor {
	return s.statusMonitor
}
