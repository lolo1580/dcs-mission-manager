package content

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dcsmanager/internal/dcsdata"
)

// QuarantineItem is one element waiting in the trash. Restoring uses its ID.
type QuarantineItem struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Category      string `json:"category,omitempty"`
	Aircraft      string `json:"aircraft,omitempty"`
	RelPath       string `json:"relPath,omitempty"`
	OriginalState string `json:"originalState,omitempty"`
	CreatedAt     string `json:"createdAt"`
	SizeBytes     int64  `json:"sizeBytes"`
	Path          string `json:"path"`
}

// Quarantine lists both mods and liveries kept in the two quarantine roots.
func (s *Service) Quarantine() ([]QuarantineItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	roots := []string{
		dcsdata.ModsQuarantineDir(s.savedGames),
		dcsdata.LiveriesQuarantineDir(s.savedGames),
	}
	var out []QuarantineItem
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dir := filepath.Join(root, entry.Name())
			item, err := readQuarantine(dir)
			if err != nil {
				continue
			}
			item.ID = entry.Name()
			item.Path = dir
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

func readQuarantine(dir string) (QuarantineItem, error) {
	manifest, err := loadQuarantineManifest(dir)
	if err != nil {
		return QuarantineItem{}, err
	}
	payload := filepath.Join(dir, "payload")
	size, _ := folderSize(payload)
	return QuarantineItem{
		Kind:          manifest.Kind,
		Name:          manifest.Name,
		Category:      manifest.Category,
		Aircraft:      manifest.Aircraft,
		RelPath:       manifest.RelPath,
		OriginalState: manifest.OriginalState,
		CreatedAt:     manifest.CreatedAt,
		SizeBytes:     size,
	}, nil
}

func loadQuarantineManifest(dir string) (quarantineManifest, error) {
	var m quarantineManifest
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	if m.Kind != "mod" && m.Kind != "livery" {
		return m, errors.New("type de quarantaine inconnu")
	}
	return m, nil
}

func writeQuarantineManifest(dir string, m quarantineManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644)
}

// quarantineMod moves a mod folder into Mods.quarantine/<id>/payload and writes
// its manifest. It returns the quarantine ID.
func (s *Service) quarantineMod(category, name, originalState, src string) (string, error) {
	root := dcsdata.ModsQuarantineDir(s.savedGames)
	id := time.Now().Format("20060102-150405") + "-" + newID()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(filepath.Join(dir, "payload"), 0o755); err != nil {
		return "", err
	}
	manifest := quarantineManifest{
		Version:       1,
		Kind:          "mod",
		Category:      category,
		Name:          name,
		OriginalState: originalState,
		CreatedAt:     time.Now().Format(time.RFC3339),
	}
	if err := writeQuarantineManifest(dir, manifest); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	dest := filepath.Join(dir, "payload", category, name)
	if err := moveDir(src, dest); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return id, nil
}

// quarantineLivery moves a user livery folder into Liveries.quarantine.
func (s *Service) quarantineLivery(ref dcsdata.LiveryRef, livery dcsdata.Livery, src string) (string, error) {
	root := dcsdata.LiveriesQuarantineDir(s.savedGames)
	id := time.Now().Format("20060102-150405") + "-" + newID()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(filepath.Join(dir, "payload"), 0o755); err != nil {
		return "", err
	}
	manifest := quarantineManifest{
		Version:   1,
		Kind:      "livery",
		Name:      livery.Name,
		Aircraft:  livery.Aircraft,
		RelPath:   ref.RelPath,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if err := writeQuarantineManifest(dir, manifest); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	dest, err := safeJoin(filepath.Join(dir, "payload"), ref.RelPath)
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	if err := moveDir(src, dest); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return id, nil
}

// restoreQuarantine moves one quarantined payload back to its original place.
func (s *Service) restoreQuarantine(id string) (QuarantineItem, error) {
	for _, root := range []string{
		dcsdata.ModsQuarantineDir(s.savedGames),
		dcsdata.LiveriesQuarantineDir(s.savedGames),
	} {
		dir, err := safeJoin(root, id)
		if err != nil {
			continue
		}
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			continue
		}
		item, err := readQuarantine(dir)
		if err != nil {
			return QuarantineItem{}, err
		}
		payload := filepath.Join(dir, "payload")
		var target string
		switch item.Kind {
		case "mod":
			if err := safeElement(item.Category); err != nil {
				return QuarantineItem{}, err
			}
			if err := safeElement(item.Name); err != nil {
				return QuarantineItem{}, err
			}
			root := dcsdata.ModsDir(s.savedGames)
			if item.OriginalState == "disabled" {
				root = dcsdata.DisabledModsDir(s.savedGames)
			}
			target = filepath.Join(root, item.Category, item.Name)
			if err := moveDir(filepath.Join(payload, item.Category, item.Name), target); err != nil {
				return QuarantineItem{}, err
			}
		case "livery":
			if strings.TrimSpace(item.RelPath) == "" {
				return QuarantineItem{}, errors.New("chemin de livrée vide")
			}
			root := dcsdata.LiveriesDir(s.savedGames)
			target, err = safeJoin(root, item.RelPath)
			if err != nil {
				return QuarantineItem{}, err
			}
			if err := moveDir(filepath.Join(payload, filepath.FromSlash(item.RelPath)), target); err != nil {
				return QuarantineItem{}, err
			}
		default:
			return QuarantineItem{}, errors.New("type de quarantaine inconnu")
		}
		_ = os.RemoveAll(dir)
		item.ID = id
		return item, nil
	}
	return QuarantineItem{}, errors.New("élément de quarantaine introuvable")
}

func folderSize(dir string) (int64, int) {
	var bytes int64
	var files int
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			bytes += info.Size()
			files++
		}
		return nil
	})
	return bytes, files
}
