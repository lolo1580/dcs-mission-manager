package led

import (
	"dcsmanager/internal/biosmeta"
	"dcsmanager/internal/dcsbios"
	"dcsmanager/internal/mapping"
	"dcsmanager/internal/panel"
	"dcsmanager/internal/panelservice"
	"path/filepath"
	"testing"
)

func TestDisplayClearsOnModeOrProfileRemoval(t *testing.T) {
	bios := &fakeBios{mem: map[uint16]byte{testAddr: 42}, state: dcsbios.State{Aircraft: "TestJet", Connected: true}}
	panels := &fakePanels{devices: []panelservice.DeviceInfo{{Path: "pz70", VendorID: panel.VendorID, ProductID: panel.ProductPZ70}}, writes: map[string][][]byte{}}
	store := mapping.NewStore(filepath.Join(t.TempDir(), "mappings.json"), nil)
	profile := mapping.Profile{Aircraft: "TestJet", Displays: []mapping.DisplayBinding{{Model: panel.PZ70, Mode: "ALT", Line: "upper", Command: "VALUE"}}}
	if err := store.SetProfile(profile); err != nil {
		t.Fatal(err)
	}
	store.SetOutputsEnabled(true)
	driver := New(bios, panels, store, catalogFor(t, "TestJet", []string{"VALUE"}, testAddr, 255))
	mode := "ALT"
	driver.SetSwitchPos(func() string { return mode })
	if !driver.Sync() {
		t.Fatal("initial display missing")
	}
	mode = "HDG"
	if !driver.Sync() {
		t.Fatal("unmapped mode did not clear")
	}
	report := panels.writes["pz70"][1]
	for i := 1; i <= 10; i++ {
		if report[i] != 255 {
			t.Fatalf("stale digit %d", i)
		}
	}
	mode = "ALT"
	driver.Sync()
	if err := store.SetProfile(mapping.Profile{Aircraft: "TestJet"}); err != nil {
		t.Fatal(err)
	}
	if !driver.Sync() {
		t.Fatal("removed profile did not clear")
	}
}

func TestNumericExportRejectsStringAndPartialWord(t *testing.T) {
	mem := map[uint16]byte{100: 1}
	for _, out := range []biosmeta.Output{{Address: 100, Type: "string"}, {Address: 100, Type: "integer", Mask: 65535}} {
		if _, ok := ReadExportInt(out, mem); ok {
			t.Fatalf("invalid source accepted: %+v", out)
		}
	}
}
