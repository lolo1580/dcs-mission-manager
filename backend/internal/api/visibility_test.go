package api

import (
	"testing"

	"dcsmm/internal/visibility"
)

// TestOptionStringNested covers the shape DCS actually sends: the view settings
// live under "difficulty". Reading them from the top level (as the code used to)
// never matched, so the fog-of-war mode stayed "unknown" and the map stayed
// restricted whatever the mission allowed.
func TestOptionStringNested(t *testing.T) {
	// The real shape, as returned by DCS.getMissionOptions().
	real := map[string]any{
		"difficulty": map[string]any{
			"optionsView":            "optview_all",
			"spectatorExternalViews": false,
		},
		"miscellaneous": map[string]any{"playerName": "Cellar"},
	}
	if got := optionString(real, "optionsView"); got != "optview_all" {
		t.Errorf("nested optionsView = %q, want optview_all", got)
	}

	// The flat form must still work, so a build that flattens the table is fine.
	flat := map[string]any{"optionsView": "optview_onlyallies"}
	if got := optionString(flat, "optionsView"); got != "optview_onlyallies" {
		t.Errorf("flat optionsView = %q, want optview_onlyallies", got)
	}

	// Missing, empty and malformed inputs must not panic and must yield "".
	if got := optionString(nil, "optionsView"); got != "" {
		t.Errorf("nil options = %q, want empty", got)
	}
	if got := optionString(map[string]any{}, "optionsView"); got != "" {
		t.Errorf("empty options = %q, want empty", got)
	}
	if got := optionString(map[string]any{"difficulty": "not a table"}, "optionsView"); got != "" {
		t.Errorf("malformed difficulty = %q, want empty", got)
	}
	if got := optionString(map[string]any{"optionsView": 42}, "optionsView"); got != "" {
		t.Errorf("non-string value = %q, want empty", got)
	}
}

// TestApplyMissionOptionsSetsMode checks the whole chain: the options DCS sends
// must end up selecting the matching visibility mode.
func TestApplyMissionOptionsSetsMode(t *testing.T) {
	cases := []struct {
		value string
		want  visibility.Mode
	}{
		{"optview_all", visibility.All},
		{"optview_onlyallies", visibility.OnlyAllies},
		{"optview_allies", visibility.Allies},
		{"optview_myaircraft", visibility.MyAircraft},
		{"optview_onlymap", visibility.MapOnly},
	}
	for _, tc := range cases {
		// BroadcastMessage needs the hub, which New always provides.
		s := &Server{visibility: visibility.New(false), hub: newHub()}
		s.ApplyMissionOptions(map[string]any{
			"difficulty": map[string]any{"optionsView": tc.value},
		})
		if got := s.visibility.Mode(); got != tc.want {
			t.Errorf("optionsView %q -> mode %q, want %q", tc.value, got, tc.want)
		}
	}

	// With no options at all, the restrictive default must hold.
	s := &Server{visibility: visibility.New(false), hub: newHub()}
	s.ApplyMissionOptions(map[string]any{})
	if got := s.visibility.Mode(); got != visibility.Unknown {
		t.Errorf("no options -> mode %q, want unknown (restrictive)", got)
	}
}
