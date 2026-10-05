package mapping

import "testing"

// TestSetProfileValidation locks the fix that an unauthenticated API cannot grow
// mappings.json unboundedly nor push arbitrary bytes toward DCS-BIOS: aircraft,
// control, command and interface are all bounded and charset-checked.
func TestSetProfileValidation(t *testing.T) {
	s := NewStore(t.TempDir()+"/mappings.json", nil)

	cases := []struct {
		name    string
		profile Profile
	}{
		{"empty aircraft", Profile{Aircraft: ""}},
		{"aircraft with space", Profile{Aircraft: "a b"}},
		{"aircraft with newline", Profile{Aircraft: "a\nb"}},
		{"control with newline", Profile{Aircraft: "F-16C_50", Bindings: []Binding{
			{Control: "GEAR\nx", Command: "GEAR_LEVER"},
		}}},
		{"command too long", Profile{Aircraft: "F-16C_50", Bindings: []Binding{
			{Control: "GEAR", Command: string(make([]byte, 200))},
		}}},
		{"interface with quote", Profile{Aircraft: "F-16C_50", Bindings: []Binding{
			{Control: "GEAR", Command: "GEAR_LEVER", Interface: "set_state\""},
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.SetProfile(tc.profile); err == nil {
				t.Fatalf("expected rejection for %s", tc.name)
			}
		})
	}

	// A valid profile is still accepted.
	if err := s.SetProfile(Profile{Aircraft: "F-16C_50", Bindings: []Binding{
		{Control: "GEAR_DOWN", Command: "GEAR_LEVER", Interface: "set_state"},
	}}); err != nil {
		t.Fatalf("valid profile rejected: %v", err)
	}
}
