package content

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"dcsmanager/internal/dcsdata"
)

// LiveryActionRequest is the body of POST /api/content/liveries/action.
type LiveryActionRequest struct {
	Action       string   `json:"action"`
	ID           string   `json:"id,omitempty"`
	IDs          []string `json:"ids,omitempty"`
	Aircraft     string   `json:"aircraft,omitempty"`
	QuarantineID string   `json:"quarantineId,omitempty"`
	Force        bool     `json:"force,omitempty"`
}

// ActionLivery quarantines, restores, classifies or reveals user liveries.
func (s *Service) ActionLivery(req LiveryActionRequest) (ActionResult, error) {
	if err := s.ensureSavedGames(); err != nil {
		return ActionResult{}, err
	}
	switch req.Action {
	case "quarantine", "reveal":
		if req.ID == "" {
			return ActionResult{}, errors.New("identifiant de livrée manquant")
		}
	case "restore":
		if req.QuarantineID == "" {
			return ActionResult{}, errors.New("identifiant de quarantaine manquant")
		}
	case "classify":
		if len(req.IDs) == 0 && req.ID == "" {
			return ActionResult{}, errors.New("aucune livrée sélectionnée")
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

	switch req.Action {
	case "quarantine":
		return s.quarantineOneLivery(req.ID, req.Force)
	case "restore":
		item, err := s.restoreQuarantine(req.QuarantineID)
		if err != nil {
			return ActionResult{}, err
		}
		s.Invalidate()
		return ActionResult{Action: "restore", ID: item.ID, Path: item.RelPath}, nil
	case "classify":
		ids := req.IDs
		if len(ids) == 0 {
			ids = []string{req.ID}
		}
		return s.classifyLiveries(ids, req.Aircraft)
	case "reveal":
		path, err := s.userLiveryPath(req.ID)
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

func (s *Service) quarantineOneLivery(id string, force bool) (ActionResult, error) {
	ref, err := dcsdata.DecodeLiveryRef(id)
	if err != nil || ref.Source != "user" {
		return ActionResult{}, errors.New("livrée utilisateur introuvable")
	}
	src, err := safeJoin(dcsdata.LiveriesDir(s.savedGames), ref.RelPath)
	if err != nil {
		return ActionResult{}, err
	}
	if info, err := os.Stat(src); err != nil || !info.IsDir() {
		return ActionResult{}, errors.New("livrée introuvable")
	}
	scan, err := s.liveriesLocked()
	if err != nil {
		return ActionResult{}, err
	}
	livery, ok := liveryByID(scan, id)
	if !ok {
		return ActionResult{}, errors.New("livrée introuvable")
	}
	if len(livery.Dependencies) > 0 && !force {
		return ActionResult{}, fmt.Errorf("cette livrée est utilisée par d’autres : %s", strings.Join(livery.Dependencies, ", "))
	}
	qid, err := s.quarantineLivery(ref, livery, src)
	if err != nil {
		return ActionResult{}, err
	}
	s.Invalidate()
	return ActionResult{Action: "quarantine", ID: qid, Path: ref.RelPath}, nil
}

// classifyLiveries moves one or more flat/misplaced liveries into the standard
// Liveries\<Aircraft>\<Livery> layout. Dependencies are checked against the
// planned destinations so a ../ reference does not silently break.
func (s *Service) classifyLiveries(ids []string, requestedAircraft string) (ActionResult, error) {
	scan, err := s.liveriesLocked()
	if err != nil {
		return ActionResult{}, err
	}
	userRoot := dcsdata.LiveriesDir(s.savedGames)
	type move struct {
		id       string
		src      string
		target   string
		aircraft string
		rel      string
	}
	selected := map[string]bool{}
	for _, id := range ids {
		selected[id] = true
	}

	var moves []move
	plannedRel := map[string]string{} // old relative path -> new relative path
	for _, id := range ids {
		ref, err := dcsdata.DecodeLiveryRef(id)
		if err != nil || ref.Source != "user" {
			return ActionResult{}, errors.New("seules les livrées utilisateur peuvent être classées")
		}
		livery, ok := liveryByID(scan, id)
		if !ok {
			return ActionResult{}, errors.New("livrée introuvable")
		}
		aircraft := strings.TrimSpace(requestedAircraft)
		if aircraft == "" {
			aircraft = livery.Aircraft
		}
		if aircraft == "" {
			return ActionResult{}, fmt.Errorf("appareil inconnu pour %s", livery.Name)
		}
		if err := safeElement(aircraft); err != nil {
			return ActionResult{}, fmt.Errorf("appareil invalide: %s", aircraft)
		}
		src, err := safeJoin(userRoot, ref.RelPath)
		if err != nil {
			return ActionResult{}, err
		}
		name := filepath.Base(src)
		target := filepath.Join(userRoot, aircraft, name)
		newRel := path.Join(aircraft, name)
		plannedRel[ref.RelPath] = newRel
		moves = append(moves, move{
			id: id, src: src, target: target, aircraft: aircraft, rel: ref.RelPath,
		})
	}

	// Check dependencies against the planned layout.
	for _, m := range moves {
		livery, _ := liveryByID(scan, m.id)
		for _, dep := range livery.Dependencies {
			if newDep, ok := plannedRel[dep]; ok {
				if path.Dir(newDep) == m.aircraft {
					continue
				}
				return ActionResult{}, fmt.Errorf("%s dépend de %s, qui serait classée dans un autre appareil", livery.Name, dep)
			}
			// A dependency already living under the target aircraft folder is
			// fine; a flat dependency that stays behind is not, because the
			// moved livery would resolve ../<dep> inside the aircraft folder.
			depTarget := filepath.Join(userRoot, m.aircraft, filepath.Base(dep))
			if info, err := os.Stat(depTarget); err == nil && info.IsDir() {
				continue
			}
			return ActionResult{}, fmt.Errorf("%s dépend de %s, qui resterait à plat ; sélectionne aussi cette livrée", livery.Name, dep)
		}
	}

	var result ActionResult
	result.Action = "classify"
	seenTargets := map[string]bool{}
	for _, m := range moves {
		if filepath.Clean(m.src) == filepath.Clean(m.target) {
			continue
		}
		if seenTargets[m.target] {
			return result, fmt.Errorf("deux livrées ciblent le même dossier: %s", m.target)
		}
		seenTargets[m.target] = true
		if _, err := os.Stat(m.target); err == nil {
			return result, fmt.Errorf("la cible existe déjà: %s", m.target)
		}
	}
	for _, m := range moves {
		if filepath.Clean(m.src) == filepath.Clean(m.target) {
			result.Skipped = append(result.Skipped, m.rel)
			continue
		}
		if err := moveDir(m.src, m.target); err != nil {
			return result, err
		}
		result.Moved = append(result.Moved, m.rel)
	}
	s.Invalidate()
	return result, nil
}

func liveryByID(scan dcsdata.LiveryScan, id string) (dcsdata.Livery, bool) {
	for _, livery := range scan.Liveries {
		if livery.ID == id {
			return livery, true
		}
	}
	return dcsdata.Livery{}, false
}

func (s *Service) userLiveryPath(id string) (string, error) {
	ref, err := dcsdata.DecodeLiveryRef(id)
	if err != nil || ref.Source != "user" {
		return "", errors.New("livrée utilisateur introuvable")
	}
	path, err := safeJoin(dcsdata.LiveriesDir(s.savedGames), ref.RelPath)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return "", errors.New("livrée introuvable")
	}
	return path, nil
}

// ThumbnailPath returns a validated preview image inside a livery folder.
func (s *Service) ThumbnailPath(id string) (string, error) {
	livery, ok, err := s.Livery(id)
	if err != nil {
		return "", err
	}
	if !ok || livery.Thumbnail == "" {
		return "", errors.New("aucune miniature")
	}
	base := filepath.Clean(livery.Path)
	full := filepath.Join(base, livery.Thumbnail)
	if full != base && !strings.HasPrefix(full, base+string(os.PathSeparator)) {
		return "", errors.New("miniature hors de la livrée")
	}
	switch strings.ToLower(filepath.Ext(full)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
	default:
		return "", errors.New("format de miniature non pris en charge")
	}
	info, err := os.Stat(full)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("miniature introuvable")
	}
	return full, nil
}
