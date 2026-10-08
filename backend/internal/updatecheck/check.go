// Package updatecheck compares the installed version with published GitHub installers.
package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	releasesAPI = "https://api.github.com/repos/lolo1580/dcs-mission-manager/releases?per_page=100"
	releaseBase = "https://github.com/lolo1580/dcs-mission-manager/releases"
)

type Result struct {
	Status         string `json:"status"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion,omitempty"`
	DownloadURL    string `json:"downloadUrl,omitempty"`
	ReleaseURL     string `json:"releaseUrl,omitempty"`
	CheckedAt      string `json:"checkedAt,omitempty"`
	Error          string `json:"error,omitempty"`
}

type release struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type Checker struct {
	client   *http.Client
	endpoint string
	mu       sync.Mutex
	cached   Result
	expires  time.Time
}

func New() *Checker {
	return &Checker{client: &http.Client{Timeout: 6 * time.Second}, endpoint: releasesAPI}
}

// Check caches successful checks for six hours and failures for fifteen minutes.
func (c *Checker) Check(ctx context.Context, current string, refresh bool) Result {
	result := Result{CurrentVersion: current}
	installed, ok := parseVersion(current)
	if !ok {
		result.Status = "development"
		return result
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !refresh && c.cached.CurrentVersion == current && time.Now().Before(c.expires) {
		return c.cached
	}
	result.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	releases, err := c.fetch(ctx)
	if err != nil {
		result.Status = "error"
		result.Error = "Impossible de vérifier les releases GitHub."
		if ctx.Err() != nil {
			return result
		}
		c.expires = time.Now().Add(15 * time.Minute)
		c.cached = result
		return result
	}
	var latest version
	found := false
	for _, release := range releases {
		if release.Draft || (release.Prerelease && installed.pre == "") {
			continue
		}
		candidate, valid := parseVersion(release.TagName)
		if !valid || (release.Prerelease != (candidate.pre != "")) {
			continue
		}
		name := "DCSManager-Setup-" + strings.TrimPrefix(release.TagName, "v") + ".exe"
		for _, asset := range release.Assets {
			if asset.Name != name || !strings.HasPrefix(asset.BrowserDownloadURL, releaseBase+"/download/") {
				continue
			}
			if !found || compare(candidate, latest) > 0 {
				latest, found = candidate, true
				result.LatestVersion = strings.TrimPrefix(release.TagName, "v")
				result.DownloadURL = asset.BrowserDownloadURL
				result.ReleaseURL = releaseBase + "/tag/" + url.PathEscape(release.TagName)
			}
		}
	}
	switch {
	case !found:
		result.Status = "unavailable"
	case compare(latest, installed) > 0:
		result.Status = "available"
	default:
		result.Status = "current"
	}
	c.cached = result
	c.expires = time.Now().Add(6 * time.Hour)
	return result
}

func (c *Checker) fetch(ctx context.Context) ([]release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "DCS-Manager-Update-Checker")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub releases: HTTP %d", resp.StatusCode)
	}
	var releases []release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&releases); err != nil {
		return nil, err
	}
	return releases, nil
}

type version struct {
	parts [3]int
	pre   string
}

func parseVersion(raw string) (version, bool) {
	var v version
	raw = strings.TrimPrefix(raw, "v")
	raw = strings.SplitN(raw, "+", 2)[0]
	parts := strings.SplitN(raw, "-", 2)
	if len(parts) == 2 {
		v.pre = parts[1]
		if v.pre == "" {
			return v, false
		}
	}
	numbers := strings.Split(parts[0], ".")
	if len(numbers) != 3 {
		return v, false
	}
	for i, number := range numbers {
		if number == "" || (len(number) > 1 && number[0] == '0') {
			return v, false
		}
		n, err := strconv.Atoi(number)
		if err != nil || n < 0 {
			return v, false
		}
		v.parts[i] = n
	}
	return v, true
}

func compare(a, b version) int {
	for i := range a.parts {
		if a.parts[i] < b.parts[i] {
			return -1
		}
		if a.parts[i] > b.parts[i] {
			return 1
		}
	}
	if a.pre == b.pre {
		return 0
	}
	if a.pre == "" {
		return 1
	}
	if b.pre == "" {
		return -1
	}
	aIDs, bIDs := strings.Split(a.pre, "."), strings.Split(b.pre, ".")
	for i := 0; i < len(aIDs) && i < len(bIDs); i++ {
		an, ae := strconv.Atoi(aIDs[i])
		bn, be := strconv.Atoi(bIDs[i])
		switch {
		case ae == nil && be == nil:
			if an < bn {
				return -1
			}
			if an > bn {
				return 1
			}
		case ae == nil:
			return -1
		case be == nil:
			return 1
		default:
			if aIDs[i] < bIDs[i] {
				return -1
			}
			if aIDs[i] > bIDs[i] {
				return 1
			}
		}
	}
	if len(aIDs) < len(bIDs) {
		return -1
	}
	if len(aIDs) > len(bIDs) {
		return 1
	}
	return 0
}
