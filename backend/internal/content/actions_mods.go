package content

import (
	"errors"
	"os"
	"path/filepath"

	"dcsmanager/internal/dcsdata"
)

// ModActionRequest is the body of POST /api/content/mods/action.
type ModActionRequest struct {
	Action       string `json:"action"`
	Category     string `json:"category"`
	Name         string `json:"name"`
	QuarantineID string `json:"quarantineId,omitempty"`
}

// ActionResult reports one completed mutation.
type ActionResult struct {
	Action  string   `json:"action"`
	Path    string   `json:"path,omitempty"`
	ID      string   `json:"id,omitempty"`
	Moved   []string `json:"moved,omitempty"`
	Skipped []string `json:"skipped,omitempty"`
}

// ActionMod enables, disables, quarantines, restores or reveals one mod.
func (s *Service) ActionMod(req ModActionRequest) (ActionResult, error) {
	if err := s.ensureSavedGames(); err != nil {
		return ActionResult{}, err
	}
	switch req.Action {
	case "enable", "disable", "quarantine":
		if err := safeElement(req.Category); err != nil {
			return ActionResult{}, err
		}
		if err := safeElement(req.Name); err != nil {
			return ActionResult{}, err
		}
	case "restore":
		if req.QuarantineID == "" {
			return ActionResult{}, errors.New("identifiant de quarantaine manquant")
		}
	case "reveal":
		if err := safeElement(req.Category); err != nil {
			return ActionResult{}, err
		}
		if err := safeElement(req.Name); err != nil {
			return ActionResult{}, err
		}
	default:
		return ActionResult{}, errors.New("action inconnue")
	}

	if req.Action != "reveal" {
		if err := s.requireDCSClosed(); err != nil {
			return ActionResult{}, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.Invalidate()

	switch req.Action {
	case "enable":
		return s.moveMod(req.Category, req.Name, dcsdata.DisabledModsDir(s.savedGames), dcsdata.ModsDir(s.savedGames))
	case "disable":
		return s.moveMod(req.Category, req.Name, dcsdata.ModsDir(s.savedGames), dcsdata.DisabledModsDir(s.savedGames))
	case "quarantine":
		return s.removeMod(req.Category, req.Name)
	case "restore":
		item, err := s.restoreQuarantine(req.QuarantineID)
		if err != nil {
			return ActionResult{}, err
		}
		return ActionResult{Action: "restore", ID: item.ID, Path: item.Name}, nil
	case "reveal":
		path, err := s.modPath(req.Category, req.Name)
		if err != nil {
			return ActionResult{}, err
		}
		if err := revealPath(path); err != nil {
			return ActionResult{}, err
		}
		return ActionResult{Action: "reveal", Path: path}, nil
	}
	return ActionResult{}, errors.New("action inconnue")
}

func (s *Service) moveMod(category, name, fromRoot, toRoot string) (ActionResult, error) {
	src := filepath.Join(fromRoot, category, name)
	dst := filepath.Join(toRoot, category, name)
	if info, err := os.Stat(src); err != nil || !info.IsDir() {
		return ActionResult{}, errors.New("mod introuvable")
	}
	if err := moveDir(src, dst); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Action: "move", Path: dst}, nil
}

func (s *Service) removeMod(category, name string) (ActionResult, error) {
	active := filepath.Join(dcsdata.ModsDir(s.savedGames), category, name)
	disabled := filepath.Join(dcsdata.DisabledModsDir(s.savedGames), category, name)
	src, state := active, "active"
	if info, err := os.Stat(src); err != nil || !info.IsDir() {
		src, state = disabled, "disabled"
	}
	if info, err := os.Stat(src); err != nil || !info.IsDir() {
		return ActionResult{}, errors.New("mod introuvable")
	}
	id, err := s.quarantineMod(category, name, state, src)
	if err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Action: "quarantine", ID: id, Path: name}, nil
}

// modPath resolves an enabled or disabled mod folder for reveal.
func (s *Service) modPath(category, name string) (string, error) {
	for _, root := range []string{dcsdata.ModsDir(s.savedGames), dcsdata.DisabledModsDir(s.savedGames)} {
		path := filepath.Join(root, category, name)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path, nil
		}
	}
	return "", errors.New("mod introuvable")
}
