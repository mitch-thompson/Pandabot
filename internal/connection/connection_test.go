package connection

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"
)

// ConnectionManager represents the connection management system
type ConnectionManager struct {
	connected              bool
	reconnectAttempts      int
	maxReconnectAttempts   int
	baseReconnectDelay     time.Duration
	maxReconnectDelay      time.Duration
	backoffMultiplier      float64
	currentReconnectDelay  time.Duration
	lastReconnectAttempt   time.Time
	lastHeartbeat          time.Time
	heartbeatInterval      time.Duration
	connectionTimeout      time.Duration
	messageQueue           []QueuedMessage
	maxQueueSize           int
	connectionState        string
	failureRate            float64 // For testing
}

// QueuedMessage represents a message waiting to be sent
type QueuedMessage struct {
	Message   string
	Timestamp time.Time
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connected:             false,
		reconnectAttempts:     0,
		maxReconnectAttempts:  10,
		baseReconnectDelay:    5 * time.Second,
		maxReconnectDelay:     60 * time.Second,
		backoffMultiplier:     1.5,
		currentReconnectDelay: 5 * time.Second,
		heartbeatInterval:     30 * time.Second,
		connectionTimeout:     10 * time.Second,
		messageQueue:          make([]QueuedMessage, 0),
		maxQueueSize:          100,
		connectionState:       "disconnected",
		failureRate:           0.1, // 10% failure rate for testing
	}
}

// CalculateBackoffDelay calculates exponential backoff delay
func (cm *ConnectionManager) CalculateBackoffDelay() time.Duration {
	delay := float64(cm.baseReconnectDelay) * math.Pow(cm.backoffMultiplier, float64(cm.reconnectAttempts))
	maxDelay := float64(cm.maxReconnectDelay)
	return time.Duration(math.Min(delay, maxDelay))
}

// AttemptConnection simulates a connection attempt
func (cm *ConnectionManager) AttemptConnection() bool {
	if cm.connected {
		return true
	}
	
	if cm.connectionState == "connecting" {
		return false
	}
	
	cm.connectionState = "connecting"
	cm.lastReconnectAttempt = time.Now()
	
	// Simulate connection attempt with potential failure
	if rand.Float64() < cm.failureRate {
		// Connection failed
		cm.connectionState = "failed"
		cm.reconnectAttempts++
		cm.currentReconnectDelay = cm.CalculateBackoffDelay()
		
		if cm.reconnectAttempts >= cm.maxReconnectAttempts {
			cm.connectionState = "disconnected"
			return false
		}
		
		return false
	}
	
	// Connection successful
	cm.connected = true
	cm.connectionState = "connected"
	cm.lastHeartbeat = time.Now()
	cm.ResetConnectionState()
	
	return true
}

// ResetConnectionState resets connection tracking
func (cm *ConnectionManager) ResetConnectionState() {
	cm.reconnectAttempts = 0
	cm.currentReconnectDelay = cm.baseReconnectDelay
}

// Disconnect simulates disconnection
func (cm *ConnectionManager) Disconnect(reason string) {
	cm.connected = false
	cm.connectionState = "disconnected"
}

// QueueMessage adds a message to the queue
func (cm *ConnectionManager) QueueMessage(message string) {
	queuedMsg := QueuedMessage{
		Message:   message,
		Timestamp: time.Now(),
	}
	
	cm.messageQueue = append(cm.messageQueue, queuedMsg)
	
	// Limit queue size
	if len(cm.messageQueue) > cm.maxQueueSize {
		cm.messageQueue = cm.messageQueue[1:]
	}
}

// SendMessage attempts to send a message
func (cm *ConnectionManager) SendMessage(message string) bool {
	if !cm.connected {
		cm.QueueMessage(message)
		return false
	}
	
	// Simulate send with potential failure
	if rand.Float64() < cm.failureRate {
		cm.Disconnect("Send error")
		cm.QueueMessage(message)
		return false
	}
	
	return true
}

// FlushMessageQueue sends all queued messages
func (cm *ConnectionManager) FlushMessageQueue() (int, int) {
	if !cm.connected || len(cm.messageQueue) == 0 {
		return 0, 0
	}
	
	sent := 0
	failed := 0
	
	for i := len(cm.messageQueue) - 1; i >= 0; i-- {
		if cm.SendMessage(cm.messageQueue[i].Message) {
			cm.messageQueue = append(cm.messageQueue[:i], cm.messageQueue[i+1:]...)
			sent++
		} else {
			failed++
			break // Stop on first failure to maintain order
		}
	}
	
	return sent, failed
}

// ShouldReconnect determines if a reconnection attempt should be made
func (cm *ConnectionManager) ShouldReconnect() bool {
	if cm.connected || cm.connectionState == "connecting" {
		return false
	}
	
	if cm.reconnectAttempts >= cm.maxReconnectAttempts {
		return false
	}
	
	return time.Since(cm.lastReconnectAttempt) >= cm.currentReconnectDelay
}

// ShouldSendHeartbeat determines if a heartbeat should be sent
func (cm *ConnectionManager) ShouldSendHeartbeat() bool {
	if !cm.connected {
		return false
	}
	
	return time.Since(cm.lastHeartbeat) >= cm.heartbeatInterval
}

// SendHeartbeat sends a heartbeat
func (cm *ConnectionManager) SendHeartbeat() bool {
	if !cm.connected {
		return false
	}
	
	success := cm.SendMessage("HEARTBEAT")
	if success {
		cm.lastHeartbeat = time.Now()
	}
	
	return success
}

// GetQueueSize returns the current message queue size
func (cm *ConnectionManager) GetQueueSize() int {
	return len(cm.messageQueue)
}

// Property 21: Automatic reconnection with backoff
func TestProperty21_AutomaticReconnectionWithBackoff(t *testing.T) {
	for i := 0; i < 100; i++ {
		cm := NewConnectionManager()
		cm.failureRate = 0.8 // High failure rate to test backoff
		
		initialDelay := cm.currentReconnectDelay
		
		// Simulate multiple failed connection attempts
		for attempt := 0; attempt < 5; attempt++ {
			if cm.ShouldReconnect() {
				success := cm.AttemptConnection()
				if !success {
					// Verify backoff delay increases
					newDelay := cm.currentReconnectDelay
					if attempt > 0 && newDelay <= initialDelay {
						t.Errorf("Iteration %d: Backoff delay should increase, got %v <= %v", i, newDelay, initialDelay)
					}
					initialDelay = newDelay
					
					// Verify delay doesn't exceed maximum
					if newDelay > cm.maxReconnectDelay {
						t.Errorf("Iteration %d: Backoff delay exceeded maximum: %v > %v", i, newDelay, cm.maxReconnectDelay)
					}
				}
			}
		}
		
		// Verify max attempts are respected
		if cm.reconnectAttempts > cm.maxReconnectAttempts {
			t.Errorf("Iteration %d: Exceeded max reconnect attempts: %d > %d", i, cm.reconnectAttempts, cm.maxReconnectAttempts)
		}
	}
}

// Property 10: Connection monitoring and recovery
func TestProperty10_ConnectionMonitoringAndRecovery(t *testing.T) {
	for i := 0; i < 100; i++ {
		cm := NewConnectionManager()
		cm.failureRate = 0.3 // Moderate failure rate
		
		// Establish initial connection
		cm.failureRate = 0.0 // Ensure first connection succeeds
		if !cm.AttemptConnection() {
			t.Errorf("Iteration %d: Initial connection should succeed", i)
			continue
		}
		
		cm.failureRate = 0.3 // Reset failure rate
		
		// Simulate heartbeat monitoring
		initialHeartbeat := cm.lastHeartbeat
		
		// Fast-forward time to trigger heartbeat
		cm.lastHeartbeat = time.Now().Add(-cm.heartbeatInterval - time.Second)
		
		if !cm.ShouldSendHeartbeat() {
			t.Errorf("Iteration %d: Should send heartbeat after interval", i)
		}
		
		// Send heartbeat
		cm.SendHeartbeat()
		
		// Verify heartbeat timestamp updated
		if cm.lastHeartbeat.Before(initialHeartbeat) {
			t.Errorf("Iteration %d: Heartbeat timestamp should be updated", i)
		}
		
		// Simulate connection loss and recovery
		cm.Disconnect("Test disconnect")
		
		if cm.connected {
			t.Errorf("Iteration %d: Should be disconnected after Disconnect()", i)
		}
		
		// Attempt recovery
		cm.failureRate = 0.0 // Ensure recovery succeeds
		if cm.ShouldReconnect() {
			success := cm.AttemptConnection()
			if !success {
				t.Errorf("Iteration %d: Recovery connection should succeed", i)
			}
		}
	}
}

// Property 23: Graceful degradation and message queuing
func TestProperty23_GracefulDegradationAndMessageQueuing(t *testing.T) {
	for i := 0; i < 100; i++ {
		cm := NewConnectionManager()
		
		// Start disconnected
		cm.connected = false
		
		// Send messages while disconnected
		messageCount := rand.Intn(20) + 5
		for j := 0; j < messageCount; j++ {
			message := fmt.Sprintf("Message %d", j)
			success := cm.SendMessage(message)
			
			if success {
				t.Errorf("Iteration %d: Send should fail when disconnected", i)
			}
		}
		
		// Verify messages were queued
		queueSize := cm.GetQueueSize()
		if queueSize != messageCount {
			t.Errorf("Iteration %d: Expected %d queued messages, got %d", i, messageCount, queueSize)
		}
		
		// Connect and flush queue
		cm.connected = true
		cm.failureRate = 0.0 // Ensure flush succeeds
		
		sent, failed := cm.FlushMessageQueue()
		
		if sent != messageCount {
			t.Errorf("Iteration %d: Expected to send %d messages, sent %d", i, messageCount, sent)
		}
		
		if failed != 0 {
			t.Errorf("Iteration %d: Expected 0 failed messages, got %d", i, failed)
		}
		
		// Verify queue is empty after flush
		if cm.GetQueueSize() != 0 {
			t.Errorf("Iteration %d: Queue should be empty after successful flush", i)
		}
	}
}

// Property 24: State synchronization after reconnection
func TestProperty24_StateSynchronizationAfterReconnection(t *testing.T) {
	for i := 0; i < 100; i++ {
		cm := NewConnectionManager()
		
		// Establish connection
		cm.failureRate = 0.0
		cm.AttemptConnection()
		
		// Queue some messages while connected
		initialMessages := rand.Intn(10) + 1
		for j := 0; j < initialMessages; j++ {
			cm.QueueMessage(fmt.Sprintf("Initial message %d", j))
		}
		
		initialQueueSize := cm.GetQueueSize()
		
		// Simulate disconnection
		cm.Disconnect("Test disconnect")
		
		// Add more messages while disconnected
		disconnectedMessages := rand.Intn(10) + 1
		for j := 0; j < disconnectedMessages; j++ {
			cm.SendMessage(fmt.Sprintf("Disconnected message %d", j))
		}
		
		expectedQueueSize := initialQueueSize + disconnectedMessages
		if cm.GetQueueSize() != expectedQueueSize {
			t.Errorf("Iteration %d: Expected queue size %d, got %d", i, expectedQueueSize, cm.GetQueueSize())
		}
		
		// Reconnect
		cm.AttemptConnection()
		
		// Verify state is properly synchronized
		if !cm.connected {
			t.Errorf("Iteration %d: Should be connected after successful reconnection", i)
		}
		
		if cm.connectionState != "connected" {
			t.Errorf("Iteration %d: Connection state should be 'connected', got '%s'", i, cm.connectionState)
		}
		
		if cm.reconnectAttempts != 0 {
			t.Errorf("Iteration %d: Reconnect attempts should be reset to 0, got %d", i, cm.reconnectAttempts)
		}
		
		if cm.currentReconnectDelay != cm.baseReconnectDelay {
			t.Errorf("Iteration %d: Reconnect delay should be reset to base delay", i)
		}
		
		// Verify queued messages can be flushed
		sent, _ := cm.FlushMessageQueue()
		if sent == 0 && cm.GetQueueSize() > 0 {
			t.Errorf("Iteration %d: Should be able to flush messages after reconnection", i)
		}
	}
}

// Benchmark connection operations
func BenchmarkConnectionOperations(b *testing.B) {
	cm := NewConnectionManager()
	cm.failureRate = 0.1
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !cm.connected {
			cm.AttemptConnection()
		} else {
			message := fmt.Sprintf("Message %d", i)
			cm.SendMessage(message)
			
			if i%10 == 0 {
				cm.FlushMessageQueue()
			}
		}
	}
}