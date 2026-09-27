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

	"dcsmm/internal/luafiles"
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
	// When empty, or when it does not contain a target, the embedded copies
	// (package luafiles) are used instead, so a lone dcsmm.exe can still install.
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

// readSource returns the contents of a distribution file, preferring the on-disk
// dcs-lua directory (so a developer's edits apply immediately) and falling back
// to the embedded copy (so a released binary works on its own).
//
// The relative path is normalised to forward slashes, because the embedded keys
// are POSIX-style while filepath.Join uses the host separator.
func (in *Installer) readSource(rel string) ([]byte, error) {
	normalized := filepath.ToSlash(rel)

	if in.LuaDir != "" {
		data, err := os.ReadFile(filepath.Join(in.LuaDir, filepath.FromSlash(normalized)))
		if err == nil {
			return data, nil
		}
		// Only a missing file falls back to the embedded copy. A file that
		// exists but cannot be read (permissions, a lock) is a real problem: the
		// embedded copy may be older, so installing it silently would ship the
		// wrong scripts.
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("reading %s: %w", normalized, err)
		}
	}
	if data, ok := luafiles.Get(normalized); ok {
		return data, nil
	}
	return nil, fmt.Errorf("script %q not found (neither on disk nor embedded)", normalized)
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
	dest := filepath.Join(in.SavedGames, t.DestRel)

	content, err := in.readSource(t.Source)
	if err != nil {
		return Result{DestRel: t.DestRel}, err
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

// Marker keywords, without the punctuation. The full markers are matched first;
// the keyword is a fallback so that a marker whose punctuation was damaged by an
// encoding round-trip (a tool that read the file as Latin-1 and wrote it back as
// UTF-8 turns the em dash into "â€"") is still recognised for what it is.
const (
	beginKeyword = "DCSMM-BEGIN"
	endKeyword   = "DCSMM-END"
)

// lineContaining returns the whole line holding kw, at or after from, with its
// byte range (end includes the trailing newline).
//
// Only a Lua comment line is accepted. The markers are always comments, and
// requiring that keeps a passing mention of the keyword in real code from being
// mistaken for a block boundary — a mistake here would delete user content.
func lineContaining(s string, from int, kw string) (start, end int, ok bool) {
	search := from
	for {
		i := strings.Index(s[search:], kw)
		if i < 0 {
			return 0, 0, false
		}
		i += search
		start = strings.LastIndexByte(s[:i], '\n') + 1
		end = len(s)
		if j := strings.IndexByte(s[i:], '\n'); j >= 0 {
			end = i + j + 1
		}
		if strings.HasPrefix(strings.TrimSpace(s[start:end]), "--") {
			return start, end, true
		}
		search = end
	}
}

// findBlocks returns the byte ranges of every DCSMM managed block in s, in order.
//
// Every block is found, not just the first, on purpose: an Export.lua that, for
// whatever reason, ended up with two blocks must be repaired rather than left
// with the old one still defining the same Lua globals.
func findBlocks(s string) [][2]int {
	var spans [][2]int
	from := 0
	for {
		bStart, bEnd, ok := lineContaining(s, from, beginKeyword)
		if !ok {
			break
		}
		_, eEnd, ok := lineContaining(s, bEnd, endKeyword)
		if !ok {
			break
		}
		spans = append(spans, [2]int{bStart, eEnd})
		from = eEnd
	}
	return spans
}

// extractBlock returns the managed block of a distribution file, markers
// included, ending with a newline so that it always spans whole lines. Returns
// "" when no block is present.
func extractBlock(content string) string {
	spans := findBlocks(content)
	if len(spans) == 0 {
		return ""
	}
	return content[spans[0][0]:spans[0][1]]
}

// spliceBlock merges a managed block into an existing file:
//
//   - duplicate blocks are collapsed into a single one;
//   - the block replaces the first one in place, so surrounding content and the
//     position of the block are preserved;
//   - the block is appended at the end when none is present.
//
// It returns the new content and whether anything changed.
func spliceBlock(existing, block string) (string, bool) {
	block = strings.TrimRight(block, "\n") + "\n"

	spans := findBlocks(existing)
	if len(spans) == 0 {
		// Append, ensuring a clean separation from the previous content.
		out := existing
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		if out != "" {
			out += "\n"
		}
		out += block
		return out, true
	}

	// The block takes the place of the first one; anything sitting between two
	// blocks belongs to the user and is kept.
	var b strings.Builder
	b.WriteString(existing[:spans[0][0]])
	b.WriteString(block)
	prev := spans[0][1]
	for _, span := range spans[1:] {
		b.WriteString(existing[prev:span[0]])
		prev = span[1]
	}
	b.WriteString(existing[prev:])

	out := b.String()
	if out == existing {
		return existing, false
	}
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
	//
	// The loop is bounded: a stat error other than "does not exist" (a permission
	// problem on the parent directory, say) would otherwise never break it and
	// grow the path string forever.
	const maxAttempts = 1000
	for i := 1; i <= maxAttempts; i++ {
		_, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			// Could not tell whether the path is taken; stop rather than spin.
			return "", fmt.Errorf("checking backup path %s: %w", path, err)
		}
		if i == maxAttempts {
			return "", fmt.Errorf("could not find a free backup name for %s", dest)
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

	// Export.lua: strip every managed block (there may be more than one in a
	// file that an older installer appended to), preserving the rest.
	exportPath := filepath.Join(in.SavedGames, "Scripts", "Export.lua")
	if existing, err := os.ReadFile(exportPath); err == nil {
		spans := findBlocks(string(existing))
		if len(spans) > 0 {
			src := string(existing)
			var out strings.Builder
			prev := 0
			for _, span := range spans {
				out.WriteString(src[prev:span[0]])
				prev = span[1]
			}
			out.WriteString(src[prev:])

			remaining := strings.TrimRight(out.String(), "\n")
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
		dest := filepath.Join(in.SavedGames, t.DestRel)

		srcContent, srcErr := in.readSource(t.Source)
		destContent, destErr := os.ReadFile(dest)

		res := Result{DestRel: t.DestRel}
		switch {
		case srcErr != nil:
			res.Action = "unknown"
			res.Note = "source not found"
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
