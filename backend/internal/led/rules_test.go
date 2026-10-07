package led

import (
	"dcsmanager/internal/dcsbios"
	"dcsmanager/internal/mapping"
	"dcsmanager/internal/panel"
	"dcsmanager/internal/panelservice"
	"path/filepath"
	"testing"
)

func TestGearRulesAndDisconnect(t *testing.T) {
	bios := &fakeBios{mem: map[uint16]byte{testAddr: 1}, state: dcsbios.State{Aircraft: "A", Connected: true}}
	panels := &fakePanels{devices: []panelservice.DeviceInfo{pz55Device()}, writes: map[string][][]byte{}}
	store := mapping.NewStore(filepath.Join(t.TempDir(), "mappings.json"), nil)
	profile := mapping.Profile{Aircraft: "A", Outputs: []mapping.OutputBinding{{Model: panel.PZ55, Target: panel.TargetGearUpper, Command: "VALUE", Rules: []mapping.OutputRule{
		{Command: "VALUE", Operator: "eq", Value: 1, Color: "green"},
		{Command: "VALUE", Operator: "eq", Value: 2, Color: "red"},
	}}}}
	if err := store.SetProfile(profile); err != nil {
		t.Fatal(err)
	}
	store.SetOutputsEnabled(true)
	driver := New(bios, panels, store, catalogFor(t, "A", []string{"VALUE"}, testAddr, 255))
	for _, test := range []struct{ value, expected byte }{{1, 1}, {2, 8}, {0, 0}} {
		bios.mem[testAddr] = test.value
		driver.Sync()
		reports := panels.writes["pz55-1"]
		if reports[len(reports)-1][1] != test.expected {
			t.Fatalf("value %d: %v", test.value, reports)
		}
	}
	bios.mem[testAddr] = 1
	driver.Sync()
	bios.state.Connected = false
	if !driver.Sync() {
		t.Fatal("disconnect did not clear")
	}
	reports := panels.writes["pz55-1"]
	if reports[len(reports)-1][1] != 0 {
		t.Fatal("stale LED after disconnect")
	}
}
