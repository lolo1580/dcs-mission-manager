// Package backup saves and restores a DCS player's profile: the parts of the
// Saved Games folder that are painful to lose — the logbook, the control
// bindings, the options — as a single portable archive, with the categories to
// include chosen per call.
//
// It is deliberately self-contained and filesystem-only: no database, no DCS
// install beyond the Saved Games path. The archive is a plain zip (manifest.json
// plus a files/ tree), so a user can also open it with any zip tool.
package backup

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Category is one selectable group of the profile. Paths are relative to the
// Saved Games folder and may be files or directories; a path that does not exist
// is simply skipped when backing up (a player may have no Kneeboard, no Mods…).
type Category struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Paths   []string `json:"paths"`
	Default bool     `json:"default"`
	Note    string   `json:"note,omitempty"`
	// Large marks categories that can be many hundreds of MB (Mods, Missions),
	// so the UI can warn before selecting them.
	Large bool `json:"large,omitempty"`
}

// Categories is the fixed catalogue offered to the CLI, the API and the UI.
var Categories = []Category{
	{
		ID: "logbook", Label: "Logbook & modules", Default: true,
		Paths: []string{"MissionEditor/logbook.lua", "MissionEditor/modules.lua"},
	},
	{
		ID: "input", Label: "Control bindings", Default: true,
		Paths: []string{"Config/Input"},
	},
	{
		ID: "config", Label: "Options & manager config", Default: true,
		Paths: []string{"Config/options.lua", "Config/dcsmanager.cfg"},
	},
	{
		ID: "scripts", Label: "Scripts (Export.lua, hooks)", Default: true,
		Paths: []string{"Scripts"},
	},
	{
		ID: "kneeboard", Label: "Kneeboard", Default: false, Large: true,
		Paths: []string{"Kneeboard"},
	},
	{
		ID: "missions", Label: "Missions", Default: false, Large: true,
		Paths: []string{"Missions"},
	},
	{
		ID: "mods", Label: "Mods", Default: false, Large: true,
		Paths: []string{"Mods"},
	},
}

// DefaultCategories returns the ids selected by default.
func DefaultCategories() []string {
	var ids []string
	for _, c := range Categories {
		if c.Default {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

// Resolve maps category ids to categories. Empty input means "the defaults". An
// unknown id is an error rather than silently ignored, so a typo never produces
// a backup that is quietly missing a category.
func Resolve(ids []string) ([]Category, error) {
	if len(ids) == 0 {
		ids = DefaultCategories()
	}
	byID := map[string]Category{}
	for _, c := range Categories {
		byID[c.ID] = c
	}
	out := make([]Category, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		c, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("unknown category %q (known: %s)", id, strings.Join(categoryIDs(), ", "))
		}
		seen[id] = true
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, errors.New("no categories selected")
	}
	return out, nil
}

func categoryIDs() []string {
	ids := make([]string, len(Categories))
	for i, c := range Categories {
		ids[i] = c.ID
	}
	return ids
}

// Manifest describes one archive. It is written as manifest.json at the root.
type Manifest struct {
	Version    int       `json:"version"`
	CreatedAt  time.Time `json:"createdAt"`
	SavedGames string    `json:"savedGames,omitempty"`
	AppVersion string    `json:"appVersion,omitempty"`
	Categories []string  `json:"categories"`
	Entries    int       `json:"entries"`
	Bytes      int64     `json:"bytes"`
}

// Archive is the metadata of a stored backup.
type Archive struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	CreatedAt  time.Time `json:"createdAt"`
	Categories []string  `json:"categories"`
	Entries    int       `json:"entries"`
	Bytes      int64     `json:"bytes"`
	AppVersion string    `json:"appVersion,omitempty"`
}

const manifestName = "manifest.json"
const filesPrefix = "files/"

// Create writes a new archive of the selected categories under outDir and returns
// its metadata. The archive name is timestamped, so repeated calls never clash.
func Create(savedGames, outDir string, ids []string, appVersion string) (Archive, error) {
	cats, err := Resolve(ids)
	if err != nil {
		return Archive{}, err
	}
	if savedGames == "" {
		return Archive{}, errors.New("DCS Saved Games folder not found")
	}
	if info, err := os.Stat(savedGames); err != nil || !info.IsDir() {
		return Archive{}, fmt.Errorf("Saved Games folder %q is not a directory", savedGames)
	}
	if outDir == "" {
		return Archive{}, errors.New("no output directory for the backup")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Archive{}, err
	}

	name := "dcsmanager-backup-" + time.Now().Format("20060102-150405") + ".zip"
	path := filepath.Join(outDir, name)
	if err := createTo(savedGames, path, cats, appVersion); err != nil {
		return Archive{}, err
	}
	return readArchive(path)
}

// createTo writes the archive at a precise path. Restore uses it for the
// pre-restore safety copy, where the name must be predictable.
func createTo(savedGames, path string, cats []Category, appVersion string) (err error) {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	// Clean up a half-written temp file on any failure.
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()

	zw := zip.NewWriter(f)
	var entries int
	var bytes int64

	for _, cat := range cats {
		for _, rel := range cat.Paths {
			src := filepath.Join(savedGames, filepath.FromSlash(rel))
			info, statErr := os.Lstat(src)
			if statErr != nil {
				continue // category path absent: skip, not an error
			}
			add := func(p string) error {
				relToSG, relErr := filepath.Rel(savedGames, p)
				if relErr != nil {
					return relErr
				}
				zipName := filesPrefix + cat.ID + "/" + filepath.ToSlash(relToSG)
				n, wErr := addFile(zw, p, zipName)
				if wErr != nil {
					return wErr
				}
				entries++
				bytes += n
				return nil
			}
			if info.IsDir() {
				if wErr := filepath.WalkDir(src, func(p string, d fs.DirEntry, walkErr error) error {
					if walkErr != nil {
						return walkErr
					}
					if d.IsDir() {
						return nil
					}
					return add(p)
				}); wErr != nil {
					return wErr
				}
			} else if info.Mode().IsRegular() {
				if addErr := add(src); addErr != nil {
					return addErr
				}
			}
		}
	}

	man := Manifest{
		Version:    1,
		CreatedAt:  time.Now().UTC(),
		SavedGames: filepath.Base(savedGames),
		AppVersion: appVersion,
		Categories: categoryIDsOf(cats),
		Entries:    entries,
		Bytes:      bytes,
	}
	if err = writeManifest(zw, man); err != nil {
		return err
	}
	if err = zw.Close(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func categoryIDsOf(cats []Category) []string {
	ids := make([]string, len(cats))
	for i, c := range cats {
		ids[i] = c.ID
	}
	return ids
}

// addFile copies one file into the zip and returns how many bytes were written.
func addFile(zw *zip.Writer, src, zipName string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return 0, err
	}
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return 0, err
	}
	hdr.Name = zipName
	hdr.Method = zip.Deflate
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return 0, err
	}
	return io.Copy(w, in)
}

func writeManifest(zw *zip.Writer, man Manifest) error {
	w, err := zw.Create(manifestName)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(man)
}

// List returns the archives found in dir, newest first. A non-archive file is
// ignored rather than failing the whole listing.
func List(dir string) ([]Archive, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Archive{}, nil
		}
		return nil, err
	}
	var out []Archive
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".zip") {
			continue
		}
		a, err := readArchive(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// readArchive reads an archive's manifest, falling back to the file's own
// timestamp when the manifest is missing or unreadable.
func readArchive(path string) (Archive, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Archive{}, err
	}
	a := Archive{Name: filepath.Base(path), Path: path, CreatedAt: info.ModTime().UTC(), Bytes: info.Size()}

	zr, err := zip.OpenReader(path)
	if err != nil {
		return Archive{}, fmt.Errorf("not a zip archive: %w", err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != manifestName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return a, nil
		}
		var man Manifest
		decErr := json.NewDecoder(io.LimitReader(rc, 1<<20)).Decode(&man)
		rc.Close()
		if decErr == nil {
			a.CreatedAt = man.CreatedAt
			a.Categories = man.Categories
			a.Entries = man.Entries
			a.AppVersion = man.AppVersion
		}
		return a, nil
	}
	return a, nil
}

// RestoreResult reports what a restore did (or would do, on a dry run).
type RestoreResult struct {
	Files        int    `json:"files"`
	DryRun       bool   `json:"dryRun"`
	SafetyBackup string `json:"safetyBackup,omitempty"`
}

// Restore extracts an archive back into the Saved Games folder.
//
// Safety first, because this overwrites live DCS files:
//   - only entries under files/ are considered, and every target is checked to
//     stay inside Saved Games (no "../", no absolute path from the archive);
//   - before writing anything, the current state of the archive's categories is
//     itself saved beside the archive as a .prerestore.zip, so a wrong restore
//     can be undone;
//   - a dry run reports the file count without touching the disk.
func Restore(archivePath, savedGames string, dryRun bool) (RestoreResult, error) {
	if savedGames == "" {
		return RestoreResult{}, errors.New("DCS Saved Games folder not found")
	}
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return RestoreResult{}, err
	}
	defer zr.Close()

	var man Manifest
	type target struct {
		src  *zip.File
		dest string
	}
	var targets []target
	root := filepath.Clean(savedGames)

	for _, f := range zr.File {
		if f.Name == manifestName {
			rc, oErr := f.Open()
			if oErr == nil {
				_ = json.NewDecoder(io.LimitReader(rc, 1<<20)).Decode(&man)
				rc.Close()
			}
			continue
		}
		if !strings.HasPrefix(f.Name, filesPrefix) {
			continue // ignore anything unexpected at the archive root
		}
		rest := strings.TrimPrefix(f.Name, filesPrefix)
		slash := strings.IndexByte(rest, '/')
		if slash <= 0 || slash == len(rest)-1 {
			return RestoreResult{}, fmt.Errorf("malformed entry %q", f.Name)
		}
		rel := rest[slash+1:] // drop the category segment
		dest := filepath.Join(root, filepath.FromSlash(rel))
		// Path-traversal guard: the resolved target must stay under Saved Games.
		if dest != root && !strings.HasPrefix(dest, root+string(os.PathSeparator)) {
			return RestoreResult{}, fmt.Errorf("refusing entry outside Saved Games: %q", f.Name)
		}
		targets = append(targets, target{src: f, dest: dest})
	}

	res := RestoreResult{Files: len(targets), DryRun: dryRun}
	if dryRun || len(targets) == 0 {
		return res, nil
	}

	// Safety copy of what is about to be overwritten.
	if len(man.Categories) > 0 {
		safety := strings.TrimSuffix(archivePath, ".zip") + ".prerestore.zip"
		if cats, sErr := Resolve(man.Categories); sErr == nil {
			if cErr := createTo(savedGames, safety, cats, man.AppVersion); cErr == nil {
				res.SafetyBackup = safety
			}
		}
	}

	for _, t := range targets {
		if err := os.MkdirAll(filepath.Dir(t.dest), 0o755); err != nil {
			return res, err
		}
		if err := extractFile(t.src, t.dest); err != nil {
			return res, err
		}
	}
	return res, nil
}

func extractFile(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	mode := f.Mode()
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
