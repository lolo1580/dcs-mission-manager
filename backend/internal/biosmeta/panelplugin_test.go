package biosmeta

import "testing"

func TestPanelPluginCatalogueDoesNotChangeBIOS(t *testing.T) {
	original := newCatalog("FA-18C_hornet", map[string]map[string]Control{"Gear": {"GEAR": {Inputs: []Input{{Interface: "set_state"}}, Outputs: []Output{{Address: 123}}}}})
	augmented := WithPanelPlugin(original)
	trim, ok := augmented.ByID("DCSM_PITCH_TRIM")
	if !ok || !trim.Supports("variable_step") || len(augmented.InCategory("DCS Manager plugin")) != 1 {
		t.Fatal("plugin unavailable to editor")
	}
	if _, ok := original.ByID("DCSM_PITCH_TRIM"); ok {
		t.Fatal("original mutated")
	}
	gear, ok := augmented.ByID("GEAR")
	if !ok || gear.Outputs[0].Address != 123 {
		t.Fatal("LED metadata lost")
	}
	other := newCatalog("F-16C_50", nil)
	if WithPanelPlugin(other) != other {
		t.Fatal("Hornet plugin exposed to another aircraft")
	}
}
