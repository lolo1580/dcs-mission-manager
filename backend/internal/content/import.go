package content

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dcsmanager/internal/dcsdata"
)

const maxUploadBytes = int64(16) << 30

// ImportWarning is a machine-readable warning shown before or after an import.
type ImportWarning struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// ImportItem is one mod or livery detected inside an archive.
type ImportItem struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"` // mod or livery
	Category  string          `json:"category,omitempty"`
	Aircraft  string          `json:"aircraft,omitempty"`
	Name      string          `json:"name"`
	SourceRel string          `json:"sourceRel"`
	TargetRel string          `json:"targetRel"`
	Structure string          `json:"structure,omitempty"`
	SizeBytes int64           `json:"sizeBytes"`
	Files     int             `json:"files"`
	Conflicts []string        `json:"conflicts,omitempty"`
	Warnings  []ImportWarning `json:"warnings,omitempty"`
}

// ImportPreview is stored beside the uploaded archive until the user commits
// or cancels it.
type ImportPreview struct {
	ID          string          `json:"id"`
	ArchiveName string          `json:"archiveName"`
	Format      string          `json:"format"`
	CreatedAt   string          `json:"createdAt"`
	Items       []ImportItem    `json:"items"`
	Warnings    []ImportWarning `json:"warnings,omitempty"`
}

// ImportSelection is one item the user chose to install, with optional
// overrides for category, aircraft or folder name.
type ImportSelection struct {
	ID       string `json:"id"`
	Category string `json:"category,omitempty"`
	Aircraft string `json:"aircraft,omitempty"`
	Name     string `json:"name,omitempty"`
}

// ImportInstalled is one successfully installed item.
type ImportInstalled struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	TargetRel string `json:"targetRel"`
}

// ImportResult reports what an import changed.
type ImportResult struct {
	Installed   []ImportInstalled `json:"installed"`
	Quarantined []string          `json:"quarantined,omitempty"`
}

// InspectUpload stores an uploaded archive, lists its candidates and returns a
// preview. It does not write to Saved Games.
func (s *Service) InspectUpload(r io.Reader, filename string) (ImportPreview, error) {
	if err := s.ensureSavedGames(); err != nil {
		return ImportPreview{}, err
	}
	_ = s.CleanupImports(2 * time.Hour)
	root := s.importRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return ImportPreview{}, err
	}
	id := time.Now().Format("20060102-150405") + "-" + newID()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ImportPreview{}, err
	}
	upload := filepath.Join(dir, "upload")
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(dir)
		}
	}()

	out, err := os.OpenFile(upload, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return ImportPreview{}, err
	}
	n, copyErr := io.Copy(out, io.LimitReader(r, maxUploadBytes+1))
	closeErr := out.Close()
	if copyErr != nil {
		return ImportPreview{}, copyErr
	}
	if closeErr != nil {
		return ImportPreview{}, closeErr
	}
	if n > maxUploadBytes {
		return ImportPreview{}, errors.New("archive trop volumineuse")
	}

	format, entries, err := listArchive(upload)
	if err != nil {
		return ImportPreview{}, err
	}
	items, warnings := analyzeArchive(upload, entries)
	s.fillConflicts(items)
	preview := ImportPreview{
		ID:          id,
		ArchiveName: filepath.Base(filename),
		Format:      format,
		CreatedAt:   time.Now().Format(time.RFC3339),
		Items:       items,
		Warnings:    warnings,
	}
	if len(items) == 0 {
		return ImportPreview{}, errors.New("aucun mod ni livrée reconnu dans l’archive")
	}
	if err := writePreview(dir, preview); err != nil {
		return ImportPreview{}, err
	}
	cleanup = false
	return preview, nil
}

// CommitImport extracts and installs the selected items.
func (s *Service) CommitImport(id string, selections []ImportSelection, replace bool) (ImportResult, error) {
	if err := s.ensureSavedGames(); err != nil {
		return ImportResult{}, err
	}
	if len(selections) == 0 {
		return ImportResult{}, errors.New("aucun élément sélectionné")
	}
	if err := s.requireDCSClosed(); err != nil {
		return ImportResult{}, err
	}
	dir, err := s.importDir(id)
	if err != nil {
		return ImportResult{}, err
	}
	preview, err := readPreview(dir)
	if err != nil {
		return ImportResult{}, err
	}
	upload := filepath.Join(dir, "upload")
	byID := map[string]ImportItem{}
	for _, item := range preview.Items {
		byID[item.ID] = item
	}

	type planned struct {
		item      ImportItem
		target    string
		targetRel string
	}
	var plans []planned
	conflicts := map[string]bool{}
	seenSelections := map[string]bool{}
	seenTargets := map[string]bool{}
	for _, selection := range selections {
		if seenSelections[selection.ID] {
			continue
		}
		seenSelections[selection.ID] = true
		item, ok := byID[selection.ID]
		if !ok {
			return ImportResult{}, fmt.Errorf("élément d’import inconnu: %s", selection.ID)
		}
		targetRel, err := plannedTarget(item, selection)
		if err != nil {
			return ImportResult{}, err
		}
		if seenTargets[targetRel] {
			return ImportResult{}, fmt.Errorf("%w : deux éléments ciblent %s", ErrConflict, targetRel)
		}
		seenTargets[targetRel] = true
		target, err := safeJoin(s.savedGames, targetRel)
		if err != nil {
			return ImportResult{}, err
		}
		if info, err := os.Stat(target); err == nil && info.IsDir() {
			conflicts[targetRel] = true
		}
		if item.Kind == "mod" {
			parts := strings.Split(targetRel, "/")
			if len(parts) >= 3 {
				disabled := filepath.Join(dcsdata.DisabledModsDir(s.savedGames), parts[1], strings.Join(parts[2:], string(os.PathSeparator)))
				if info, err := os.Stat(disabled); err == nil && info.IsDir() {
					conflicts["Mods.disabled/"+strings.Join(parts[1:], "/")] = true
				}
			}
		}
		plans = append(plans, planned{item: item, target: target, targetRel: targetRel})
	}
	if len(conflicts) > 0 && !replace {
		names := make([]string, 0, len(conflicts))
		for name := range conflicts {
			names = append(names, name)
		}
		sort.Strings(names)
		return ImportResult{}, fmt.Errorf("%w : %s existe déjà", ErrConflict, strings.Join(names, ", "))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.Invalidate()
	var result ImportResult
	for _, plan := range plans {
		// Extract and validate the new content before moving the old target to
		// quarantine: a broken archive must not remove a working install.
		stage := filepath.Join(dir, "stage", plan.item.ID)
		_ = os.RemoveAll(stage)
		var selected []string
		if plan.item.SourceRel != "" {
			selected = []string{plan.item.SourceRel}
		}
		if err := extractArchive(upload, stage, selected); err != nil {
			return result, err
		}
		src := stage
		if plan.item.SourceRel != "" {
			src = filepath.Join(stage, filepath.FromSlash(plan.item.SourceRel))
		}
		if info, err := os.Stat(src); err != nil || !info.IsDir() {
			return result, errors.New("contenu extrait introuvable")
		}

		var quarantined []string
		if info, err := os.Stat(plan.target); err == nil && info.IsDir() {
			qid, err := s.quarantineTarget(plan.target, plan.targetRel)
			if err != nil {
				return result, err
			}
			quarantined = append(quarantined, qid)
			result.Quarantined = append(result.Quarantined, qid)
		}
		if plan.item.Kind == "mod" {
			parts := strings.Split(plan.targetRel, "/")
			if len(parts) >= 3 {
				disabled := filepath.Join(dcsdata.DisabledModsDir(s.savedGames), parts[1], parts[2])
				if info, err := os.Stat(disabled); err == nil && info.IsDir() {
					qid, err := s.quarantineMod(parts[1], parts[2], "disabled", disabled)
					if err != nil {
						return result, err
					}
					quarantined = append(quarantined, qid)
					result.Quarantined = append(result.Quarantined, qid)
				}
			}
		}
		if err := moveDir(src, plan.target); err != nil {
			// Put the previous content back when the final move fails.
			for i := len(quarantined) - 1; i >= 0; i-- {
				_, _ = s.restoreQuarantine(quarantined[i])
			}
			return result, err
		}
		result.Installed = append(result.Installed, ImportInstalled{
			Kind:      plan.item.Kind,
			Name:      path.Base(plan.targetRel),
			TargetRel: plan.targetRel,
		})
	}
	s.Invalidate()
	_ = os.RemoveAll(dir)
	return result, nil
}

func plannedTarget(item ImportItem, selection ImportSelection) (string, error) {
	name := strings.TrimSpace(selection.Name)
	if name == "" {
		name = item.Name
	}
	if err := safeElement(name); err != nil {
		return "", fmt.Errorf("nom invalide: %s", name)
	}
	switch item.Kind {
	case "mod":
		category := strings.TrimSpace(selection.Category)
		if category == "" {
			category = item.Category
		}
		if category == "" {
			return "", errors.New("catégorie de mod requise")
		}
		if err := safeElement(category); err != nil {
			return "", fmt.Errorf("catégorie invalide: %s", category)
		}
		return path.Join("Mods", category, name), nil
	case "livery":
		aircraft := strings.TrimSpace(selection.Aircraft)
		if aircraft == "" {
			aircraft = item.Aircraft
		}
		if aircraft == "" {
			return path.Join("Liveries", name), nil
		}
		if err := safeElement(aircraft); err != nil {
			return "", fmt.Errorf("appareil invalide: %s", aircraft)
		}
		return path.Join("Liveries", aircraft, name), nil
	}
	return "", errors.New("type d’élément inconnu")
}

func (s *Service) quarantineTarget(target, targetRel string) (string, error) {
	if strings.HasPrefix(targetRel, "Mods"+string(os.PathSeparator)) || strings.HasPrefix(targetRel, "Mods/") {
		parts := strings.Split(filepath.ToSlash(targetRel), "/")
		if len(parts) >= 3 {
			state := "active"
			if strings.HasPrefix(filepath.Clean(target), filepath.Clean(dcsdata.DisabledModsDir(s.savedGames))) {
				state = "disabled"
			}
			return s.quarantineMod(parts[1], strings.Join(parts[2:], "/"), state, target)
		}
		return "", errors.New("cible de mod invalide")
	}
	if strings.HasPrefix(filepath.ToSlash(targetRel), "Liveries/") {
		rel := strings.TrimPrefix(filepath.ToSlash(targetRel), "Liveries/")
		ref := dcsdata.LiveryRef{Source: "user", RelPath: rel}
		name := path.Base(rel)
		aircraft := ""
		if dir := path.Dir(rel); dir != "." {
			aircraft = strings.Split(dir, "/")[0]
		}
		return s.quarantineLivery(ref, dcsdata.Livery{Name: name, Aircraft: aircraft}, target)
	}
	return "", errors.New("cible d’import inconnue")
}

func (s *Service) importRoot() string {
	return filepath.Join(s.dataDir, "imports")
}

func (s *Service) importDir(id string) (string, error) {
	if err := safeElement(id); err != nil {
		return "", errors.New("identifiant d’import invalide")
	}
	dir := filepath.Join(s.importRoot(), id)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", errors.New("import introuvable ou expiré")
	}
	return dir, nil
}

// CancelImport removes an inspected archive and its staging.
func (s *Service) CancelImport(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir, err := s.importDir(id)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// CleanupImports removes abandoned uploads older than maxAge.
func (s *Service) CleanupImports(maxAge time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	root := s.importRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(root, entry.Name()))
	}
	return nil
}

func writePreview(dir string, preview ImportPreview) error {
	data, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "preview.json"), data, 0o644)
}

func readPreview(dir string) (ImportPreview, error) {
	var preview ImportPreview
	data, err := os.ReadFile(filepath.Join(dir, "preview.json"))
	if err != nil {
		return preview, err
	}
	if err := json.Unmarshal(data, &preview); err != nil {
		return preview, err
	}
	return preview, nil
}

// fillConflicts annotates preview items with existing targets. It is advisory;
// CommitImport re-checks under the mutation lock.
func (s *Service) fillConflicts(items []ImportItem) {
	for i := range items {
		item := &items[i]
		if item.TargetRel == "" {
			continue
		}
		target, err := safeJoin(s.savedGames, item.TargetRel)
		if err != nil {
			continue
		}
		if info, err := os.Stat(target); err == nil && info.IsDir() {
			item.Conflicts = append(item.Conflicts, item.TargetRel)
		}
		if item.Kind == "mod" {
			parts := strings.Split(item.TargetRel, "/")
			if len(parts) >= 3 {
				disabled := filepath.Join(dcsdata.DisabledModsDir(s.savedGames), parts[1], strings.Join(parts[2:], string(os.PathSeparator)))
				if info, err := os.Stat(disabled); err == nil && info.IsDir() {
					item.Conflicts = append(item.Conflicts, "Mods.disabled/"+strings.Join(parts[1:], "/"))
				}
			}
		}
	}
}

// analyzeArchive classifies the archive members into mod and livery candidates.
func analyzeArchive(upload string, entries []archiveEntry) ([]ImportItem, []ImportWarning) {
	modRoots := modCandidates(entries)
	liveryRoots := liveryCandidates(entries, modRoots)
	wanted := map[string]int64{}
	for _, root := range modRoots {
		wanted[metaPath(root, "entry.lua")] = 512 << 10
	}
	for _, root := range liveryRoots {
		wanted[metaPath(root, "description.lua")] = 512 << 10
	}
	meta, _ := readArchiveMeta(upload, wanted)

	var items []ImportItem
	for i, root := range modRoots {
		item := buildModItem(i, root, entries)
		items = append(items, item)
	}
	for i, root := range liveryRoots {
		item := buildLiveryItem(len(modRoots)+i, root, entries, meta)
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind == "mod"
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return items, nil
}

func modCandidates(entries []archiveEntry) []string {
	roots := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}
		name := strings.ReplaceAll(entry.Name, "\\", "/")
		if !strings.EqualFold(path.Base(name), "entry.lua") {
			continue
		}
		root := path.Dir(name)
		if root == "." {
			root = ""
		}
		if cleaned, err := safeArchiveName(root); err == nil || root == "" {
			if root != "" {
				root = cleaned
			}
			roots[root] = true
		}
	}
	list := make([]string, 0, len(roots))
	for root := range roots {
		list = append(list, root)
	}
	sort.Slice(list, func(i, j int) bool {
		return strings.Count(list[i], "/") < strings.Count(list[j], "/")
	})
	// Keep only the top-most entry.lua root: a mod often ships helper entry.lua
	// files in subfolders, which are not separate mods.
	var top []string
	for _, root := range list {
		nested := false
		for _, parent := range top {
			if parent == "" || root == parent || strings.HasPrefix(root, parent+"/") {
				nested = true
				break
			}
		}
		if !nested {
			top = append(top, root)
		}
	}
	return top
}

func liveryCandidates(entries []archiveEntry, modRoots []string) []string {
	roots := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}
		name := strings.ReplaceAll(entry.Name, "\\", "/")
		if !strings.EqualFold(path.Base(name), "description.lua") {
			continue
		}
		root := path.Dir(name)
		if root == "." {
			root = ""
		}
		underMod := false
		for _, modRoot := range modRoots {
			if modRoot == "" || root == modRoot || strings.HasPrefix(root, modRoot+"/") {
				underMod = true
				break
			}
		}
		if underMod {
			continue
		}
		if root != "" {
			cleaned, err := safeArchiveName(root)
			if err != nil {
				continue
			}
			root = cleaned
		}
		roots[root] = true
	}
	list := make([]string, 0, len(roots))
	for root := range roots {
		list = append(list, root)
	}
	sort.Strings(list)
	return list
}

func metaPath(root, file string) string {
	if root == "" {
		return file
	}
	return path.Join(root, file)
}

func buildModItem(index int, root string, entries []archiveEntry) ImportItem {
	item := ImportItem{
		ID:        fmt.Sprintf("mod-%d", index),
		Kind:      "mod",
		SourceRel: root,
		Name:      path.Base(root),
	}
	if root == "" {
		item.Name = "mod"
	}
	parts := strings.Split(root, "/")
	category := ""
	switch {
	case len(parts) >= 3 && strings.EqualFold(parts[0], "Mods"):
		category = parts[1]
		item.Name = parts[2]
	case len(parts) >= 2 && knownModCategory(parts[0]):
		category = parts[0]
		item.Name = parts[1]
	case len(parts) >= 2 && knownModCategory(parts[len(parts)-2]):
		category = parts[len(parts)-2]
		item.Name = parts[len(parts)-1]
	}
	item.Category = category
	if category == "" {
		item.Warnings = append(item.Warnings, ImportWarning{Code: "category_required"})
	} else {
		item.TargetRel = path.Join("Mods", category, item.Name)
	}
	item.SizeBytes, item.Files = rootSize(entries, root)
	return item
}

func buildLiveryItem(index int, root string, entries []archiveEntry, meta map[string][]byte) ImportItem {
	item := ImportItem{
		ID:        fmt.Sprintf("livery-%d", index),
		Kind:      "livery",
		SourceRel: root,
		Name:      path.Base(root),
	}
	if root == "" {
		item.Name = "livery"
	}
	parts := strings.Split(root, "/")
	structure := "nonstandard"
	aircraft := ""
	for i := len(parts) - 1; i >= 0; i-- {
		if !strings.EqualFold(parts[i], "Liveries") {
			continue
		}
		rest := parts[i+1:]
		if len(rest) >= 2 {
			aircraft = rest[0]
			item.Name = rest[1]
			structure = "standard"
			for _, segment := range parts[:i] {
				if strings.EqualFold(segment, "CoreMods") || strings.EqualFold(segment, "Mods") || strings.EqualFold(segment, "Bazar") {
					structure = "misplaced"
					break
				}
			}
		}
		break
	}
	if structure == "nonstandard" {
		if len(parts) == 1 {
			structure = "flat"
		} else {
			item.Name = parts[len(parts)-1]
		}
	}
	name, _, textures := dcsdata.ParseLiveryDescription(meta[metaPath(root, "description.lua")])
	if aircraft == "" {
		aircraft = dcsdata.InferAircraft(name, textures)
	}
	for _, texture := range textures {
		if strings.HasPrefix(strings.ReplaceAll(texture, "\\", "/"), "../") {
			item.Warnings = append(item.Warnings, ImportWarning{Code: "external_dependencies"})
			break
		}
	}
	item.Aircraft = aircraft
	item.Structure = structure
	if aircraft == "" {
		item.Warnings = append(item.Warnings, ImportWarning{Code: "aircraft_unknown"})
	}
	switch structure {
	case "flat":
		item.TargetRel = path.Join("Liveries", item.Name)
		item.Warnings = append(item.Warnings, ImportWarning{Code: "structure_flat"})
	case "misplaced":
		item.TargetRel = path.Join("Liveries", aircraft, item.Name)
		item.Warnings = append(item.Warnings, ImportWarning{Code: "structure_misplaced"})
	case "standard":
		item.TargetRel = path.Join("Liveries", aircraft, item.Name)
	default:
		item.TargetRel = path.Join("Liveries", item.Name)
		item.Warnings = append(item.Warnings, ImportWarning{Code: "structure_nonstandard"})
	}
	item.SizeBytes, item.Files = rootSize(entries, root)
	return item
}

func rootSize(entries []archiveEntry, root string) (int64, int) {
	var bytes int64
	var files int
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}
		name := strings.ReplaceAll(entry.Name, "\\", "/")
		if root != "" && name != root && !strings.HasPrefix(name, root+"/") {
			continue
		}
		bytes += entry.Size
		files++
	}
	return bytes, files
}

func knownModCategory(name string) bool {
	switch strings.ToLower(name) {
	case "aircraft", "tech", "terrains", "campaigns", "services", "misc":
		return true
	}
	return false
}
