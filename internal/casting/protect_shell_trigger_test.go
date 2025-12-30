package casting

import (
	"PandaBot/internal/entity"
	"testing"
)

type MockEngine struct {
	LastRequest *CastRequest
}

func (m *MockEngine) RequestCast(request *CastRequest) error {
	m.LastRequest = request
	return nil
}

func TestProtectShellTriggers(t *testing.T) {
	mockEngine := &MockEngine{}
	tp := NewTriggerProcessor(mockEngine)

	casterJobLevels := map[string]int{"WHM": 75}
	partyMembers := []*entity.Entity{
		{Name: "Caster", Job: "WHM", JobLevel: 75},
		{Name: "Sender", Job: "WAR", JobLevel: 75},
	}

	tests := []struct {
		triggerType  string
		sender       string
		expectedType CastType
	}{
		{"protect", "Sender", CastTypeProtect},
		{"shell", "Sender", CastTypeShell},
	}

	for _, tt := range tests {
		t.Run(tt.triggerType, func(t *testing.T) {
			_, err := tp.ProcessTriggerEvent(tt.triggerType, tt.sender, 3, "Caster", 500, casterJobLevels, partyMembers)
			if err != nil {
				t.Fatalf("Failed to process trigger %s: %v", tt.triggerType, err)
			}

			if mockEngine.LastRequest == nil {
				t.Fatal("No request sent to engine")
			}

			if mockEngine.LastRequest.Type != tt.expectedType {
				t.Errorf("Expected CastType %v, got %v", tt.expectedType, mockEngine.LastRequest.Type)
			}

			if mockEngine.LastRequest.Target != tt.sender {
				t.Errorf("Expected target %s, got %s", tt.sender, mockEngine.LastRequest.Target)
			}
		})
	}
}
