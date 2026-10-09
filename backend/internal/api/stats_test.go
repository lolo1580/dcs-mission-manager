package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"dcsmanager/internal/category"
	"dcsmanager/internal/db"
	"dcsmanager/internal/stats"
)

// TestStatsEndpointsDegradeWithoutDatabase guards a real panic: stats.New
// returns a non-nil service even when the database is disabled
// (DCSMANAGER_DB_ENABLED=false), so a handler that only checked `stats == nil`
// dereferenced a nil *db.DB. Every stats route must instead answer
// {"enabled": false} with a 200.
func TestStatsEndpointsDegradeWithoutDatabase(t *testing.T) {
	s := &Server{
		hub:   newHub(),
		stats: stats.New(nil, category.New("")),
		db:    nil,
	}

	routes := map[string]http.HandlerFunc{
		"/api/stats/overview":  s.handleStatsOverview,
		"/api/stats/pilots":    s.handleStatsPilots,
		"/api/stats/weapons":   s.handleStatsWeapons,
		"/api/stats/engines":   s.handleStatsEngines,
		"/api/stats/network":   s.handleStatsNetwork,
		"/api/stats/missions":  s.handleStatsMissions,
		"/api/stats/trend":     s.handleStatsTrend,
		"/api/career/insights": s.handleCareerInsights,
	}
	for path, h := range routes {
		rec := httptest.NewRecorder()
		// The handler must not panic; httptest would surface a panic as a test
		// failure via the deferred recover in the server, but calling directly
		// here means a panic aborts the test, which is the point.
		h(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d, want 200", path, rec.Code)
		}
		var body struct {
			Enabled *bool `json:"enabled"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if body.Enabled == nil || *body.Enabled {
			t.Errorf("%s: enabled = %v, want false", path, body.Enabled)
		}
	}
}

func TestMissionScopeDefaultsToLatestLiveMission(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	live, err := database.StartMission("Real flight", "Caucasus", db.SourceLive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.StartMission("Fixture", "Caucasus", db.SourceTest); err != nil {
		t.Fatal(err)
	}
	s := &Server{db: database, stats: stats.New(database, category.New(""))}
	r := httptest.NewRequest(http.MethodGet, "/api/stats/overview?scope=mission", nil)
	if got := s.scopeFromRequest(r).MissionID; got != live {
		t.Fatalf("default mission = %d, want %d", got, live)
	}
	rec := httptest.NewRecorder()
	s.handleStatsMissions(rec, httptest.NewRequest(http.MethodGet, "/api/stats/missions", nil))
	var body struct {
		Missions []struct {
			ID int64 `json:"id"`
		} `json:"missions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Missions) != 1 || body.Missions[0].ID != live {
		t.Fatalf("selectable missions: %+v", body.Missions)
	}
}

func TestStatsScopeReadsDateWindow(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest(http.MethodGet, "/api/stats/overview?scope=career&from=1700000000000&before=1800000000000", nil)
	scope := s.scopeFromRequest(r)
	if scope.FromMs != 1_700_000_000_000 || scope.BeforeMs != 1_800_000_000_000 {
		t.Fatalf("date window = %+v", scope)
	}
}
