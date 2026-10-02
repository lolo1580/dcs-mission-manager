package dcsdata

import (
	"os"
	"path/filepath"
	"testing"

	"dcsmanager/internal/lua"
)

// TestLoadLogbookOnRealFile parses the machine's own logbook when present.
func TestLoadLogbookOnRealFile(t *testing.T) {
	sg := filepath.Join(os.Getenv("USERPROFILE"), "Saved Games", "DCS")
	if _, err := os.Stat(LogbookPath(sg)); err != nil {
		t.Skip("no DCS install on this machine")
	}
	lb, err := LoadLogbook(sg)
	if err != nil {
		t.Fatalf("LoadLogbook: %v", err)
	}
	if len(lb.Players) == 0 {
		t.Fatal("expected at least one player")
	}
	for _, p := range lb.Players {
		if p.Name == "" {
			t.Errorf("player without a name: %+v", p)
		}
		for _, a := range p.Aircraft {
			if a.Type == "" {
				t.Errorf("aircraft without a type: %+v", a)
			}
		}
		t.Logf("player %q (rank %q, squadron %q): %d airframe(s), %.1f h total, awards %v",
			p.Name, p.Rank, p.Squadron, len(p.Aircraft), p.TotalFlightHours, p.Awards)
		for _, a := range p.Aircraft {
			t.Logf("   %-14s %8.1f h  %d landings  %d deaths", a.Type, a.FlightHours, a.Landings, a.Deaths)
		}
	}
}

// TestParseLogbookSynthetic checks the parser on a hand-written document.
func TestParseLogbookSynthetic(t *testing.T) {
	src := []byte(`
logbook = {
	["currentPlayerName"] = "Laurent Keller",
	["players"] = {
		[1] = {
			["name"] = "Laurent Keller",
			["rankName"] = "Captain",
			["squadronName"] = "FlSt17",
			["awards"] = { [2] = {}, [22] = {} },
			["statistics"] = {
				["missionsCount"] = 42,
				["totalScore"] = 1500,
				["M-2000C"] = { ["flightHours"] = 120.5, ["landings"] = 30, ["deaths"] = 2, ["aaKills"] = 4 },
				["F-16C_50"] = { ["flightHours"] = 300.25, ["landings"] = 80, ["deaths"] = 5 },
			},
		},
	},
}
`)
	root, err := lua.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	lb := parseLogbook(root)
	if lb.CurrentPlayer != "Laurent Keller" {
		t.Errorf("CurrentPlayer = %q", lb.CurrentPlayer)
	}
	if len(lb.Players) != 1 {
		t.Fatalf("players = %d, want 1", len(lb.Players))
	}
	p := lb.Players[0]
	if p.Rank != "Captain" || p.Squadron != "FlSt17" {
		t.Errorf("rank/squadron wrong: %+v", p)
	}
	if len(p.Awards) != 2 || p.Awards[0] != 2 || p.Awards[1] != 22 {
		t.Errorf("awards = %v, want [2 22]", p.Awards)
	}
	if p.Aggregate["missionsCount"] != float64(42) {
		t.Errorf("aggregate missionsCount = %v, want 42", p.Aggregate["missionsCount"])
	}
	if len(p.Aircraft) != 2 {
		t.Fatalf("aircraft = %d, want 2", len(p.Aircraft))
	}
	// Sorted by flight hours, so the F-16 (300.25 h) comes first.
	if p.Aircraft[0].Type != "F-16C_50" {
		t.Errorf("first aircraft = %q, want F-16C_50 (most hours)", p.Aircraft[0].Type)
	}
	if got := p.TotalFlightHours; got != 420.75 {
		t.Errorf("TotalFlightHours = %v, want 420.75", got)
	}
	if p.Aircraft[1].AAKills != 4 || p.Aircraft[1].Landings != 30 {
		t.Errorf("M-2000C parsed wrong: %+v", p.Aircraft[1])
	}
}
