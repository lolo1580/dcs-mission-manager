// Package install handles installing (and removing) the DCS-side Lua scripts
// into a DCS "Saved Games" folder.
//
// The guiding rule is safety: an existing Export.lua is almost always already
// used by other tools (Tacview, SRS, DCS-BIOS, …). This package therefore never
// overwrites such a file. It merges a marked block between
// `>>> DCSMM-BEGIN >>>` and `<<< DCSMM-END <<<` markers, and always keeps a
// timestamped backup before touching anything.
package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Markers delimiting the managed block. These strings are load-bearing: they
// are written into the user's Export.lua and must match byte-for-byte between
// this constant and the `dcs-lua/` scripts.
const (
	BeginMarker = "-- >>> DCSMM-BEGIN (managed block — do not edit by hand) >>>"
	EndMarker   = "-- <<< DCSMM-END <<<"
)

// Target describes where a script must be installed inside Saved Games.
type Target struct {
	// Source is the file to read from the distribution (dcs-lua/...).
	Source string
	// DestRel is the destination path relative to the Saved Games root.
	DestRel string
	// NeedsBlock is true when the file is merged into an existing file rather
	// than copied wholesale (Export.lua).
	NeedsBlock bool
}

// DefaultTargets is the set of files the manager installs.
func DefaultTargets() []Target {
	return []Target{
		{Source: "Export.lua", DestRel: filepath.Join("Scripts", "Export.lua"), NeedsBlock: true},
		{Source: filepath.Join("Hooks", "dcsmm.lua"), DestRel: filepath.Join("Scripts", "Hooks", "dcsmm.lua"), NeedsBlock: false},
		{Source: filepath.Join("Config", "dcsmm.cfg"), DestRel: filepath.Join("Config", "dcsmm.cfg"), NeedsBlock: false},
	}
}

// Result reports what happened for one target.
type Result struct {
	DestRel string `json:"destRel"`
	Action  string `json:"action"` // created | updated | merged | unchanged | skipped
	Backup  string `json:"backup,omitempty"`
	Note    string `json:"note,omitempty"`
}

// Installer installs the Lua scripts into a Saved Games folder.
type Installer struct {
	// LuaDir is the directory containing the distribution scripts (dcs-lua).
	LuaDir string
	// SavedGames is the DCS Saved Games folder.
	SavedGames string
	// DryRun reports actions without writing anything.
	DryRun bool
	// Now is overridable for tests (backup timestamps).
	Now func() time.Time
}

// New creates an installer.
func New(luaDir, savedGames string) *Installer {
	return &Installer{LuaDir: luaDir, SavedGames: savedGames, Now: time.Now}
}

// FindSavedGames locates the DCS Saved Games folder, preferring OpenBeta when
// present, and falling back to the default location under the user profile.
func FindSavedGames() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("dossier utilisateur introuvable : %w", err)
	}
	base := filepath.Join(home, "Saved Games")
	candidates := []string{
		filepath.Join(base, "DCS.openbeta"),
		filepath.Join(base, "DCS"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c, nil
		}
	}

	// Not found: report the candidates so the user can pass --saved-games.
	return "", fmt.Errorf(
		"DCS Saved Games folder not found (tried: %s). Use --saved-games <path>",
		strings.Join(candidates, ", "))
}

// Install installs (or updates) every target.
func (in *Installer) Install(targets []Target) ([]Result, error) {
	var results []Result
	for _, t := range targets {
		res, err := in.installOne(t)
		if err != nil {
			return results, fmt.Errorf("%s : %w", t.DestRel, err)
		}
		results = append(results, res)
	}
	return results, nil
}

func (in *Installer) installOne(t Target) (Result, error) {
	src := filepath.Join(in.LuaDir, t.Source)
	dest := filepath.Join(in.SavedGames, t.DestRel)

	content, err := os.ReadFile(src)
	if err != nil {
		return Result{DestRel: t.DestRel}, fmt.Errorf("lecture de la source : %w", err)
	}

	if !t.NeedsBlock {
		return in.copyFile(t.DestRel, dest, content)
	}
	return in.mergeBlock(t.DestRel, dest, content)
}

// copyFile writes content to dest, backing up an existing differing file.
func (in *Installer) copyFile(destRel, dest string, content []byte) (Result, error) {
	existing, err := os.ReadFile(dest)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if in.DryRun {
			return Result{DestRel: destRel, Action: "created", Note: "(simulation)"}, nil
		}
		if err := writeFile(dest, content); err != nil {
			return Result{DestRel: destRel}, err
		}
		return Result{DestRel: destRel, Action: "created"}, nil
	case err != nil:
		return Result{DestRel: destRel}, err
	case string(existing) == string(content):
		return Result{DestRel: destRel, Action: "unchanged"}, nil
	}

	backup, err := in.backup(dest, existing)
	if err != nil {
		return Result{DestRel: destRel}, err
	}
	if in.DryRun {
		return Result{DestRel: destRel, Action: "updated", Backup: backup, Note: "(simulation)"}, nil
	}
	if err := writeFile(dest, content); err != nil {
		return Result{DestRel: destRel}, err
	}
	return Result{DestRel: destRel, Action: "updated", Backup: backup}, nil
}

// mergeBlock installs content as a marked block inside dest, preserving any
// pre-existing content. It is idempotent: re-running replaces the block in place.
func (in *Installer) mergeBlock(destRel, dest string, content []byte) (Result, error) {
	block := extractBlock(string(content))
	if block == "" {
		return Result{DestRel: destRel}, errors.New("bloc DCSMM introuvable dans la source")
	}

	existing, err := os.ReadFile(dest)
	if errors.Is(err, os.ErrNotExist) {
		// No existing file: the source *is* the file.
		if in.DryRun {
			return Result{DestRel: destRel, Action: "created", Note: "(simulation)"}, nil
		}
		if err := writeFile(dest, content); err != nil {
			return Result{DestRel: destRel}, err
		}
		return Result{DestRel: destRel, Action: "created"}, nil
	}
	if err != nil {
		return Result{DestRel: destRel}, err
	}

	merged, changed := spliceBlock(string(existing), block)
	if !changed {
		return Result{DestRel: destRel, Action: "unchanged"}, nil
	}

	backup, err := in.backup(dest, existing)
	if err != nil {
		return Result{DestRel: destRel}, err
	}
	if in.DryRun {
		return Result{DestRel: destRel, Action: "merged", Backup: backup, Note: "(simulation)"}, nil
	}
	if err := writeFile(dest, []byte(merged)); err != nil {
		return Result{DestRel: destRel}, err
	}
	return Result{DestRel: destRel, Action: "merged", Backup: backup}, nil
}

// extractBlock returns the text from the BEGIN marker to the END marker of a
// distribution file, inclusive. Returns "" when the markers are absent.
func extractBlock(content string) string {
	start := strings.Index(content, BeginMarker)
	if start < 0 {
		return ""
	}
	end := strings.Index(content[start:], EndMarker)
	if end < 0 {
		return ""
	}
	return content[start : start+end+len(EndMarker)]
}

// spliceBlock merges a managed block into an existing file:
//
//   - if the file already contains the markers, the block is replaced in place;
//   - otherwise the block is appended at the end, on its own lines.
//
// It returns the new content and whether anything changed.
func spliceBlock(existing, block string) (string, bool) {
	if start := strings.Index(existing, BeginMarker); start >= 0 {
		if rel := strings.Index(existing[start:], EndMarker); rel >= 0 {
			end := start + rel + len(EndMarker)
			if existing[start:end] == block {
				return existing, false
			}
			return existing[:start] + block + existing[end:], true
		}
	}

	// Append, ensuring a clean separation from the previous content.
	out := existing
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if out != "" {
		out += "\n"
	}
	out += block + "\n"
	return out, true
}

// backup writes a timestamped copy of content next to dest and returns its path.
func (in *Installer) backup(dest string, content []byte) (string, error) {
	now := time.Now()
	if in.Now != nil {
		now = in.Now()
	}
	suffix := now.Format("20060102-150405")
	path := fmt.Sprintf("%s.bak-%s", dest, suffix)

	// Avoid clobbering an existing backup from the same second.
	for i := 1; ; i++ {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			break
		}
		path = fmt.Sprintf("%s.bak-%s-%d", dest, suffix, i)
	}

	if in.DryRun {
		return path, nil
	}
	if err := writeFile(path, content); err != nil {
		return "", err
	}
	return path, nil
}

// Uninstall removes the managed artifacts:
//
//   - the marked block is stripped from Export.lua (the rest is preserved);
//   - Hooks/dcsmm.lua is deleted;
//   - Config/dcsmm.cfg is left alone (it holds the user's address/ports).
func (in *Installer) Uninstall() ([]Result, error) {
	var results []Result

	// Export.lua: strip the block.
	exportPath := filepath.Join(in.SavedGames, "Scripts", "Export.lua")
	if existing, err := os.ReadFile(exportPath); err == nil {
		begin := strings.Index(string(existing), BeginMarker)
		if begin >= 0 {
			if rel := strings.Index(string(existing[begin:]), EndMarker); rel >= 0 {
				end := begin + rel + len(EndMarker)
				remaining := strings.TrimRight(string(existing[:begin]), "\n")
				if end < len(existing) {
					remaining += string(existing[end:])
				}
				remaining = strings.TrimRight(remaining, "\n")
				if remaining != "" {
					remaining += "\n"
				}

				backup, berr := in.backup(exportPath, existing)
				if berr != nil {
					return results, berr
				}
				if !in.DryRun {
					if err := writeFile(exportPath, []byte(remaining)); err != nil {
						return results, err
					}
				}
				results = append(results, Result{
					DestRel: filepath.Join("Scripts", "Export.lua"),
					Action:  "block-removed", Backup: backup,
				})
			}
		} else {
			results = append(results, Result{
				DestRel: filepath.Join("Scripts", "Export.lua"),
				Action:  "skipped", Note: "aucun bloc DCSMM",
			})
		}
	}

	// Hooks/dcsmm.lua: our own file, safe to delete.
	hooksPath := filepath.Join(in.SavedGames, "Scripts", "Hooks", "dcsmm.lua")
	if _, err := os.Stat(hooksPath); err == nil {
		if !in.DryRun {
			if err := os.Remove(hooksPath); err != nil {
				return results, err
			}
		}
		results = append(results, Result{
			DestRel: filepath.Join("Scripts", "Hooks", "dcsmm.lua"),
			Action:  "removed",
		})
	}

	return results, nil
}

// Status reports, for each default target, whether it is installed, missing or
// out of date relative to the distribution.
func (in *Installer) Status(targets []Target) []Result {
	var results []Result
	for _, t := range targets {
		src := filepath.Join(in.LuaDir, t.Source)
		dest := filepath.Join(in.SavedGames, t.DestRel)

		srcContent, srcErr := os.ReadFile(src)
		destContent, destErr := os.ReadFile(dest)

		res := Result{DestRel: t.DestRel}
		switch {
		case srcErr != nil:
			res.Action = "unknown"
			res.Note = "source introuvable"
		case errors.Is(destErr, os.ErrNotExist):
			res.Action = "missing"
		case destErr != nil:
			res.Action = "unknown"
			res.Note = destErr.Error()
		case t.NeedsBlock:
			installedBlock := extractBlock(string(destContent))
			if installedBlock == "" {
				res.Action = "missing"
				res.Note = "bloc absent"
			} else if installedBlock == extractBlock(string(srcContent)) {
				res.Action = "installed"
			} else {
				res.Action = "outdated"
			}
		default:
			if string(destContent) == string(srcContent) {
				res.Action = "installed"
			} else {
				res.Action = "outdated"
			}
		}
		results = append(results, res)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].DestRel < results[j].DestRel })
	return results
}

func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}
