package syncer

import (
	"strings"
	"testing"

	"dcsmanager/internal/config"
)

func TestNormalizeURL(t *testing.T) {
	good, err := NormalizeURL("http://192.168.1.20:8080/")
	if err != nil || good != "http://192.168.1.20:8080" {
		t.Fatalf("normalize = %q, %v", good, err)
	}
	if _, err := NormalizeURL("ftp://example.com"); err == nil {
		t.Fatal("accepted ftp")
	}
	if _, err := NormalizeURL("https://user:pass@example.com"); err == nil {
		t.Fatal("accepted credentials in URL")
	}
	if _, err := NormalizeURL("example.com"); err == nil {
		t.Fatal("accepted a URL without a scheme")
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	root := t.TempDir()
	user, initial, err := EnsureSettings(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(initial.ClientID, "desk_") || user.ClientID != initial.ClientID {
		t.Fatalf("client id = %q", initial.ClientID)
	}
	if initial.HasToken {
		t.Fatal("unexpected token on a fresh profile")
	}

	saved, err := Save(root, "http://server:8080/", "personal-token")
	if err != nil {
		t.Fatal(err)
	}
	if saved.ServerURL != "http://server:8080" || !saved.HasToken {
		t.Fatalf("saved = %+v", saved)
	}
	if saved.ClientID != initial.ClientID {
		t.Fatalf("client id changed: %q -> %q", initial.ClientID, saved.ClientID)
	}

	// Saving without a token keeps the existing one.
	saved, err = Save(root, "http://server:8080", "")
	if err != nil {
		t.Fatal(err)
	}
	if !saved.HasToken {
		t.Fatal("token was erased")
	}
	user, err = config.ReadUserSettings(root)
	if err != nil || user.SyncToken != "personal-token" {
		t.Fatalf("token = %q, %v", user.SyncToken, err)
	}

	if err := RecordResult(root, 3, ""); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LastSync == 0 || loaded.LastResult != "3 mission(s)" {
		t.Fatalf("loaded = %+v", loaded)
	}
}
