package server

import (
	"PandaBot/internal/protocol"
	"bufio"
	"testing"
	"time"
)

func TestSpellCompleteFallback(t *testing.T) {
	config := DefaultConfig()
	s := NewServer(config)

	mc := &mockConn{}
	client := &Client{
		conn:         mc,
		reader:       bufio.NewReader(mc),
		writer:       bufio.NewWriter(mc),
		lastSeen:     time.Now(),
		commandQueue: make([]*QueuedCommand, 0),
	}

	commandID := "test_cmd_123"
	client.currentCommand = &QueuedCommand{
		ID:       commandID,
		Command:  "/ma \"Cure\" <t>",
		State:    CommandInProgress,
		Priority: 5,
	}

	// Message with "id" instead of "command_id"
	body := map[string]interface{}{
		"id":        commandID,
		"timestamp": time.Now().Unix(),
	}

	msg := &protocol.Message{
		Type: protocol.TypeSpellComplete,
		Body: body,
	}

	s.handleJSONSpellComplete(client, msg)

	if client.currentCommand != nil {
		t.Errorf("Expected currentCommand to be cleared, but it is still %v", client.currentCommand)
	}
}

func TestSpellFailedFallback(t *testing.T) {
	config := DefaultConfig()
	s := NewServer(config)

	mc := &mockConn{}
	client := &Client{
		conn:         mc,
		reader:       bufio.NewReader(mc),
		writer:       bufio.NewWriter(mc),
		lastSeen:     time.Now(),
		commandQueue: make([]*QueuedCommand, 0),
	}

	commandID := "test_cmd_456"
	client.currentCommand = &QueuedCommand{
		ID:       commandID,
		Command:  "/ma \"Cure\" <t>",
		State:    CommandInProgress,
		Priority: 5,
	}

	// Message with "id" instead of "command_id"
	body := map[string]interface{}{
		"id":        commandID,
		"error":     "Interrupted",
		"timestamp": time.Now().Unix(),
	}

	msg := &protocol.Message{
		Type: protocol.TypeSpellFailed,
		Body: body,
	}

	s.handleJSONSpellFailed(client, msg)

	if client.currentCommand != nil {
		t.Errorf("Expected currentCommand to be cleared, but it is still %v", client.currentCommand)
	}
}
