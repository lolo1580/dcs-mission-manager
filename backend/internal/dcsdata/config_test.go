package dcsdata

import (
	"os"
	"path/filepath"
	"testing"

	"dcsmanager/internal/lua"
)

// TestLoadConfigOnRealFiles reads the machine's real Config folder when present.
func TestLoadConfigOnRealFiles(t *testing.T) {
	sg := filepath.Join(os.Getenv("USERPROFILE"), "Saved Games", "DCS")
	if _, err := os.Stat(ConfigDir(sg)); err != nil {
		t.Skip("no DCS install on this machine")
	}
	cfg, err := LoadConfig(sg)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	for _, s := range cfg.Sections {
		t.Logf("section %-14s %d setting(s)", s.Name, len(s.Settings))
	}
	t.Logf("plugins: %d, manager settings: %d, language %q", len(cfg.Plugins), len(cfg.Manager), cfg.Language)
}

// TestLoadConfigSynthetic checks the parsing and ordering on a hand-written
// options.lua, pluginsEnabled.lua and lang.cfg.
func TestLoadConfigSynthetic(t *testing.T) {
	sg := t.TempDir()
	dir := filepath.Join(sg, "Config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("options.lua", `
options = {
	["miscellaneous"] = { ["fps"] = 120 },
	["graphics"] = { ["MSAA"] = 4, ["Upscaling"] = "OFF", ["Scaling"] = 0.66 },
	["difficulty"] = { ["immortal"] = false, ["geffect"] = "realistic", ["labels"] = 1 },
}
`)
	write("pluginsEnabled.lua", `pluginsEnabled = { ["Kola"] = false, ["Syria"] = true }`)
	write("lang.cfg", "fr\n")
	write("dcsmanager.cfg", "dcsmanager_host = \"127.0.0.1\"\ndcsmanager_tcp_port = 7779\n")

	cfg, err := LoadConfig(sg)
	if err != nil {
		t.Fatal(err)
	}
	// Sections follow DCS's order: graphics, difficulty, then miscellaneous.
	var names []string
	for _, s := range cfg.Sections {
		names = append(names, s.Name)
	}
	want := []string{"graphics", "difficulty", "miscellaneous"}
	if len(names) != len(want) {
		t.Fatalf("sections = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("sections = %v, want %v", names, want)
		}
	}
	// Values are rendered readably.
	graphics := cfg.Sections[0]
	got := map[string]string{}
	for _, s := range graphics.Settings {
		got[s.Key] = s.Value
	}
	if got["MSAA"] != "4" || got["Upscaling"] != "OFF" || got["Scaling"] != "0.66" {
		t.Errorf("graphics settings wrong: %+v", got)
	}
	if got["Scaling"] == "0.6600000000000001" {
		t.Error("float formatting should be compact")
	}
	// difficulty booleans render on/off.
	diff := map[string]string{}
	for _, s := range cfg.Sections[1].Settings {
		diff[s.Key] = s.Value
	}
	if diff["immortal"] != "off" || diff["geffect"] != "realistic" || diff["labels"] != "1" {
		t.Errorf("difficulty settings wrong: %+v", diff)
	}
	// Plugins, sorted, with their toggle.
	if len(cfg.Plugins) != 2 || cfg.Plugins[0].Name != "Kola" || cfg.Plugins[0].Enabled {
		t.Errorf("plugins wrong: %+v", cfg.Plugins)
	}
	if cfg.Plugins[1].Name != "Syria" || !cfg.Plugins[1].Enabled {
		t.Errorf("plugins wrong: %+v", cfg.Plugins)
	}
	if cfg.Language != "fr" {
		t.Errorf("language = %q", cfg.Language)
	}
	// The manager's own config is flat, not a table.
	mgr := map[string]string{}
	for _, s := range cfg.Manager {
		mgr[s.Key] = s.Value
	}
	if mgr["dcsmanager_tcp_port"] != "7779" {
		t.Errorf("manager config wrong: %+v", mgr)
	}
}

// TestLoadConfigMissingFiles checks an absent Config folder yields an empty
// structure, not an error.
func TestLoadConfigMissingFiles(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Sections) != 0 || len(cfg.Plugins) != 0 || cfg.Language != "" {
		t.Fatalf("expected an empty config, got %+v", cfg)
	}
}

// TestFormatValue checks the rendering helper directly.
func TestFormatValue(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, "—"},
		{true, "on"},
		{false, "off"},
		{"OFF", "OFF"},
		{float64(4), "4"},
		{float64(0.66), "0.66"},
		{[]any{1, 2}, "(2)"},
		{map[string]any{"a": 1}, "(1)"},
	}
	for _, tc := range cases {
		if got := formatValue(tc.in); got != tc.want {
			t.Errorf("formatValue(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestLoadLuaFileMissing confirms the helper reports absent files as (nil, nil).
func TestLoadLuaFileMissing(t *testing.T) {
	root, err := loadLuaFile(filepath.Join(t.TempDir(), "absent.lua"))
	if err != nil || root != nil {
		t.Fatalf("got (%v, %v), want (nil, nil)", root, err)
	}
	// And that it parses a real document.
	dir := t.TempDir()
	path := filepath.Join(dir, "x.lua")
	os.WriteFile(path, []byte("a = 1\n"), 0o644)
	root, err = loadLuaFile(path)
	if err != nil || root["a"] != float64(1) {
		t.Fatalf("parse failed: (%v, %v)", root, err)
	}
	if _, err := lua.Parse([]byte("a = 1")); err != nil {
		t.Fatal(err)
	}
}
