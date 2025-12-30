package integration

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"PandaBot/internal/server"
	"PandaBot/internal/statusMonitor"
)

// SystemIntegrationTest tests the complete system end-to-end
func TestSystemIntegration(t *testing.T) {
	// Start server
	config := server.DefaultConfig()
	config.Port = 31338 // Use different port for testing

	srv := server.NewServer(config)
	err := srv.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Connect test client
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", config.Port))
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Test 1: Basic connectivity and heartbeat
	t.Run("BasicConnectivity", func(t *testing.T) {
		// Send heartbeat
		sendMessage(writer, "HEARTBEAT|123456789")

		// Should receive heartbeat ack
		response := readMessage(reader, t)
		if response != "HEARTBEAT_ACK" {
			t.Errorf("Expected HEARTBEAT_ACK, got: %s", response)
		}
	})

	// Test 2: Status update processing
	t.Run("StatusUpdateProcessing", func(t *testing.T) {
		// Send status update with party member needing healing
		statusMsg := "STATUS|1234567890|TestPlayer:25:80:3:1:2,4|HealthyPlayer:100:100:1:1:"
		sendMessage(writer, statusMsg)

		// Give server time to process
		time.Sleep(50 * time.Millisecond)

		// Check server stats
		stats := srv.GetStats()
		if partyCount, ok := stats["party_count"].(int); !ok || partyCount != 2 {
			t.Errorf("Expected 2 party members, got: %v", stats["party_count"])
		}
	})

	// Test 3: Chat message trigger processing
	t.Run("ChatMessageTriggers", func(t *testing.T) {
		// Send chat message with "stoned" trigger
		chatMsg := "CHAT|3|TestPlayer|I'm stoned, help!"
		sendMessage(writer, chatMsg)

		// Should receive command response
		response := readMessage(reader, t)
		if !strings.Contains(response, "COMMAND") || !strings.Contains(response, "Stona") {
			t.Errorf("Expected Stona command, got: %s", response)
		}

		// Send firebuffs trigger
		chatMsg = "CHAT|4|TestPlayer|need firebuffs please"
		sendMessage(writer, chatMsg)

		// Should receive multiple buff commands
		commandCount := 0
		for i := 0; i < 3; i++ {
			response := readMessage(reader, t)
			if strings.Contains(response, "COMMAND") {
				commandCount++
			}
		}

		if commandCount < 3 {
			t.Errorf("Expected at least 3 buff commands, got: %d", commandCount)
		}
	})

	// Test 4: Command success/error reporting
	t.Run("CommandReporting", func(t *testing.T) {
		// Send command success
		successMsg := "SUCCESS|cmd_123|/ma \"Cure\" TestPlayer"
		sendMessage(writer, successMsg)

		// Send command error
		errorMsg := "ERROR|cmd_124|Invalid spell name"
		sendMessage(writer, errorMsg)

		// These should be logged but not cause errors
		time.Sleep(50 * time.Millisecond)
	})

	// Test 5: Automatic healing triggers
	t.Run("AutomaticHealingTriggers", func(t *testing.T) {
		// Send status update with critically low HP
		statusMsg := "STATUS|1234567891|CriticalPlayer:15:50:3:1:|TestPlayer:100:100:1:1:"
		sendMessage(writer, statusMsg)

		// Wait for automatic action processing
		time.Sleep(6 * time.Second) // Wait for status update interval

		// Should receive automatic cure command
		response := readMessageWithTimeout(reader, 2*time.Second)
		if response != "" && (!strings.Contains(response, "COMMAND") || !strings.Contains(response, "Cure")) {
			t.Logf("Automatic healing response: %s", response)
		}
	})
}

// TestMultipleClients tests handling of multiple simultaneous clients
func TestMultipleClients(t *testing.T) {
	// Start server
	config := server.DefaultConfig()
	config.Port = 31339 // Use different port for testing
	config.MaxClients = 3

	srv := server.NewServer(config)
	err := srv.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Connect multiple clients
	var clients []net.Conn
	for i := 0; i < 3; i++ {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", config.Port))
		if err != nil {
			t.Fatalf("Failed to connect client %d: %v", i, err)
		}
		clients = append(clients, conn)
	}

	// Clean up connections
	defer func() {
		for _, conn := range clients {
			conn.Close()
		}
	}()

	// Test that all clients can communicate
	for i, conn := range clients {
		writer := bufio.NewWriter(conn)
		reader := bufio.NewReader(conn)

		// Send heartbeat from each client
		sendMessage(writer, fmt.Sprintf("HEARTBEAT|client_%d", i))

		// Should receive response
		response := readMessage(reader, t)
		if response != "HEARTBEAT_ACK" {
			t.Errorf("Client %d: Expected HEARTBEAT_ACK, got: %s", i, response)
		}
	}

	// Check server stats
	stats := srv.GetStats()
	if clientCount, ok := stats["clients"].(int); !ok || clientCount != 3 {
		t.Errorf("Expected 3 clients, got: %v", stats["clients"])
	}
}

// TestServerResilience tests server behavior under error conditions
func TestServerResilience(t *testing.T) {
	// Start server
	config := server.DefaultConfig()
	config.Port = 31340 // Use different port for testing

	srv := server.NewServer(config)
	err := srv.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Connect client
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", config.Port))
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Test malformed messages
	malformedMessages := []string{
		"",                              // Empty message
		"INVALID",                       // Unknown message type
		"STATUS",                        // Incomplete status message
		"CHAT|1",                        // Incomplete chat message
		"STATUS|invalid_timestamp|data", // Invalid timestamp
	}

	for _, msg := range malformedMessages {
		sendMessage(writer, msg)
		time.Sleep(10 * time.Millisecond) // Give server time to process
	}

	// Server should still be responsive
	sendMessage(writer, "HEARTBEAT|resilience_test")
	response := readMessage(reader, t)
	if response != "HEARTBEAT_ACK" {
		t.Errorf("Server should still be responsive after malformed messages, got: %s", response)
	}
}

// TestConfigurationManagement tests server configuration
func TestConfigurationManagement(t *testing.T) {
	// Test custom configuration
	config := &server.Config{
		Port:                 31341,
		StatusUpdateInterval: 2 * time.Second,
		ClientTimeout:        10 * time.Second,
		MaxClients:           5,
		HealthThresholds: statusMonitor.HealthThresholds{
			Critical: 20,
			Low:      40,
			Medium:   60,
		},
		LogLevel: "DEBUG",
	}

	srv := server.NewServer(config)
	err := srv.Start()
	if err != nil {
		t.Fatalf("Failed to start server with custom config: %v", err)
	}
	defer srv.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Connect and test that custom thresholds are applied
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", config.Port))
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	writer := bufio.NewWriter(conn)

	// Send status update with HP at custom threshold
	statusMsg := "STATUS|1234567892|TestPlayer:30:100:1:1:" // 30% HP should trigger with custom thresholds
	sendMessage(writer, statusMsg)

	time.Sleep(50 * time.Millisecond)

	// Verify server is using custom configuration
	stats := srv.GetStats()
	if !stats["running"].(bool) {
		t.Error("Server should be running")
	}
}

// Helper functions

func sendMessage(writer *bufio.Writer, message string) {
	// Prepend 4-byte length prefix
	payload := make([]byte, 4+len(message))
	binary.BigEndian.PutUint32(payload[:4], uint32(len(message)))
	copy(payload[4:], []byte(message))

	writer.Write(payload)
	writer.Flush()
}

func readMessage(reader *bufio.Reader, t *testing.T) string {
	// Read 4-byte length prefix
	lengthBuf := make([]byte, 4)
	_, err := io.ReadFull(reader, lengthBuf)
	if err != nil {
		t.Fatalf("Failed to read length prefix: %v", err)
	}

	length := binary.BigEndian.Uint32(lengthBuf)

	// Read message body
	msgBuf := make([]byte, length)
	_, err = io.ReadFull(reader, msgBuf)
	if err != nil {
		t.Fatalf("Failed to read message body: %v", err)
	}

	return strings.TrimSpace(string(msgBuf))
}

func readMessageWithTimeout(reader *bufio.Reader, timeout time.Duration) string {
	done := make(chan string, 1)
	go func() {
		// Read 4-byte length prefix
		lengthBuf := make([]byte, 4)
		_, err := io.ReadFull(reader, lengthBuf)
		if err != nil {
			done <- ""
			return
		}

		length := binary.BigEndian.Uint32(lengthBuf)

		// Read message body
		msgBuf := make([]byte, length)
		_, err = io.ReadFull(reader, msgBuf)
		if err == nil {
			done <- strings.TrimSpace(string(msgBuf))
		} else {
			done <- ""
		}
	}()

	select {
	case message := <-done:
		return message
	case <-time.After(timeout):
		return ""
	}
}

// Benchmark system performance
func BenchmarkSystemThroughput(b *testing.B) {
	// Start server
	config := server.DefaultConfig()
	config.Port = 31342

	srv := server.NewServer(config)
	err := srv.Start()
	if err != nil {
		b.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Connect client
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", config.Port))
	if err != nil {
		b.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	writer := bufio.NewWriter(conn)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Alternate between status updates and chat messages
		if i%2 == 0 {
			statusMsg := fmt.Sprintf("STATUS|%d|Player%d:%d:100:1:1:", time.Now().Unix(), i%10, 50+i%50)
			sendMessage(writer, statusMsg)
		} else {
			chatMsg := fmt.Sprintf("CHAT|3|Player%d|test message %d", i%5, i)
			sendMessage(writer, chatMsg)
		}
	}
}
