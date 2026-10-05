package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// ManagerClient talks to the manager's REST API. It is read-only: the plugin
// never writes to the manager.
type ManagerClient struct {
	base   string
	client *http.Client
}

// NewManagerClient builds a client for the configured manager URL.
func NewManagerClient(cfg Config) *ManagerClient {
	return &ManagerClient{
		base:   cfg.ManagerURL,
		client: &http.Client{Timeout: cfg.RequestTimeout},
	}
}

// Health mirrors the manager's /api/health payload.
type Health struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Units   int    `json:"units"`
}

// Health pings the manager.
func (c *ManagerClient) Health(ctx context.Context) (Health, error) {
	var h Health
	err := c.getJSON(ctx, "/api/health", nil, &h)
	return h, err
}

// Event is the manager's persisted event shape (internal/model.Event).
type Event struct {
	ID     int64           `json:"id"`
	Event  string          `json:"event"`
	Args   []any           `json:"args,omitempty"`
	Detail json.RawMessage `json:"detail,omitempty"`
	T      float64         `json:"t,omitempty"`
	RealTS int64           `json:"realTs"`
}

// Chat is the manager's chat shape (internal/model.Chat).
type Chat struct {
	ID      int64  `json:"id"`
	From    string `json:"from"`
	Message string `json:"message"`
	RealTS  int64  `json:"realTs"`
}

// Mission is the manager's mission shape (internal/model.Mission).
type Mission struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Theatre   string `json:"theatre,omitempty"`
	Source    string `json:"source,omitempty"`
	StartedAt int64  `json:"startedAt"`
	EndedAt   int64  `json:"endedAt,omitempty"`
	Winner    string `json:"winner,omitempty"`
}

type eventsFeed struct {
	Count       int     `json:"count"`
	Events      []Event `json:"events"`
	NextSinceID int64   `json:"nextSinceId"`
}

type chatFeed struct {
	Count       int    `json:"count"`
	Chat        []Chat `json:"chat"`
	NextSinceID int64  `json:"nextSinceId"`
}

type missionsFeed struct {
	Count       int       `json:"count"`
	Missions    []Mission `json:"missions"`
	NextSinceID int64     `json:"nextSinceId"`
}

// EventsSince fetches events with an id strictly greater than sinceID, oldest
// first, together with the next cursor.
func (c *ManagerClient) EventsSince(ctx context.Context, sinceID int64, limit int) ([]Event, int64, error) {
	q := url.Values{}
	q.Set("sinceId", strconv.FormatInt(sinceID, 10))
	q.Set("limit", strconv.Itoa(limit))

	var feed eventsFeed
	if err := c.getJSON(ctx, "/api/history/events", q, &feed); err != nil {
		return nil, 0, err
	}
	next := feed.NextSinceID
	if next == 0 && len(feed.Events) > 0 {
		next = feed.Events[len(feed.Events)-1].ID
	}
	return feed.Events, next, nil
}

// ChatSince fetches chat with an id strictly greater than sinceID.
func (c *ManagerClient) ChatSince(ctx context.Context, sinceID int64, limit int) ([]Chat, int64, error) {
	q := url.Values{}
	q.Set("sinceId", strconv.FormatInt(sinceID, 10))
	q.Set("limit", strconv.Itoa(limit))

	var feed chatFeed
	if err := c.getJSON(ctx, "/api/history/chat", q, &feed); err != nil {
		return nil, 0, err
	}
	next := feed.NextSinceID
	if next == 0 && len(feed.Chat) > 0 {
		next = feed.Chat[len(feed.Chat)-1].ID
	}
	return feed.Chat, next, nil
}

// MissionsSince fetches missions with an id strictly greater than sinceID.
func (c *ManagerClient) MissionsSince(ctx context.Context, sinceID int64, limit int) ([]Mission, int64, error) {
	q := url.Values{}
	q.Set("sinceId", strconv.FormatInt(sinceID, 10))
	q.Set("limit", strconv.Itoa(limit))

	var feed missionsFeed
	if err := c.getJSON(ctx, "/api/history/missions", q, &feed); err != nil {
		return nil, 0, err
	}
	next := feed.NextSinceID
	if next == 0 && len(feed.Missions) > 0 {
		next = feed.Missions[len(feed.Missions)-1].ID
	}
	return feed.Missions, next, nil
}

// RecentMissions fetches the most recent missions (newest first), without a
// cursor. The incremental feed only ever sees NEW missions; a mission that ends
// keeps its id but gains ended_at/winner, so the recent missions must be
// refreshed separately to pick those updates up.
func (c *ManagerClient) RecentMissions(ctx context.Context, limit int) ([]Mission, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))

	var feed missionsFeed
	if err := c.getJSON(ctx, "/api/history/missions", q, &feed); err != nil {
		return nil, err
	}
	return feed.Missions, nil
}

// StatsRaw returns the raw JSON body of one /api/stats/<kind> endpoint. The body
// is stored as-is in Postgres, so the plugin never has to re-implement the
// manager's aggregation logic.
func (c *ManagerClient) StatsRaw(ctx context.Context, kind, scope string, includeTest bool) ([]byte, error) {
	q := url.Values{}
	q.Set("scope", scope)
	if includeTest {
		q.Set("includeTest", "1")
	}
	return c.getRaw(ctx, "/api/stats/"+kind, q)
}

func (c *ManagerClient) getRaw(ctx context.Context, path string, q url.Values) ([]byte, error) {
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: http %d: %s", path, resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

func (c *ManagerClient) getJSON(ctx context.Context, path string, q url.Values, dst any) error {
	body, err := c.getRaw(ctx, path, q)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dst)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
