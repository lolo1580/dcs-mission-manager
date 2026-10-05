package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStatsRaw(t *testing.T) {
	var gotPath, gotScope, gotInclude string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotScope = r.URL.Query().Get("scope")
		gotInclude = r.URL.Query().Get("includeTest")
		_, _ = w.Write([]byte(`{"kills":3}`))
	}))
	defer srv.Close()

	c := NewManagerClient(Config{ManagerURL: srv.URL, RequestTimeout: 2 * time.Second})
	body, err := c.StatsRaw(context.Background(), "overview", "career", true)
	if err != nil {
		t.Fatalf("StatsRaw: %v", err)
	}
	if string(body) != `{"kills":3}` {
		t.Errorf("body = %q", body)
	}
	if gotPath != "/api/stats/overview" {
		t.Errorf("path = %q", gotPath)
	}
	if gotScope != "career" {
		t.Errorf("scope = %q", gotScope)
	}
	if gotInclude != "1" {
		t.Errorf("includeTest = %q", gotInclude)
	}
}

func TestStatsRawError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewManagerClient(Config{ManagerURL: srv.URL, RequestTimeout: 2 * time.Second})
	if _, err := c.StatsRaw(context.Background(), "overview", "career", false); err == nil {
		t.Fatal("expected an error on HTTP 500")
	}
}

func TestHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok","service":"dcsmanager","units":12}`))
	}))
	defer srv.Close()

	c := NewManagerClient(Config{ManagerURL: srv.URL, RequestTimeout: 2 * time.Second})
	h, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if h.Service != "dcsmanager" || h.Units != 12 {
		t.Errorf("health = %+v", h)
	}
}

func TestEventsSince(t *testing.T) {
	var gotSince, gotLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSince = r.URL.Query().Get("sinceId")
		gotLimit = r.URL.Query().Get("limit")
		_, _ = w.Write([]byte(`{"count":1,"events":[{"id":8,"event":"kill","args":[1],"realTs":123}],"nextSinceId":8}`))
	}))
	defer srv.Close()

	c := NewManagerClient(Config{ManagerURL: srv.URL, RequestTimeout: 2 * time.Second})
	events, next, err := c.EventsSince(context.Background(), 7, 500)
	if err != nil {
		t.Fatalf("EventsSince: %v", err)
	}
	if gotSince != "7" || gotLimit != "500" {
		t.Errorf("query = sinceId=%q limit=%q", gotSince, gotLimit)
	}
	if len(events) != 1 || events[0].Event != "kill" || events[0].ID != 8 {
		t.Errorf("events = %+v", events)
	}
	if next != 8 {
		t.Errorf("next = %d, want 8", next)
	}
}

// TestEventsSinceDerivesCursor covers a manager that does not report nextSinceId
// (the pre-P3 shape): the plugin must fall back to the last id in the batch.
func TestEventsSinceDerivesCursor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"count":2,"events":[{"id":3,"event":"a"},{"id":9,"event":"b"}]}`))
	}))
	defer srv.Close()

	c := NewManagerClient(Config{ManagerURL: srv.URL, RequestTimeout: 2 * time.Second})
	_, next, err := c.EventsSince(context.Background(), 0, 500)
	if err != nil {
		t.Fatal(err)
	}
	if next != 9 {
		t.Errorf("derived next = %d, want 9", next)
	}
}

func TestMissionsSince(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("sinceId"); got != "4" {
			t.Errorf("sinceId = %q, want 4", got)
		}
		_, _ = w.Write([]byte(`{"count":1,"missions":[{"id":5,"name":"M","startedAt":1}],"nextSinceId":5}`))
	}))
	defer srv.Close()

	c := NewManagerClient(Config{ManagerURL: srv.URL, RequestTimeout: 2 * time.Second})
	missions, next, err := c.MissionsSince(context.Background(), 4, 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(missions) != 1 || missions[0].ID != 5 || missions[0].Name != "M" {
		t.Errorf("missions = %+v", missions)
	}
	if next != 5 {
		t.Errorf("next = %d, want 5", next)
	}
}

func TestRecentMissions(t *testing.T) {
	var gotSince string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSince = r.URL.Query().Get("sinceId")
		_, _ = w.Write([]byte(`{"count":1,"missions":[{"id":9,"name":"ended","endedAt":42,"winner":"red"}]}`))
	}))
	defer srv.Close()

	c := NewManagerClient(Config{ManagerURL: srv.URL, RequestTimeout: 2 * time.Second})
	missions, err := c.RecentMissions(context.Background(), 50)
	if err != nil {
		t.Fatal(err)
	}
	if gotSince != "" {
		t.Errorf("RecentMissions must not send sinceId, got %q", gotSince)
	}
	if len(missions) != 1 || missions[0].EndedAt != 42 || missions[0].Winner != "red" {
		t.Errorf("missions = %+v", missions)
	}
}
