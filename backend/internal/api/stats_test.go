package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dcsmanager/internal/category"
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
		"/api/stats/overview": s.handleStatsOverview,
		"/api/stats/pilots":   s.handleStatsPilots,
		"/api/stats/weapons":  s.handleStatsWeapons,
		"/api/stats/engines":  s.handleStatsEngines,
		"/api/stats/network":  s.handleStatsNetwork,
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
