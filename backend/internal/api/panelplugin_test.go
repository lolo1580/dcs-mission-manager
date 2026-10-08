package api

import (
	"dcsmanager/internal/panelplugin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTrimSwitchRequiresExplicitValueAndConnectedAircraft(t *testing.T) {
	client := panelplugin.New()
	defer client.Close()
	s := &Server{panelPlugin: client}
	for _, tc := range []struct {
		body string
		code int
	}{
		{`{}`, 400}, {`{"trimEnabled":null}`, 400}, {`{`, 400},
		{`{"trimEnabled":true}`, 409}, {`{"trimEnabled":false}`, 200},
	} {
		r := httptest.NewRecorder()
		s.handlePanelPlugin(r, httptest.NewRequest(http.MethodPost, "/api/panels/plugin", strings.NewReader(tc.body)))
		if r.Code != tc.code {
			t.Fatalf("%s: %d %s", tc.body, r.Code, r.Body.String())
		}
		if client.State().TrimEnabled {
			t.Fatal("disconnected trim armed")
		}
	}
}
