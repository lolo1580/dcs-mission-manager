// Package syncer uploads finished missions from a desktop to a DCS Manager
// server. It is entirely optional: the desktop reads its settings from
// `settings.json` and never contacts a server unless one is configured.
package syncer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dcsmanager/internal/checklist"
	"dcsmanager/internal/config"
	"dcsmanager/internal/db"
	"dcsmanager/internal/idgen"
)

// Settings is the desktop-side sync configuration, stored beside the database.
type Settings struct {
	ServerURL  string `json:"serverUrl"`
	HasToken   bool   `json:"hasToken"`
	ClientID   string `json:"clientId"`
	LastSync   int64  `json:"lastSync"`
	LastResult string `json:"lastResult"`
	SyncCursor int64  `json:"syncCursor"`
}

// Load reads the sync part of settings.json. An empty ClientID is generated and
// written back on the next save; callers that need one immediately should use
// EnsureSettings.
func Load(root string) (Settings, error) {
	user, err := config.ReadUserSettings(root)
	if err != nil {
		return Settings{}, err
	}
	return Settings{
		ServerURL:  user.ServerURL,
		HasToken:   user.SyncToken != "",
		ClientID:   user.ClientID,
		LastSync:   user.LastSync,
		LastResult: user.LastResult,
		SyncCursor: user.SyncCursor,
	}, nil
}

// Save updates the sync fields while preserving the other user paths. An empty
// token keeps the existing one; this is what lets the UI show a placeholder
// instead of the saved secret.
func Save(root, serverURL, token string) (Settings, error) {
	user, err := config.ReadUserSettings(root)
	if err != nil {
		return Settings{}, err
	}
	if serverURL = strings.TrimSpace(serverURL); serverURL != "" {
		normalized, err := NormalizeURL(serverURL)
		if err != nil {
			return Settings{}, err
		}
		serverURL = normalized
	} else {
		serverURL = user.ServerURL
	}
	if serverURL != user.ServerURL {
		// A cursor and a token belong to one server; switching servers must pull
		// from zero and must not send the previous server's token to the new one.
		user.SyncCursor = 0
		if strings.TrimSpace(token) == "" {
			user.SyncToken = ""
		}
	}
	user.ServerURL = serverURL
	if token = strings.TrimSpace(token); token != "" {
		user.SyncToken = token
	}
	if user.ClientID == "" {
		user.ClientID = idgen.New("desk_")
	}
	if err := config.WriteUserSettings(root, user); err != nil {
		return Settings{}, err
	}
	return Settings{
		ServerURL:  user.ServerURL,
		HasToken:   user.SyncToken != "",
		ClientID:   user.ClientID,
		LastSync:   user.LastSync,
		LastResult: user.LastResult,
		SyncCursor: user.SyncCursor,
	}, nil
}

// RecordResult stores the outcome of an upload and sets the last-sync time to
// now. It is kept for callers that do not track the start time.
func RecordResult(root string, imported int, detail string) error {
	return RecordSync(root, imported, detail, 0)
}

// RecordSync stores a successful sync. lastSync should be the time captured
// before the exchange, so a local edit made during the sync is still selected
// by the next push.
func RecordSync(root string, imported int, detail string, lastSync int64) error {
	user, err := config.ReadUserSettings(root)
	if err != nil {
		return err
	}
	if lastSync == 0 {
		lastSync = time.Now().UnixMilli()
	}
	user.LastSync = lastSync
	if detail == "" {
		detail = fmt.Sprintf("%d mission(s)", imported)
	}
	user.LastResult = detail
	return config.WriteUserSettings(root, user)
}

// RecordError records a failed attempt without advancing the last-sync time.
func RecordError(root, detail string) error {
	user, err := config.ReadUserSettings(root)
	if err != nil {
		return err
	}
	user.LastResult = detail
	return config.WriteUserSettings(root, user)
}

// EnsureSettings returns the settings with a client identifier, creating and
// saving one when the file predates this feature.
func EnsureSettings(root string) (config.UserSettings, Settings, error) {
	user, err := config.ReadUserSettings(root)
	if err != nil {
		return user, Settings{}, err
	}
	if user.ClientID == "" {
		user.ClientID = idgen.New("desk_")
		if err := config.WriteUserSettings(root, user); err != nil {
			return user, Settings{}, err
		}
	}
	return user, Settings{
		ServerURL:  user.ServerURL,
		HasToken:   user.SyncToken != "",
		ClientID:   user.ClientID,
		LastSync:   user.LastSync,
		LastResult: user.LastResult,
		SyncCursor: user.SyncCursor,
	}, nil
}

// SaveCursor records the last server revision applied locally.
func SaveCursor(root string, cursor int64) error {
	user, err := config.ReadUserSettings(root)
	if err != nil {
		return err
	}
	user.SyncCursor = cursor
	return config.WriteUserSettings(root, user)
}

// NormalizeURL trims a trailing slash and rejects anything that is not a clean
// HTTP(S) base URL.
func NormalizeURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", errors.New("server URL must look like http://host:port")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("server URL must use http or https")
	}
	if u.User != nil {
		return "", errors.New("server URL must not contain credentials")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}

// Result is what the server reports after an upload.
type Result struct {
	Imported int `json:"imported"`
}

// Health is the server's lightweight identity, used by the "test connection"
// button.
type Health struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Mode    string `json:"mode"`
}

// Client talks to one DCS Manager server.
type Client struct {
	BaseURL  string
	Token    string
	ClientID string
	DB       *db.DB
	HTTP     *http.Client
}

// NewClient builds a client with a bounded timeout.
func NewClient(baseURL, token, clientID string, database *db.DB) *Client {
	return &Client{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		Token:    token,
		ClientID: clientID,
		DB:       database,
		HTTP:     &http.Client{Timeout: 3 * time.Minute},
	}
}

// Ping checks that the URL points at a DCS Manager server.
func (c *Client) Ping(ctx context.Context) (Health, error) {
	if c.BaseURL == "" {
		return Health{}, errors.New("no server URL configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/health", nil)
	if err != nil {
		return Health{}, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Health{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Health{}, fmt.Errorf("server answered %d", resp.StatusCode)
	}
	var health Health
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&health); err != nil {
		return Health{}, err
	}
	if health.Status != "ok" {
		return Health{}, errors.New("the URL does not look like a DCS Manager server")
	}
	return health, nil
}

// Upload sends the recent live missions. It is idempotent on the server.
func (c *Client) Upload(ctx context.Context) (Result, error) {
	if c.BaseURL == "" {
		return Result{}, errors.New("no server URL configured")
	}
	if c.Token == "" {
		return Result{}, errors.New("no personal token configured")
	}
	if c.DB == nil {
		return Result{}, errors.New("persistence is disabled")
	}
	missions, err := c.DB.ExportMissions(200)
	if err != nil {
		return Result{}, err
	}
	payload := struct {
		ClientID string             `json:"clientId"`
		Missions []db.MissionExport `json:"missions"`
	}{ClientID: c.ClientID, Missions: missions}
	body, err := json.Marshal(payload)
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/sync/upload", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var detail struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(data, &detail)
		if detail.Error != "" {
			return Result{}, fmt.Errorf("%s", detail.Error)
		}
		return Result{}, fmt.Errorf("server answered %d", resp.StatusCode)
	}
	var result Result
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return Result{}, err
	}
	return result, nil
}

// DocumentResult reports the documents exchanged by SyncDocuments.
type DocumentResult struct {
	Checklists int `json:"checklists"`
	Conflicts  int `json:"conflicts"`
}

// SyncDocuments pulls server changes, then pushes the local documents marked
// dirty by a save or deletion. The dirty flag is a per-document outbox; remote
// versions are stored clean, so a pull never sends them back. Exporting the
// dirty set before the pull preserves a local version that the server may
// reject as older: the push records it in conflict copies.
func (c *Client) SyncDocuments(ctx context.Context, database *db.DB, cursor int64) (DocumentResult, int64, error) {
	if c.BaseURL == "" {
		return DocumentResult{}, cursor, errors.New("no server URL configured")
	}
	if c.Token == "" {
		return DocumentResult{}, cursor, errors.New("no personal token configured")
	}
	if database == nil {
		return DocumentResult{}, cursor, errors.New("persistence is disabled")
	}

	checklists, err := database.ExportChecklistsForSync(500)
	if err != nil {
		return DocumentResult{}, cursor, err
	}

	pullURL := c.BaseURL + "/api/sync/pull?since=" + strconv.FormatInt(cursor, 10)
	pullReq, err := http.NewRequestWithContext(ctx, http.MethodGet, pullURL, nil)
	if err != nil {
		return DocumentResult{}, cursor, err
	}
	pullReq.Header.Set("Authorization", "Bearer "+c.Token)
	pullResp, err := c.HTTP.Do(pullReq)
	if err != nil {
		return DocumentResult{}, cursor, err
	}
	defer pullResp.Body.Close()
	if pullResp.StatusCode != http.StatusOK {
		return DocumentResult{}, cursor, responseError(pullResp)
	}
	var pulled struct {
		Checklists []checklist.Checklist `json:"checklists"`
		Cursor     int64                 `json:"cursor"`
	}
	if err := json.NewDecoder(io.LimitReader(pullResp.Body, 32<<20)).Decode(&pulled); err != nil {
		return DocumentResult{}, cursor, err
	}
	for _, item := range pulled.Checklists {
		if _, err := database.ApplySyncedChecklist(item); err != nil {
			return DocumentResult{}, cursor, err
		}
	}
	nextCursor := pulled.Cursor

	push := struct {
		ClientID   string                `json:"clientId"`
		Checklists []checklist.Checklist `json:"checklists"`
	}{ClientID: c.ClientID, Checklists: checklists}
	body, err := json.Marshal(push)
	if err != nil {
		return DocumentResult{}, cursor, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/sync/push", bytes.NewReader(body))
	if err != nil {
		return DocumentResult{}, cursor, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return DocumentResult{}, cursor, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return DocumentResult{}, cursor, responseError(resp)
	}
	var pushed struct {
		Checklists []checklist.Checklist `json:"checklists"`
		Conflicts  int                   `json:"conflicts"`
		Cursor     int64                 `json:"cursor"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&pushed); err != nil {
		return DocumentResult{}, cursor, err
	}
	for _, item := range pushed.Checklists {
		if _, err := database.ApplySyncedChecklist(item); err != nil {
			return DocumentResult{}, cursor, err
		}
	}
	if err := database.MarkChecklistsSynced(checklists); err != nil {
		return DocumentResult{}, cursor, err
	}
	if pushed.Cursor > nextCursor {
		nextCursor = pushed.Cursor
	}
	return DocumentResult{
		Checklists: len(pushed.Checklists),
		Conflicts:  pushed.Conflicts,
	}, nextCursor, nil
}

func responseError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var detail struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(data, &detail)
	if detail.Error != "" {
		return fmt.Errorf("%s", detail.Error)
	}
	return fmt.Errorf("server answered %d", resp.StatusCode)
}
