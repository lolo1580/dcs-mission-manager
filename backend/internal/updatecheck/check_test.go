package updatecheck

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestVersionOrdering(t *testing.T) {
	cases := []struct{ older, newer string }{
		{"1.0.0-beta.3", "1.0.0-beta.4"},
		{"1.0.0-beta.9", "1.0.0-beta.10"},
		{"1.0.0-rc.2", "1.0.0"},
		{"1.0.0", "1.0.1"},
		{"1.0.20261008", "1.0.20261009"},
	}
	for _, tc := range cases {
		a, okA := parseVersion(tc.older)
		b, okB := parseVersion(tc.newer)
		if !okA || !okB || compare(a, b) >= 0 {
			t.Fatalf("expected %s before %s", tc.older, tc.newer)
		}
	}
}

func TestCheckRequiresInstallerAndRespectsReleaseChannel(t *testing.T) {
	var calls atomic.Int32
	transport := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)
		body := `[
		{"tag_name":"v1.1.0-beta.1","prerelease":true,"assets":[{"name":"DCSManager-Setup-1.1.0-beta.1.exe","browser_download_url":"https://github.com/lolo1580/dcs-mission-manager/releases/download/v1.1.0-beta.1/DCSManager-Setup-1.1.0-beta.1.exe"}]},
		{"tag_name":"v1.0.1","assets":[{"name":"dcsmanager.exe","browser_download_url":"https://github.com/lolo1580/dcs-mission-manager/releases/download/v1.0.1/dcsmanager.exe"}]},
		{"tag_name":"v1.0.0","assets":[{"name":"DCSManager-Setup-1.0.0.exe","browser_download_url":"https://github.com/lolo1580/dcs-mission-manager/releases/download/v1.0.0/DCSManager-Setup-1.0.0.exe"}]}
		]`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	checker := New()
	checker.client.Transport = transport
	if got := checker.Check(context.Background(), "1.0.0", false); got.Status != "current" || got.LatestVersion != "1.0.0" {
		t.Fatalf("stable release result: %+v", got)
	}
	if got := checker.Check(context.Background(), "1.0.0-beta.3", false); got.Status != "available" || got.LatestVersion != "1.1.0-beta.1" || got.DownloadURL == "" {
		t.Fatalf("beta release result: %+v", got)
	}
	if got := checker.Check(context.Background(), "1.0.0-beta.3", false); got.Status != "available" {
		t.Fatalf("cached release result: %+v", got)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected two release requests, got %d", calls.Load())
	}
	checker.Check(context.Background(), "1.0.0-beta.3", true)
	if calls.Load() != 3 {
		t.Fatalf("manual refresh did not fetch releases")
	}
}

func TestCheckOfflineAndDevelopment(t *testing.T) {
	checker := New()
	checker.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })
	if got := checker.Check(context.Background(), "dev", false); got.Status != "development" {
		t.Fatalf("development: %+v", got)
	}
	if got := checker.Check(context.Background(), "1.0.0", false); got.Status != "error" {
		t.Fatalf("offline: %+v", got)
	}
}
