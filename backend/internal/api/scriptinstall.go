package api

import (
	"dcsmanager/internal/dcsdata"
	"dcsmanager/internal/install"
	"net/http"
	"os"
	"path/filepath"
)

func (s *Server) scriptStatusNow() dcsdata.ScriptStatus {
	if s.cfg.SavedGames == "" {
		return s.scripts
	}
	in := install.New("", s.cfg.SavedGames)
	var states []dcsdata.ScriptState
	for _, result := range in.Status(install.DefaultTargets()) {
		st := dcsdata.ScriptState{DestRel: filepath.ToSlash(result.DestRel), State: result.Action, Note: result.Note}
		// Custom settings are intentionally preserved, not an outdated script.
		if st.DestRel == "Config/dcsmanager.cfg" {
			if info, err := os.Stat(filepath.Join(s.cfg.SavedGames, result.DestRel)); err == nil && info.Mode().IsRegular() {
				st.State = "installed"
				st.Note = "configuration utilisateur conservée"
			}
		}
		states = append(states, st)
	}
	return dcsdata.InspectScripts(s.cfg.SavedGames, states)
}

func (s *Server) handleScriptInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "use POST"})
		return
	}
	s.scriptInstallMu.Lock()
	defer s.scriptInstallMu.Unlock()
	if s.cfg.SavedGames == "" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Dossier Saved Games DCS non détecté."})
		return
	}
	info, err := os.Stat(s.cfg.SavedGames)
	if err != nil || !info.IsDir() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Dossier Saved Games DCS introuvable."})
		return
	}
	check := s.dcsRunning
	if check == nil {
		check = install.DCSRunning
	}
	running, err := check()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Impossible de vérifier si DCS est fermé."})
		return
	}
	if running {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Ferme DCS avant d’installer les scripts, puis relance le jeu."})
		return
	}
	results, err := install.New("", s.cfg.SavedGames).InstallPreservingConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "results": results})
		return
	}
	s.debugf("scripts", "installed Lua scripts in %s", s.cfg.SavedGames)
	writeJSON(w, http.StatusOK, map[string]any{"results": results, "scripts": s.scriptStatusNow(), "restartRequired": true})
}
