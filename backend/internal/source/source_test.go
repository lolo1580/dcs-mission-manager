package source

import "testing"

// TestIsTestPayload checks recognition of the fixtures, and that ordinary DCS
// payloads are left alone. The detector must never fire on a plausible callsign.
func TestIsTestPayload(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    bool
	}{
		{
			name:    "fixture ownship",
			payload: `{"type":"ownship","name":"Viper 1-1","unitType":"F-16C_50","lat":42}`,
			want:    true,
		},
		{
			name:    "another fixture",
			payload: `{"type":"ownship","name":"Hind 3-1","unitType":"Mi-24P","lat":42}`,
			want:    true,
		},
		{
			name:    "real player",
			payload: `{"type":"ownship","name":"Cellar","unitType":"F-16C_50","lat":42}`,
			want:    false,
		},
		{
			name:    "a callsign that merely contains a digit",
			payload: `{"type":"ownship","name":"Viper 2-1","unitType":"F/A-18C","lat":42}`,
			want:    false,
		},
		{
			name:    "no name at all",
			payload: `{"type":"ownship","lat":42,"lng":41}`,
			want:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsTestPayload([]byte(tc.payload)); got != tc.want {
				t.Fatalf("want %v, got %v", tc.want, got)
			}
		})
	}
}

// TestDetectorLatches ensures the verdict survives later clean traffic: once a
// session is known to be simulated, silence must not make it look real again.
func TestDetectorLatches(t *testing.T) {
	d := NewDetector()
	if d.Observed() {
		t.Fatal("a fresh detector should not report simulated traffic")
	}

	if !d.Observe([]byte(`{"type":"ownship","name":"Viper 1-1"}`)) {
		t.Fatal("the fixture should have been detected")
	}
	if !d.Observed() {
		t.Fatal("the verdict should be latched")
	}

	// A later, ordinary payload must not clear it.
	if !d.Observe([]byte(`{"type":"ownship","name":"Cellar"}`)) {
		t.Fatal("the latch should keep the session marked as simulated")
	}
}

// TestForced checks the environment override, including rejection of a bad value
// (which must not silently tag anything).
func TestForced(t *testing.T) {
	for _, tc := range []struct{ value, want string }{
		{"test", "test"},
		{"live", "live"},
		{"", ""},
		{"nonsense", ""},
	} {
		t.Setenv("DCSMM_SOURCE", tc.value)
		if got := Forced(); got != tc.want {
			t.Errorf("DCSMM_SOURCE=%q: want %q, got %q", tc.value, tc.want, got)
		}
	}
}
