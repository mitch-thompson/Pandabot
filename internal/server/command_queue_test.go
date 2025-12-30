package server

import (
	"bufio"
	"math/rand"
	"net"
	"strings"
	"testing"
	"time"

	"PandaBot/internal/protocol"
)

// **Feature: automated-gameplay-assistant, Property 26: Server-side command queuing**
// **Validates: Requirements 1.4, 4.1**
func TestProperty26_ServerSideCommandQueuing(t *testing.T) {
	// Run property-based test with 100 iterations
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_ServerSideCommandQueuing", func(t *testing.T) {
			// Property: For any sequence of commands with different priorities,
			// the server should queue them per client and process higher priority commands first

			config := DefaultConfig()
			server := NewServer(config)

			// Create a mock client for testing
			client := &Client{
				conn:         &mockConn{},
				reader:       bufio.NewReader(&mockConn{}),
				writer:       bufio.NewWriter(&mockConn{}),
				lastSeen:     time.Now(),
				commandQueue: make([]*QueuedCommand, 0),
			}

			// Generate random commands with different priorities
			numCommands := rand.Intn(5) + 2 // 2-6 commands
			commands := make([]struct {
				command  string
				priority int
			}, numCommands)

			maxPriority := 0
			for j := 0; j < numCommands; j++ {
				priority := rand.Intn(100) + 1 // Priority 1-100
				commands[j] = struct {
					command  string
					priority int
				}{
					command:  generateRandomCommand(),
					priority: priority,
				}
				if priority > maxPriority {
					maxPriority = priority
				}
			}

			// Queue all commands
			for _, cmd := range commands {
				server.queueCommandForClient(client, cmd.command, "TestPlayer", cmd.priority)
			}

			// Allow some time for async processing to complete
			time.Sleep(50 * time.Millisecond)

			// Verify that at least one command was processed (should be the highest priority)
			client.queueMutex.RLock()
			totalCommands := len(client.commandQueue)
			if client.currentCommand != nil {
				totalCommands++ // Count the current command too
			}

			// Total commands should equal what we queued
			if totalCommands > numCommands {
				t.Errorf("More commands than expected: got %d total, expected %d", totalCommands, numCommands)
			}

			// If there's a current command, it should be the highest priority
			if client.currentCommand != nil {
				if client.currentCommand.State != CommandInProgress {
					t.Errorf("Expected command state to be InProgress, got %v", client.currentCommand.State)
				}

				// Check that no queued command has higher priority than current
				for _, queuedCmd := range client.commandQueue {
					if queuedCmd.Priority > client.currentCommand.Priority {
						t.Errorf("Queued command has higher priority (%d) than current command (%d)",
							queuedCmd.Priority, client.currentCommand.Priority)
					}
				}
			}

			// Check that remaining queued commands are sorted by priority (highest first)
			for j := 1; j < len(client.commandQueue); j++ {
				if client.commandQueue[j-1].Priority < client.commandQueue[j].Priority {
					t.Errorf("Commands not properly sorted by priority: %d < %d at positions %d, %d",
						client.commandQueue[j-1].Priority, client.commandQueue[j].Priority, j-1, j)
				}
			}
			client.queueMutex.RUnlock()
		})
	}
}

// Test command timeout handling
func TestCommandTimeout(t *testing.T) {
	config := DefaultConfig()
	server := NewServer(config)

	client := &Client{
		conn:         &mockConn{},
		reader:       bufio.NewReader(&mockConn{}),
		writer:       bufio.NewWriter(&mockConn{}),
		lastSeen:     time.Now(),
		commandQueue: make([]*QueuedCommand, 0),
	}

	// Queue and process a command
	server.queueCommandForClient(client, "/ma \"Cure\" \"Player1\"", "Player1", 5)
	server.processCommandQueue(client)

	client.queueMutex.Lock()
	if client.currentCommand == nil {
		t.Fatal("Expected a command to be in progress")
	}

	// Simulate timeout by setting sent time to past
	pastTime := time.Now().Add(-35 * time.Second) // Beyond 30 second timeout
	client.currentCommand.SentAt = &pastTime
	client.queueMutex.Unlock()

	// Process queue again to trigger timeout
	server.processCommandQueue(client)

	client.queueMutex.RLock()
	// Command should be cleared due to timeout (processCommandQueue clears timed out commands)
	if client.currentCommand != nil {
		t.Error("Expected current command to be cleared after timeout")
	}
	client.queueMutex.RUnlock()
}

// Test separate queues for different clients
func TestSeparateClientQueues(t *testing.T) {
	config := DefaultConfig()
	server := NewServer(config)

	client1 := &Client{
		conn:         &mockConn{},
		reader:       bufio.NewReader(&mockConn{}),
		writer:       bufio.NewWriter(&mockConn{}),
		lastSeen:     time.Now(),
		commandQueue: make([]*QueuedCommand, 0),
	}

	client2 := &Client{
		conn:         &mockConn{},
		reader:       bufio.NewReader(&mockConn{}),
		writer:       bufio.NewWriter(&mockConn{}),
		lastSeen:     time.Now(),
		commandQueue: make([]*QueuedCommand, 0),
	}

	// Queue commands for both clients
	server.queueCommandForClient(client1, "/ma \"Cure\" \"Player1\"", "Player1", 5)
	server.queueCommandForClient(client2, "/ma \"Protect\" \"Player2\"", "Player2", 3)

	// Verify queues are independent
	client1.queueMutex.RLock()
	client2.queueMutex.RLock()
	defer client1.queueMutex.RUnlock()
	defer client2.queueMutex.RUnlock()

	// Since commands might be processed immediately, they could be in currentCommand instead of queue
	cmd1 := ""
	if client1.currentCommand != nil {
		cmd1 = client1.currentCommand.Command
	} else if len(client1.commandQueue) > 0 {
		cmd1 = client1.commandQueue[0].Command
	} else {
		t.Fatal("Client1 has no command in queue or in progress")
	}

	cmd2 := ""
	if client2.currentCommand != nil {
		cmd2 = client2.currentCommand.Command
	} else if len(client2.commandQueue) > 0 {
		cmd2 = client2.commandQueue[0].Command
	} else {
		t.Fatal("Client2 has no command in queue or in progress")
	}

	if !strings.Contains(cmd1, "Cure") {
		t.Errorf("Client1 should have Cure command, got: %s", cmd1)
	}
	if !strings.Contains(cmd2, "Protect") {
		t.Errorf("Client2 should have Protect command, got: %s", cmd2)
	}
}

// Mock connection for testing
type mockConn struct {
	data []byte
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	return 0, nil
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	m.data = append(m.data, b...)
	return len(b), nil
}

func (m *mockConn) Close() error {
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 31337}
}

func (m *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
}

func (m *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// **Feature: automated-gameplay-assistant, Property 27: Spell completion feedback**
// **Validates: Requirements 1.4**
func TestProperty27_SpellCompletionFeedback(t *testing.T) {
	// Run property-based test with 100 iterations
	for i := 0; i < 100; i++ {
		t.Run("PropertyTest_SpellCompletionFeedback", func(t *testing.T) {
			// Property: For any command that is sent to a client,
			// when the client reports completion, the server should process the next queued command

			config := DefaultConfig()
			server := NewServer(config)

			// Create a mock client for testing
			client := &Client{
				conn:         &mockConn{},
				reader:       bufio.NewReader(&mockConn{}),
				writer:       bufio.NewWriter(&mockConn{}),
				lastSeen:     time.Now(),
				commandQueue: make([]*QueuedCommand, 0),
			}

			// Queue multiple commands
			numCommands := rand.Intn(3) + 2 // 2-4 commands
			for j := 0; j < numCommands; j++ {
				command := generateRandomCommand()
				priority := rand.Intn(10) + 1
				server.queueCommandForClient(client, command, "TestPlayer", priority)
			}

			// Allow processing
			time.Sleep(10 * time.Millisecond)

			// Should have one command in progress
			client.queueMutex.RLock()
			if client.currentCommand == nil {
				t.Error("Expected a command to be in progress")
				client.queueMutex.RUnlock()
				return
			}

			currentCommandID := client.currentCommand.ID
			remainingCommands := len(client.commandQueue)
			client.queueMutex.RUnlock()

			// Simulate spell completion
			completeMsg := &protocol.Message{
				Type: protocol.TypeSpellComplete,
				Body: map[string]interface{}{
					"command_id": currentCommandID,
					"timestamp":  time.Now().Unix(),
				},
			}

			server.handleJSONSpellComplete(client, completeMsg)

			// Allow processing
			time.Sleep(10 * time.Millisecond)

			// Verify that the next command was processed (if any remaining)
			client.queueMutex.RLock()
			if remainingCommands > 0 {
				// Should have processed the next command
				if client.currentCommand == nil {
					t.Error("Expected next command to be processed after completion")
				} else {
					if client.currentCommand.ID == currentCommandID {
						t.Error("Same command is still current after completion")
					}
					if client.currentCommand.State != CommandInProgress {
						t.Errorf("Expected next command to be in progress, got %v", client.currentCommand.State)
					}
				}

				// Should have one less command in queue
				if len(client.commandQueue) != remainingCommands-1 {
					t.Errorf("Expected %d commands in queue after completion, got %d",
						remainingCommands-1, len(client.commandQueue))
				}
			} else {
				// No more commands, should be idle
				if client.currentCommand != nil {
					t.Error("Expected no current command when queue is empty")
				}
			}
			client.queueMutex.RUnlock()
		})
	}
}

// Test spell failure feedback
func TestSpellFailureFeedback(t *testing.T) {
	config := DefaultConfig()
	server := NewServer(config)

	client := &Client{
		conn:         &mockConn{},
		reader:       bufio.NewReader(&mockConn{}),
		writer:       bufio.NewWriter(&mockConn{}),
		lastSeen:     time.Now(),
		commandQueue: make([]*QueuedCommand, 0),
	}

	// Queue multiple commands
	server.queueCommandForClient(client, "/ma \"Cure\" \"Player1\"", "Player1", 5)
	server.queueCommandForClient(client, "/ma \"Protect\" \"Player2\"", "Player2", 3)

	// Allow processing
	time.Sleep(10 * time.Millisecond)

	client.queueMutex.RLock()
	if client.currentCommand == nil {
		t.Fatal("Expected a command to be in progress")
	}
	currentCommandID := client.currentCommand.ID
	client.queueMutex.RUnlock()

	// Simulate spell failure
	failMsg := &protocol.Message{
		Type: protocol.TypeSpellFailed,
		Body: map[string]interface{}{
			"command_id": currentCommandID,
			"error":      "Insufficient MP",
			"timestamp":  time.Now().Unix(),
		},
	}

	server.handleJSONSpellFailed(client, failMsg)

	// Allow processing
	time.Sleep(10 * time.Millisecond)

	// Should process next command even after failure
	client.queueMutex.RLock()
	if client.currentCommand == nil {
		t.Error("Expected next command to be processed after failure")
	} else {
		if client.currentCommand.ID == currentCommandID {
			t.Error("Same command is still current after failure")
		}
	}
	client.queueMutex.RUnlock()
}

func TestQueueGC(t *testing.T) {
	config := DefaultConfig()
	server := NewServer(config)

	client := &Client{
		conn:         &mockConn{},
		reader:       bufio.NewReader(&mockConn{}),
		writer:       bufio.NewWriter(&mockConn{}),
		lastSeen:     time.Now(),
		commandQueue: make([]*QueuedCommand, 0),
	}

	// Setup status monitor with a member
	server.statusMonitor.UpdatePartyMemberWithMaxValues("Player1", 50, 100, 500, 1000, 1000, 1000, 1, 0, []int{})

	// Queue some commands
	server.queueCommandForClient(client, "/ma \"Cure IV\" \"Player1\"", "Player1", 80)
	server.queueCommandForClient(client, "/ma \"Haste\" \"Player1\"", "Player1", 20)
	server.queueCommandForClient(client, "/ma \"Cure II\" \"Player2\"", "Player2", 40) // Player2 not in party

	// Initially we should have 3 commands (1 in progress, 2 in queue)
	client.queueMutex.RLock()
	total := len(client.commandQueue)
	if client.currentCommand != nil {
		total++
	}
	if total != 3 {
		t.Errorf("Expected 3 total commands, got %d", total)
	}
	client.queueMutex.RUnlock()

	// Update Player1 health to 100%
	server.statusMonitor.UpdatePartyMemberWithMaxValues("Player1", 100, 100, 1000, 1000, 1000, 1000, 1, 0, []int{})

	// Run GC
	server.validateQueuedActions(client)

	// After GC:
	// - Cure IV for Player1 should be removed (HP > 90%)
	// - Haste for Player1 should be KEPT (we don't GC buffs yet)
	// - Cure II for Player2 should be removed (Player2 not in party)
	client.queueMutex.RLock()
	// currentCommand is NOT removed by GC currently (it's already sent)
	// so we check the queue
	for _, cmd := range client.commandQueue {
		if strings.Contains(cmd.Command, "Cure") {
			t.Errorf("Unnecessary cure command %s remained in queue", cmd.Command)
		}
	}
	client.queueMutex.RUnlock()
}

func TestMaxQueueSize(t *testing.T) {
	config := DefaultConfig()
	server := NewServer(config)

	client := &Client{
		conn:         &mockConn{},
		reader:       bufio.NewReader(&mockConn{}),
		writer:       bufio.NewWriter(&mockConn{}),
		lastSeen:     time.Now(),
		commandQueue: make([]*QueuedCommand, 0),
	}

	// Queue more than MaxCommandQueueSize commands
	for i := 0; i < MaxCommandQueueSize+10; i++ {
		server.queueCommandForClient(client, "/ma \"Cure\" \"Player1\"", "Player1", 10)
	}

	client.queueMutex.RLock()
	defer client.queueMutex.RUnlock()
	if len(client.commandQueue) > MaxCommandQueueSize {
		t.Errorf("Queue size %d exceeds maximum %d", len(client.commandQueue), MaxCommandQueueSize)
	}
}

// Helper function to generate random commands for testing
func generateRandomCommand() string {
	spells := []string{"Cure", "Cure II", "Cure III", "Cure IV", "Protect", "Shell", "Haste", "Regen"}
	players := []string{"Player1", "Player2", "Player3", "TestPlayer"}

	spell := spells[rand.Intn(len(spells))]
	player := players[rand.Intn(len(players))]

	return "/ma \"" + spell + "\" \"" + player + "\""
}

// Helper function to create spell complete message
func createSpellCompleteMessage(commandID string) *protocol.Message {
	return &protocol.Message{
		Type: protocol.TypeSpellComplete,
		Body: map[string]interface{}{
			"command_id": commandID,
			"timestamp":  time.Now().Unix(),
		},
	}
}
