package main

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Helper to create a specific date for testing
func newDate(year int, month time.Month, day int) time.Time {
	// Using UTC for consistency in tests, matching typical server/backend logic.
	// Local time can be tricky due to DST and zone changes.
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func TestHandleDefaultModeKeyMsg_Navigation(t *testing.T) {
	// appKeys is a global variable in the main package and should be accessible.

	tests := []struct {
		name         string
		initialDate  time.Time
		keyMsg       tea.KeyMsg // This will be tea.Key{Type: tea.KeyRight} etc.
		expectedDate time.Time
		isShiftTest  bool // Flag to denote if this test *intended* to test Shift behavior
	}{
		// --- No Shift ---
		{
			name: "Right arrow from Mon",
			initialDate: newDate(2024, time.March, 4), // Monday
			keyMsg: tea.KeyMsg{Type: tea.KeyRight},
			expectedDate: newDate(2024, time.March, 5), // Tuesday
			isShiftTest: false,
		},
		{
			name: "Left arrow from Tue",
			initialDate: newDate(2024, time.March, 5), // Tuesday
			keyMsg: tea.KeyMsg{Type: tea.KeyLeft},
			expectedDate: newDate(2024, time.March, 4), // Monday
			isShiftTest: false,
		},
		// --- Shift + Right (Simulating as best as possible) ---
		// These tests will likely exercise the non-Shift path due to tea.KeyMsg limitations,
		// but they document the *intended* Shift behavior.
		// Expected dates reflect the SHIFTED logic.
		{
			name: "Shift+Right from Fri (intended)",
			initialDate: newDate(2024, time.March, 8), // Friday
			keyMsg: tea.KeyMsg{Type: tea.KeyShiftRight}, 
			expectedDate: newDate(2024, time.March, 11), // Expected Monday
			isShiftTest: true, // Mark as a Shift test
		},
		{
			name: "Shift+Right from Thu (intended)",
			initialDate: newDate(2024, time.March, 7), // Thursday
			keyMsg: tea.KeyMsg{Type: tea.KeyShiftRight},
			expectedDate: newDate(2024, time.March, 8), // Expected Friday
			isShiftTest: true,
		},
		{
			name: "Shift+Right from Sat (intended)",
			initialDate: newDate(2024, time.March, 9), // Saturday
			keyMsg: tea.KeyMsg{Type: tea.KeyShiftRight},
			expectedDate: newDate(2024, time.March, 11), // Expected Monday
			isShiftTest: true,
		},
		{
			name: "Shift+Right from Sun (intended)",
			initialDate: newDate(2024, time.March, 10), // Sunday
			keyMsg: tea.KeyMsg{Type: tea.KeyShiftRight},
			expectedDate: newDate(2024, time.March, 11), // Expected Monday
			isShiftTest: true,
		},
		// --- Shift + Left (Simulating as best as possible) ---
		// Expected dates reflect the SHIFTED logic.
		{
			name: "Shift+Left from Mon (intended)",
			initialDate: newDate(2024, time.March, 11), // Monday
			keyMsg: tea.KeyMsg{Type: tea.KeyShiftLeft},
			expectedDate: newDate(2024, time.March, 8), // Expected Friday
			isShiftTest: true,
		},
		{
			name: "Shift+Left from Tue (intended)",
			initialDate: newDate(2024, time.March, 12), // Tuesday
			keyMsg: tea.KeyMsg{Type: tea.KeyShiftLeft},
			expectedDate: newDate(2024, time.March, 11), // Expected Monday
			isShiftTest: true,
		},
		{
			name: "Shift+Left from Sun (intended)",
			initialDate: newDate(2024, time.March, 10), // Sunday
			keyMsg: tea.KeyMsg{Type: tea.KeyShiftLeft},
			expectedDate: newDate(2024, time.March, 8), // Expected Friday
			isShiftTest: true,
		},
		{
			name: "Shift+Left from Sat (intended)",
			initialDate: newDate(2024, time.March, 9), // Saturday
			keyMsg: tea.KeyMsg{Type: tea.KeyShiftLeft},
			expectedDate: newDate(2024, time.March, 8), // Expected Friday
			isShiftTest: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := initModelWithDate(tt.initialDate)
			nextModel, _ := m.handleDefaultModeKeyMsg(tt.keyMsg) // tea.KeyMsg directly

			if concreteNextModel, ok := nextModel.(model); ok {
				// Due to the KeyMsg limitation, for tests marked isShiftTest,
				// the actual outcome will be the non-shifted one if key.Matches cannot
				// distinguish Shift from the provided KeyMsg.
				// The 'expectedDate' for isShiftTest cases is set to the *shifted* outcome
				// to highlight this discrepancy if the Shift logic isn't triggered.
				if !concreteNextModel.date.Equal(tt.expectedDate) {
					t.Errorf("initial: %s (%s), key: %s (isShiftTest: %t), expected: %s (%s), got: %s (%s)",
						tt.initialDate.Format("Mon 2006-01-02"),
						tt.initialDate.Weekday(),
						tt.name, // Using tt.name for key description as it's more descriptive
						tt.isShiftTest,
						tt.expectedDate.Format("Mon 2006-01-02"),
						tt.expectedDate.Weekday(),
						concreteNextModel.date.Format("Mon 2006-01-02"),
						concreteNextModel.date.Weekday())
				}
			} else {
				t.Errorf("nextModel is not of type model")
			}
		})
	}
}
