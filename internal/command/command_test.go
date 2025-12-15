package command

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// CommandExecutor represents the command execution system
type CommandExecutor struct {
	commands     []Command
	failureRate  float64
	maxRetries   int
	errorReports []ErrorReport
}

// Command represents a game command with metadata
type Command struct {
	ID       string
	Command  string
	Priority int
	Retries  int
}

// ErrorReport represents an error report sent back to server
type ErrorReport struct {
	CommandID string
	Error     string
	Timestamp time.Time
}

// Priority levels
const (
	PriorityCritical = 1
	PriorityHigh     = 2
	PriorityMedium   = 3
	PriorityLow      = 4
)

// NewCommandExecutor creates a new command executor
func NewCommandExecutor() *CommandExecutor {
	return &CommandExecutor{
		commands:     make([]Command, 0),
		failureRate:  0.1, // 10% failure rate for testing
		maxRetries:   3,
		errorReports: make([]ErrorReport, 0),
	}
}

// AddCommand adds a command to the priority queue
func (ce *CommandExecutor) AddCommand(id, command string, priority int) {
	cmd := Command{
		ID:       id,
		Command:  command,
		Priority: priority,
		Retries:  0,
	}
	
	// Insert based on priority (lower number = higher priority)
	inserted := false
	for i, existing := range ce.commands {
		if existing.Priority > priority {
			ce.commands = append(ce.commands[:i], append([]Command{cmd}, ce.commands[i:]...)...)
			inserted = true
			break
		}
	}
	
	if !inserted {
		ce.commands = append(ce.commands, cmd)
	}
}

// ExecuteNext executes the next command in the queue
func (ce *CommandExecutor) ExecuteNext() (bool, error) {
	if len(ce.commands) == 0 {
		return false, fmt.Errorf("no commands in queue")
	}
	
	cmd := ce.commands[0]
	ce.commands = ce.commands[1:]
	
	// Simulate command execution with potential failure
	if rand.Float64() < ce.failureRate {
		cmd.Retries++
		if cmd.Retries <= ce.maxRetries {
			// Re-add to queue for retry
			ce.AddCommand(cmd.ID, cmd.Command, cmd.Priority)
			return false, fmt.Errorf("command execution failed, retry %d/%d", cmd.Retries, ce.maxRetries)
		} else {
			// Max retries exceeded
			ce.errorReports = append(ce.errorReports, ErrorReport{
				CommandID: cmd.ID,
				Error:     "max retries exceeded",
				Timestamp: time.Now(),
			})
			return false, fmt.Errorf("command failed after %d retries", ce.maxRetries)
		}
	}
	
	return true, nil
}

// ValidateCommand validates a command string
func (ce *CommandExecutor) ValidateCommand(command string) error {
	if command == "" {
		return fmt.Errorf("empty command")
	}
	
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return fmt.Errorf("empty command after trimming")
	}
	
	validPrefixes := []string{"/ma", "/ja", "/ws", "/item", "/pet", "/echo", "/tell", "/party"}
	hasValidPrefix := false
	
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(strings.ToLower(trimmed), strings.ToLower(prefix)) {
			hasValidPrefix = true
			break
		}
	}
	
	if !hasValidPrefix {
		return fmt.Errorf("invalid command prefix")
	}
	
	return nil
}

// GetQueueLength returns the current queue length
func (ce *CommandExecutor) GetQueueLength() int {
	return len(ce.commands)
}

// GetErrorReports returns all error reports
func (ce *CommandExecutor) GetErrorReports() []ErrorReport {
	return ce.errorReports
}

// Property 1: Command execution and parsing
func TestProperty1_CommandExecutionAndParsing(t *testing.T) {
	for i := 0; i < 100; i++ {
		executor := NewCommandExecutor()
		
		// Generate random valid commands
		commands := []string{
			"/ma \"Cure IV\" PlayerName",
			"/ja \"Provoke\" <t>",
			"/ws \"Savage Blade\" <t>",
			"/item \"Hi-Potion\" <me>",
			"/echo Test message",
		}
		
		for j, cmd := range commands {
			id := fmt.Sprintf("cmd_%d", j)
			priority := rand.Intn(4) + 1
			
			// Validate command
			err := executor.ValidateCommand(cmd)
			if err != nil {
				t.Errorf("Iteration %d: Valid command failed validation: %v", i, err)
			}
			
			// Add to queue
			executor.AddCommand(id, cmd, priority)
		}
		
		// Execute all commands
		initialCount := executor.GetQueueLength()
		if initialCount != len(commands) {
			t.Errorf("Iteration %d: Expected %d commands in queue, got %d", i, len(commands), initialCount)
		}
		
		// Process queue
		for executor.GetQueueLength() > 0 {
			executor.ExecuteNext()
		}
	}
}

// Property 2: Error handling for malformed commands
func TestProperty2_ErrorHandlingMalformedCommands(t *testing.T) {
	for i := 0; i < 100; i++ {
		executor := NewCommandExecutor()
		
		// Generate malformed commands
		malformedCommands := []string{
			"",                    // Empty
			"   ",                 // Whitespace only
			"invalid command",     // No valid prefix
			"random text",         // Invalid format
			"/unknown command",    // Unknown prefix
		}
		
		for _, cmd := range malformedCommands {
			err := executor.ValidateCommand(cmd)
			if err == nil {
				t.Errorf("Iteration %d: Malformed command should have failed validation: '%s'", i, cmd)
			}
		}
		
		// Valid commands should still work
		validCmd := "/ma \"Cure\" <me>"
		err := executor.ValidateCommand(validCmd)
		if err != nil {
			t.Errorf("Iteration %d: Valid command failed validation: %v", i, err)
		}
	}
}

// Property 3: Command queue management
func TestProperty3_CommandQueueManagement(t *testing.T) {
	for i := 0; i < 100; i++ {
		executor := NewCommandExecutor()
		
		// Add commands with different priorities
		commands := []struct {
			id       string
			command  string
			priority int
		}{
			{"low1", "/echo Low priority 1", PriorityLow},
			{"critical1", "/ma \"Cure IV\" <me>", PriorityCritical},
			{"medium1", "/ma \"Cure\" <me>", PriorityMedium},
			{"high1", "/ma \"Stona\" <me>", PriorityHigh},
			{"low2", "/echo Low priority 2", PriorityLow},
			{"critical2", "/ma \"Curaga\" <me>", PriorityCritical},
		}
		
		// Add in random order
		shuffled := make([]struct {
			id       string
			command  string
			priority int
		}, len(commands))
		copy(shuffled, commands)
		
		for j := len(shuffled) - 1; j > 0; j-- {
			k := rand.Intn(j + 1)
			shuffled[j], shuffled[k] = shuffled[k], shuffled[j]
		}
		
		for _, cmd := range shuffled {
			executor.AddCommand(cmd.id, cmd.command, cmd.priority)
		}
		
		// Verify priority ordering
		lastPriority := 0
		for executor.GetQueueLength() > 0 {
			// Peek at next command (simulate)
			if len(executor.commands) > 0 {
				currentPriority := executor.commands[0].Priority
				if currentPriority < lastPriority {
					t.Errorf("Iteration %d: Priority ordering violated: %d should not come after %d", i, currentPriority, lastPriority)
				}
				lastPriority = currentPriority
			}
			
			executor.ExecuteNext()
		}
	}
}

// Property 4: Error reporting
func TestProperty4_ErrorReporting(t *testing.T) {
	for i := 0; i < 100; i++ {
		executor := NewCommandExecutor()
		executor.failureRate = 1.0 // Force all commands to fail initially
		
		// Add commands that will fail
		commandCount := rand.Intn(5) + 1
		for j := 0; j < commandCount; j++ {
			id := fmt.Sprintf("cmd_%d", j)
			cmd := "/ma \"Cure\" <me>"
			executor.AddCommand(id, cmd, PriorityMedium)
		}
		
		initialQueueSize := executor.GetQueueLength()
		
		// Process all commands (they will fail and retry)
		attempts := 0
		maxAttempts := (executor.maxRetries + 1) * commandCount * 2 // Safety margin
		
		for executor.GetQueueLength() > 0 && attempts < maxAttempts {
			executor.ExecuteNext()
			attempts++
		}
		
		// Check that error reports were generated
		errorReports := executor.GetErrorReports()
		if len(errorReports) != initialQueueSize {
			t.Errorf("Iteration %d: Expected %d error reports, got %d", i, initialQueueSize, len(errorReports))
		}
		
		// Verify error report content
		for _, report := range errorReports {
			if report.CommandID == "" {
				t.Errorf("Iteration %d: Error report missing command ID", i)
			}
			if report.Error == "" {
				t.Errorf("Iteration %d: Error report missing error message", i)
			}
			if report.Timestamp.IsZero() {
				t.Errorf("Iteration %d: Error report missing timestamp", i)
			}
		}
	}
}

// Benchmark command queue operations
func BenchmarkCommandQueueOperations(b *testing.B) {
	executor := NewCommandExecutor()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("cmd_%d", i)
		cmd := "/ma \"Cure\" <me>"
		priority := rand.Intn(4) + 1
		
		executor.AddCommand(id, cmd, priority)
		
		if executor.GetQueueLength() > 100 {
			executor.ExecuteNext()
		}
	}
}