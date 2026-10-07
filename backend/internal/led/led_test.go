package led

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"dcsmanager/internal/biosmeta"
	"dcsmanager/internal/dcsbios"
	"dcsmanager/internal/mapping"
	"dcsmanager/internal/panel"
	"dcsmanager/internal/panelservice"
)

// fakeBios is a DCS-BIOS client stub: a fixed memory image and state.
type fakeBios struct {
	mem   map[uint16]byte
	state dcsbios.State
}

func (f *fakeBios) Memory() map[uint16]byte { return f.mem }
func (f *fakeBios) State() dcsbios.State    { return f.state }

// fakePanels records the reports written to each panel.
type fakePanels struct {
	devices []panelservice.DeviceInfo
	writes  map[string][][]byte
}

func (f *fakePanels) Devices() []panelservice.DeviceInfo { return f.devices }
func (f *fakePanels) Write(path string, report []byte) error {
	f.writes[path] = append(f.writes[path], report)
	return nil
}

// catalogFor writes a one-aircraft DCS-BIOS metadata file giving every id an
// exported integer at the same address and mask, and returns a loader for it.
func catalogFor(t *testing.T, module string, ids []string, address, mask uint16) func(string) (*biosmeta.Catalog, error) {
	t.Helper()
	sg := t.TempDir()
	dir := biosmeta.JSONDir(sg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{"Lights":{`
	for i, id := range ids {
		if i > 0 {
			raw += ","
		}
		raw += `"` + id + `":{"category":"Lights","control_type":"led","identifier":"` + id +
			`","inputs":[],"outputs":[{"address":` + strconv.Itoa(int(address)) +
			`,"mask":` + strconv.Itoa(int(mask)) + `,"max_value":1,"type":"integer"}]}`
	}
	raw += `}}`
	if err := os.WriteFile(filepath.Join(dir, module+".json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	return func(string) (*biosmeta.Catalog, error) { return biosmeta.LoadModule(sg, module) }
}

// The address the fake metadata exports every test output at.
const (
	testAddr = 0x0100
	testMask = 0x0001
)

// gearProfile drives the three PZ55 gear lights from three separate controls, as
// the shipped starter profiles do.
func gearProfile() mapping.Profile {
	return mapping.Profile{
		Aircraft: "TestJet",
		Outputs: []mapping.OutputBinding{
			{Model: panel.PZ55, Target: panel.TargetGearUpper, Command: "LIGHT_GEAR_N", Color: "green"},
			{Model: panel.PZ55, Target: panel.TargetGearLeft, Command: "LIGHT_GEAR_L", Color: "red"},
			{Model: panel.PZ55, Target: panel.TargetGearRight, Command: "LIGHT_GEAR_R", Color: "green"},
		},
	}
}

func pz55Device() panelservice.DeviceInfo {
	return panelservice.DeviceInfo{Path: "pz55-1", VendorID: panel.VendorID, ProductID: panel.ProductPZ55}
}

// TestSyncDoesNothingWhenDisabled locks the opt-in rule: no LED is lit while
// sending is off.
func TestSyncDoesNothingWhenDisabled(t *testing.T) {
	bios := &fakeBios{mem: map[uint16]byte{testAddr: 1}, state: dcsbios.State{Aircraft: "TestJet", Connected: true}}
	panels := &fakePanels{devices: []panelservice.DeviceInfo{pz55Device()}, writes: map[string][][]byte{}}
	store := mapping.NewStore(filepath.Join(t.TempDir(), "mappings.json"), func(string) error { return nil })
	if err := store.SetProfile(gearProfile()); err != nil {
		t.Fatal(err)
	}
	d := New(bios, panels, store, catalogFor(t, "TestJet", []string{"LIGHT_GEAR_N", "LIGHT_GEAR_L", "LIGHT_GEAR_R"}, testAddr, testMask))

	if d.Sync() {
		t.Fatal("Sync must do nothing when sending is disabled")
	}
	if len(panels.writes) != 0 {
		t.Fatalf("wrote %v while disabled", panels.writes)
	}
}

// TestSyncLightsGearFromExportedValue checks a set bit lights the indicator and a
// cleared bit turns it off, and that an unchanged value is not written twice.
func TestSyncLightsGearFromExportedValue(t *testing.T) {
	bios := &fakeBios{mem: map[uint16]byte{testAddr: 1}, state: dcsbios.State{Aircraft: "TestJet", Connected: true}}
	panels := &fakePanels{devices: []panelservice.DeviceInfo{pz55Device()}, writes: map[string][][]byte{}}
	store := mapping.NewStore(filepath.Join(t.TempDir(), "mappings.json"), func(string) error { return nil })
	if err := store.SetProfile(gearProfile()); err != nil {
		t.Fatal(err)
	}
	d := New(bios, panels, store, catalogFor(t, "TestJet", []string{"LIGHT_GEAR_N", "LIGHT_GEAR_L", "LIGHT_GEAR_R"}, testAddr, testMask))
	store.SetOutputsEnabled(true)

	if !d.Sync() {
		t.Fatal("Sync should have written the gear lights")
	}
	got := panels.writes["pz55-1"]
	if len(got) != 2 {
		t.Fatalf("PZ55 takes two reports, got %d", len(got))
	}
	// Green (bit 0) for upper, green (bit 2) for right, red (bit 4) for the left.
	want := byte(0x01 | 0x04 | 0x10)
	if got[1][1] != want {
		t.Fatalf("lights byte = %#02x, want %#02x", got[1][1], want)
	}

	// The same value again must not touch the USB.
	if d.Sync() {
		t.Fatal("an unchanged value must not be written again")
	}

	// Clearing the bit turns the lights off.
	bios.mem[testAddr] = 0
	if !d.Sync() {
		t.Fatal("clearing the value should write the lights off")
	}
	last := panels.writes["pz55-1"][len(panels.writes["pz55-1"])-1]
	if last[1] != 0 {
		t.Fatalf("cleared lights byte = %#02x, want 0", last[1])
	}
}

// TestSyncPZ70Lights checks the autopilot button lights go out in the lights byte.
func TestSyncPZ70Lights(t *testing.T) {
	profile := mapping.Profile{
		Aircraft: "TestJet",
		Outputs: []mapping.OutputBinding{
			{Model: panel.PZ70, Target: panel.TargetAP, Command: "AP_MASTER_VERT", Color: "green"},
			{Model: panel.PZ70, Target: panel.TargetALT, Command: "AP_ALT_VERT", Color: "green"},
		},
	}
	bios := &fakeBios{mem: map[uint16]byte{testAddr: 1}, state: dcsbios.State{Aircraft: "TestJet", Connected: true}}
	panels := &fakePanels{
		devices: []panelservice.DeviceInfo{{Path: "pz70-1", VendorID: panel.VendorID, ProductID: panel.ProductPZ70}},
		writes:  map[string][][]byte{},
	}
	store := mapping.NewStore(filepath.Join(t.TempDir(), "mappings.json"), func(string) error { return nil })
	if err := store.SetProfile(profile); err != nil {
		t.Fatal(err)
	}
	d := New(bios, panels, store, catalogFor(t, "TestJet", []string{"AP_MASTER_VERT", "AP_ALT_VERT"}, testAddr, testMask))
	store.SetOutputsEnabled(true)

	if !d.Sync() {
		t.Fatal("Sync should have written the PZ70 panel")
	}
	report := panels.writes["pz70-1"][0]
	if len(report) != 13 {
		t.Fatalf("PZ70 report = %d bytes, want 13", len(report))
	}
	if report[11]&byte(panel.PZ70LightAP) == 0 || report[11]&byte(panel.PZ70LightALT) == 0 {
		t.Errorf("AP and ALT lights should be set, lights byte = %#02x", report[11])
	}
}

// TestSyncWithoutAircraftOrPanels checks the driver is a no-op when there is
// nothing to drive, rather than panicking.
func TestSyncWithoutAircraftOrPanels(t *testing.T) {
	bios := &fakeBios{mem: map[uint16]byte{}, state: dcsbios.State{}}
	panels := &fakePanels{writes: map[string][][]byte{}}
	store := mapping.NewStore(filepath.Join(t.TempDir(), "mappings.json"), func(string) error { return nil })
	if err := store.SetProfile(gearProfile()); err != nil {
		t.Fatal(err)
	}
	d := New(bios, panels, store, catalogFor(t, "TestJet", []string{"LIGHT_GEAR_N", "LIGHT_GEAR_L", "LIGHT_GEAR_R"}, testAddr, testMask))
	store.SetOutputsEnabled(true)

	if d.Sync() {
		t.Fatal("no aircraft: Sync should be a no-op")
	}
	bios.state.Aircraft = "TestJet"
	if d.Sync() {
		t.Fatal("no panels: Sync should be a no-op")
	}
}
