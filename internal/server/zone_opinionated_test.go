package server

import (
	"bufio"
	"testing"
	"time"

	"PandaBot/internal/zone"
)

func TestZoneRestrictionOpinionated(t *testing.T) {
	config := DefaultConfig()
	server := NewServer(config)

	client := &Client{
		conn:         &mockConn{},
		reader:       bufio.NewReader(&mockConn{}),
		writer:       bufio.NewWriter(&mockConn{}),
		lastSeen:     time.Now(),
		commandQueue: make([]*QueuedCommand, 0),
		currentZone:  "Zone_241", // Restricted zone
	}

	cmd := &QueuedCommand{
		ID:       "test-1",
		Command:  "/ma \"Cure\" <me>",
		Target:   "<me>",
		Priority: 10,
	}

	// Should be restricted
	if server.isCommandStillNecessary(client, cmd) {
		t.Errorf("Expected command to be restricted in zone %s", client.currentZone)
	}

	client.currentZone = "Zone_1" // Non-restricted zone
	if !server.isCommandStillNecessary(client, cmd) {
		t.Errorf("Expected command to be allowed in zone %s", client.currentZone)
	}

	// Verify hardcoded list
	if !zone.IsRestricted("Zone_241") {
		t.Errorf("Zone_241 should be restricted")
	}
	if zone.IsRestricted("Zone_999") {
		t.Errorf("Zone_999 should not be restricted")
	}
}
