package dcsdata

import (
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"dcsmanager/internal/lua"
)

// LiveryWarning is a machine-readable warning the UI translates.
type LiveryWarning struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// LiveryRef identifies one livery without trusting a client-supplied path.
// For user liveries, RelPath is relative to Saved Games\DCS\Liveries. For mod
// liveries, RelPath is relative to that mod's own Liveries folder.
type LiveryRef struct {
	Source      string `json:"source"` // "user" or "mod"
	ModCategory string `json:"modCategory,omitempty"`
	ModName     string `json:"modName,omitempty"`
	RelPath     string `json:"relPath"`
}

// Livery is one skin folder, whether it lives in the standard
// Liveries\<Aircraft>\<Livery> layout, flat under Liveries, or inside a mod.
type Livery struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Aircraft    string   `json:"aircraft,omitempty"`
	Source      string   `json:"source"`
	ModCategory string   `json:"modCategory,omitempty"`
	ModName     string   `json:"modName,omitempty"`
	ModEnabled  bool     `json:"modEnabled,omitempty"`
	RelPath     string   `json:"relPath"`
	Path        string   `json:"path"`
	SizeBytes   int64    `json:"sizeBytes"`
	Files       int      `json:"files"`
	Countries   []string `json:"countries,omitempty"`
	// Thumbnail is a file name inside the livery folder, not a URL.
	Thumbnail string `json:"thumbnail,omitempty"`
	// Dependencies lists sibling livery folders referenced through ../ paths.
	Dependencies    []string        `json:"dependencies,omitempty"`
	MissingTextures []string        `json:"missingTextures,omitempty"`
	Structure       string          `json:"structure"` // standard, flat, misplaced, nonstandard
	Warnings        []LiveryWarning `json:"warnings,omitempty"`
}

// LiveryAircraftCount is a filter entry for the UI.
type LiveryAircraftCount struct {
	Aircraft string `json:"aircraft"`
	Count    int    `json:"count"`
}

// LiveryScan is the complete livery picture for the content manager.
type LiveryScan struct {
	Liveries      []Livery              `json:"liveries"`
	Aircraft      []LiveryAircraftCount `json:"aircraft"`
	TotalBytes    int64                 `json:"totalBytes"`
	UserRoot      string                `json:"userRoot,omitempty"`
	QuarantineDir string                `json:"quarantineDir,omitempty"`
	Warnings      []LiveryWarning       `json:"warnings,omitempty"`
}

// LiveriesDir returns Saved Games\DCS\Liveries.
func LiveriesDir(savedGames string) string {
	return filepath.Join(savedGames, "Liveries")
}

// LiveriesQuarantineDir returns the sibling folder used for removed liveries.
func LiveriesQuarantineDir(savedGames string) string {
	return filepath.Join(savedGames, "Liveries.quarantine")
}

// EncodeLiveryRef returns the opaque ID used by the API and the UI.
func EncodeLiveryRef(ref LiveryRef) string {
	raw, _ := json.Marshal(ref)
	return base64.RawURLEncoding.EncodeToString(raw)
}

// DecodeLiveryRef parses an opaque livery ID.
func DecodeLiveryRef(id string) (LiveryRef, error) {
	var ref LiveryRef
	raw, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return ref, err
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		return ref, err
	}
	return ref, nil
}

// ScanLiveries lists user liveries and the liveries bundled inside enabled or
// disabled mods. It never writes anything.
func ScanLiveries(savedGames string) (LiveryScan, error) {
	scan := LiveryScan{
		UserRoot:      LiveriesDir(savedGames),
		QuarantineDir: LiveriesQuarantineDir(savedGames),
	}
	if savedGames == "" {
		return scan, nil
	}

	seen := map[string]bool{}
	if root := scan.UserRoot; root != "" {
		user, err := scanLiveryRoot(root, "user", InstalledMod{})
		if err != nil {
			return scan, err
		}
		scan.Liveries = append(scan.Liveries, user...)
	}
	mods, err := ScanMods(savedGames)
	if err != nil {
		return scan, err
	}
	for _, mod := range mods.Mods {
		root := filepath.Join(mod.Path, "Liveries")
		if !isDir(root) {
			continue
		}
		found, err := scanLiveryRoot(root, "mod", mod)
		if err != nil {
			continue
		}
		scan.Liveries = append(scan.Liveries, found...)
	}

	// A mod and the user root cannot normally overlap, but a hand-made junction
	// could. Keep the first occurrence so one skin is not listed twice.
	filtered := scan.Liveries[:0]
	for _, livery := range scan.Liveries {
		if seen[livery.ID] {
			continue
		}
		seen[livery.ID] = true
		filtered = append(filtered, livery)
	}
	scan.Liveries = filtered

	sort.SliceStable(scan.Liveries, func(i, j int) bool {
		a, b := scan.Liveries[i], scan.Liveries[j]
		if a.Aircraft != b.Aircraft {
			if a.Aircraft == "" {
				return false
			}
			if b.Aircraft == "" {
				return true
			}
			return strings.ToLower(a.Aircraft) < strings.ToLower(b.Aircraft)
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})

	counts := map[string]int{}
	for _, livery := range scan.Liveries {
		scan.TotalBytes += livery.SizeBytes
		counts[livery.Aircraft]++
	}
	for aircraft, count := range counts {
		scan.Aircraft = append(scan.Aircraft, LiveryAircraftCount{Aircraft: aircraft, Count: count})
	}
	sort.SliceStable(scan.Aircraft, func(i, j int) bool {
		if scan.Aircraft[i].Aircraft == "" {
			return false
		}
		if scan.Aircraft[j].Aircraft == "" {
			return true
		}
		return strings.ToLower(scan.Aircraft[i].Aircraft) < strings.ToLower(scan.Aircraft[j].Aircraft)
	})
	return scan, nil
}

// scanLiveryRoot walks a Liveries-style folder and returns every description.lua
// it finds. A wrapper folder without a description.lua is transparent: the
// nested livery folders are still discovered.
func scanLiveryRoot(root, source string, mod InstalledMod) ([]Livery, error) {
	root = filepath.Clean(root)
	if !isDir(root) {
		return nil, nil
	}
	var out []Livery
	err := filepath.WalkDir(root, func(full string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		relFull, relErr := filepath.Rel(root, full)
		if relErr != nil {
			return nil
		}
		relFull = filepath.ToSlash(relFull)
		if relFull != "." && strings.Count(relFull, "/") > 9 {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || !strings.EqualFold(d.Name(), "description.lua") {
			return nil
		}
		relDir := path.Dir(relFull)
		if relDir == "." {
			relDir = ""
		}
		livery := buildLivery(root, relDir, source, mod)
		// Cockpit texture packs use the same description.lua shape but are not
		// selectable liveries; listing them would drown the real skins.
		if strings.HasPrefix(strings.ToLower(livery.Aircraft), "cockpit_") {
			return nil
		}
		out = append(out, livery)
		return nil
	})
	return out, err
}

// buildLivery reads one livery folder. It never fails the whole scan because a
// single malformed description.lua is common in downloaded packs.
func buildLivery(root, relDir, source string, mod InstalledMod) Livery {
	full := filepath.Join(root, filepath.FromSlash(relDir))
	l := Livery{
		Name:        filepath.Base(full),
		Source:      source,
		ModCategory: mod.Category,
		ModName:     mod.Name,
		ModEnabled:  mod.Enabled,
		RelPath:     relDir,
		Path:        full,
	}
	l.SizeBytes, l.Files = folderSize(full)
	if l.Name == "." || l.Name == "" {
		l.Name = "Livery"
	}

	structure, aircraft := classifyLiveryPath(relDir, source)
	l.Structure = structure
	l.Aircraft = aircraft

	data, err := os.ReadFile(filepath.Join(full, "description.lua"))
	if err != nil {
		l.Warnings = append(l.Warnings, LiveryWarning{Code: "description_unreadable", Detail: err.Error()})
	} else {
		name, countries, textures := parseLiveryDescription(data)
		if name != "" {
			l.Name = name
		}
		l.Countries = countries
		if l.Aircraft == "" {
			l.Aircraft = inferAircraft(l.Name, textures)
		}
		l.Dependencies, l.MissingTextures = inspectTextures(root, relDir, textures)
		for _, dep := range l.Dependencies {
			depPath := filepath.Join(root, filepath.FromSlash(dep))
			if !isDir(depPath) {
				l.Warnings = append(l.Warnings, LiveryWarning{Code: "missing_dependency", Detail: dep})
			}
		}
		if len(l.MissingTextures) > 0 {
			l.Warnings = append(l.Warnings, LiveryWarning{
				Code:   "missing_textures",
				Detail: strings.Join(l.MissingTextures[:min(len(l.MissingTextures), 12)], ", "),
			})
		}
	}
	if l.Aircraft == "" {
		l.Warnings = append(l.Warnings, LiveryWarning{Code: "unknown_aircraft"})
	}
	switch structure {
	case "flat":
		l.Warnings = append(l.Warnings, LiveryWarning{Code: "structure_flat"})
	case "misplaced":
		l.Warnings = append(l.Warnings, LiveryWarning{Code: "structure_misplaced"})
	case "nonstandard":
		l.Warnings = append(l.Warnings, LiveryWarning{Code: "structure_nonstandard"})
	}
	if source == "mod" && !mod.Enabled {
		l.Warnings = append(l.Warnings, LiveryWarning{Code: "livery_in_disabled_mod"})
	}
	l.Thumbnail = findThumbnail(full)
	l.ID = EncodeLiveryRef(LiveryRef{
		Source:      source,
		ModCategory: mod.Category,
		ModName:     mod.Name,
		RelPath:     relDir,
	})
	return l
}

// classifyLiveryPath recognises the common layouts and returns the structure
// plus the aircraft folder when it is authoritative.
func classifyLiveryPath(relDir, source string) (structure, aircraft string) {
	parts := strings.Split(relDir, "/")
	lower := make([]string, len(parts))
	for i, part := range parts {
		lower[i] = strings.ToLower(part)
	}
	// Paths inside a mis-extracted pack are full
	// CoreMods/aircraft/.../Liveries/Aircraft/Livery trees. Find the last
	// "Liveries" segment and read the two folders after it.
	for i := len(lower) - 1; i >= 0; i-- {
		if lower[i] != "liveries" {
			continue
		}
		rest := parts[i+1:]
		if len(rest) >= 2 {
			structure = "standard"
			for _, segment := range lower[:i] {
				if segment == "coremods" || segment == "mods" || segment == "bazar" {
					structure = "misplaced"
					break
				}
			}
			return structure, rest[0]
		}
	}
	switch len(parts) {
	case 1:
		return "flat", ""
	case 2:
		return "standard", parts[0]
	default:
		// A path such as "Swiss Mirage III/CoreMods/..." has no Liveries
		// segment. The parent is not reliably an aircraft, so infer later.
		return "nonstandard", ""
	}
}

// ParseLiveryDescription exposes the description.lua reader to the import
// pipeline, which inspects files before they are installed.
func ParseLiveryDescription(data []byte) (name string, countries []string, textures []string) {
	return parseLiveryDescription(data)
}

// InferAircraft exposes the texture-signature helper to the import pipeline.
func InferAircraft(name string, textures []string) string {
	return inferAircraft(name, textures)
}

// parseLiveryDescription reads the fields a manager needs from description.lua.
// The existing Lua data parser handles the normal DCS shape; a small regex
// fallback keeps a hand-edited file from hiding the livery entirely.
func parseLiveryDescription(data []byte) (name string, countries []string, textures []string) {
	if root, err := lua.Parse(data); err == nil {
		if v, ok := root["name"].(string); ok {
			name = strings.TrimSpace(v)
		}
		countries = stringsFromLua(root["countries"])
		textures = texturesFromLua(root["livery"])
		if name != "" || len(textures) > 0 {
			return name, countries, textures
		}
	}

	text := string(data)
	name = firstMatch(regexpName, text)
	countries = regexpQuoted(regexpCountries.FindStringSubmatch(text))
	textures = regexpTextures(text)
	return name, countries, textures
}

var (
	regexpName       = regexp.MustCompile(`(?m)\bname\s*=\s*["']([^"']+)["']`)
	regexpCountries  = regexp.MustCompile(`(?s)\bcountries\s*=\s*\{([^}]*)\}`)
	regexpTextureRow = regexp.MustCompile(`\{[^{}]*,\s*[^,}]*,\s*["']([^"']+)["']`)
	regexpQuotedText = regexp.MustCompile(`["']([^"']+)["']`)
)

func regexpQuoted(groups []string) []string {
	if len(groups) < 2 {
		return nil
	}
	var out []string
	for _, match := range regexpQuotedText.FindAllStringSubmatch(groups[1], -1) {
		if len(match) > 1 && match[1] != "" {
			out = append(out, match[1])
		}
	}
	return out
}

func regexpTextures(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, match := range regexpTextureRow.FindAllStringSubmatch(text, -1) {
		if len(match) < 2 {
			continue
		}
		texture := strings.TrimSpace(match[1])
		if texture == "" || strings.EqualFold(texture, "empty") || seen[texture] {
			continue
		}
		seen[texture] = true
		out = append(out, texture)
	}
	return out
}

// texturesFromLua accepts both the slice and the numeric-keyed map forms the
// Lua parser can produce for a table of texture rows.
func texturesFromLua(v any) []string {
	var out []string
	seen := map[string]bool{}
	add := func(row any) {
		values := rowValues(row)
		if len(values) < 3 {
			return
		}
		texture, ok := values[2].(string)
		if !ok {
			return
		}
		texture = strings.TrimSpace(texture)
		if texture == "" || strings.EqualFold(texture, "empty") || seen[texture] {
			return
		}
		seen[texture] = true
		out = append(out, texture)
	}
	switch rows := v.(type) {
	case []any:
		for _, row := range rows {
			add(row)
		}
	case map[string]any:
		keys := make([]string, 0, len(rows))
		for key := range rows {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool { return naturalLess(keys[i], keys[j]) })
		for _, key := range keys {
			add(rows[key])
		}
	}
	return out
}

func rowValues(row any) []any {
	switch values := row.(type) {
	case []any:
		return values
	case map[string]any:
		out := make([]any, 0, len(values))
		for i := 1; ; i++ {
			v, ok := values[strconv.Itoa(i)]
			if !ok {
				break
			}
			out = append(out, v)
		}
		return out
	}
	return nil
}

func stringsFromLua(v any) []string {
	var out []string
	switch values := v.(type) {
	case []any:
		for _, value := range values {
			if s, ok := value.(string); ok && s != "" {
				out = append(out, s)
			}
		}
	case map[string]any:
		for _, value := range values {
			if s, ok := value.(string); ok && s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// inspectTextures resolves ../ references and checks the referenced files. It
// returns dependency folders relative to root and missing texture names.
func inspectTextures(root, relDir string, textures []string) ([]string, []string) {
	deps := map[string]bool{}
	missing := map[string]bool{}
	for _, texture := range textures {
		tex := strings.ReplaceAll(strings.TrimSpace(texture), "\\", "/")
		if tex == "" || strings.EqualFold(tex, "empty") {
			continue
		}
		if strings.HasPrefix(tex, "/") || strings.HasPrefix(tex, "../") || strings.Contains(tex, "/../") {
			joined := path.Clean(path.Join(relDir, path.Dir(tex)))
			if joined == ".." || strings.HasPrefix(joined, "../") {
				missing[texture] = true
				continue
			}
			deps[joined] = true
			depIndex := fileIndex(filepath.Join(root, filepath.FromSlash(joined)))
			if !textureFound(path.Base(tex), depIndex) {
				missing[texture] = true
			}
			continue
		}
		if strings.Contains(tex, "/") {
			// A subfolder inside the livery: check it under the livery folder.
			if !fileExists(filepath.Join(root, filepath.FromSlash(relDir), filepath.FromSlash(tex))) {
				missing[texture] = true
			}
			continue
		}
		// A bare texture name is normally inherited from the default livery or
		// from DCS itself. Treating it as missing would flag almost every skin,
		// so only explicit paths are checked.
	}
	dependencies := make([]string, 0, len(deps))
	for dep := range deps {
		dependencies = append(dependencies, dep)
	}
	sort.Strings(dependencies)
	missingList := make([]string, 0, len(missing))
	for name := range missing {
		missingList = append(missingList, name)
	}
	sort.Strings(missingList)
	return dependencies, missingList
}

// fileIndex returns the lower-case basenames of every file in dir. DCS lookup
// is case-insensitive on Windows, so the manager compares that way too.
func fileIndex(dir string) map[string]bool {
	index := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return index
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			index[strings.ToLower(entry.Name())] = true
		}
	}
	return index
}

func textureFound(name string, index map[string]bool) bool {
	base := strings.ToLower(strings.TrimSpace(name))
	if base == "" {
		return true
	}
	if index[base] {
		return true
	}
	for _, ext := range []string{".dds", ".tga", ".png", ".jpg", ".jpeg", ".bmp"} {
		if index[base+ext] {
			return true
		}
	}
	return false
}

// findThumbnail returns an image file DCS packs sometimes ship next to
// description.lua. DDS textures cannot be shown by a browser, so only ordinary
// web image formats are considered.
func findThumbnail(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	prefixes := []string{"thumbnail", "preview", "screenshot", "picture"}
	exts := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true}
	for _, prefix := range prefixes {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := strings.ToLower(entry.Name())
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			if exts[strings.ToLower(filepath.Ext(name))] {
				return entry.Name()
			}
		}
	}
	return ""
}

// aircraftSignature maps DCS texture prefixes and folder names to the aircraft
// type folder DCS uses. It covers the common modules and is intentionally
// conservative: when no signature matches, the livery stays "unknown" and the
// user can classify it explicitly.
type aircraftSignature struct {
	Aircraft string
	Tokens   []string
}

var aircraftSignatures = []aircraftSignature{
	{"F-5E-3", []string{"f-5e3", "f-5e-3", "f-5e"}},
	{"M-2000C", []string{"m2kc", "m2000", "m-2000"}},
	{"FA-18C_hornet", []string{"f18c", "fa-18c", "hornet_"}},
	{"F-16C_50", []string{"f-16c", "f16c"}},
	{"F-14B", []string{"f-14b", "f14b", "f-14"}},
	{"F-15C", []string{"f-15c", "f15c"}},
	{"F-4E-45MC", []string{"f-4e", "f4e"}},
	{"A-10C_2", []string{"a-10c", "a10c"}},
	{"A-4E-C", []string{"a-4e", "a4e"}},
	{"AV8BNA", []string{"av8b", "harrier"}},
	{"AH-64D_BLK_II", []string{"ah-64", "ah64", "apache"}},
	{"UH-1H", []string{"uh-1", "uh1", "huey"}},
	{"Mi-24P", []string{"mi-24", "mi24", "hind"}},
	{"Ka-50", []string{"ka-50", "ka50"}},
	{"AJS37", []string{"ajs37", "viggen"}},
	{"JF-17", []string{"jf-17", "jf17"}},
	{"C-101CC", []string{"c-101", "c101"}},
	{"L-39C", []string{"l-39", "l39"}},
	{"M-2000C", []string{"mirage 2000", "mirage-2000"}},
	{"SpitfireLFMkIX", []string{"spitfire"}},
	{"P-51D-30-NA", []string{"p-51", "p51"}},
	{"FW-190D9", []string{"fw-190", "fw190"}},
	{"Bf-109K-4", []string{"bf-109", "bf109"}},
	{"MosquitoFBMkVI", []string{"mosquito"}},
	{"MiG-15bis", []string{"mig-15", "mig15"}},
	{"MiG-19P", []string{"mig-19", "mig19"}},
	{"MiG-21bis", []string{"mig-21", "mig21"}},
	{"MiG-29A", []string{"mig-29", "mig29"}},
	{"Su-27", []string{"su-27", "su27"}},
	{"Su-33", []string{"su-33", "su33"}},
	{"Su-25T", []string{"su-25", "su25"}},
	{"F-86F-35", []string{"f-86", "f86"}},
}

func inferAircraft(name string, textures []string) string {
	haystack := strings.ToLower(name)
	for _, texture := range textures {
		haystack += " " + strings.ToLower(texture)
	}
	for _, signature := range aircraftSignatures {
		for _, token := range signature.Tokens {
			if strings.Contains(haystack, token) {
				return signature.Aircraft
			}
		}
	}
	return ""
}

func naturalLess(a, b string) bool {
	// Numeric table keys sort numerically; non-numeric keys sort lexically.
	ai, aErr := strconv.Atoi(a)
	bi, bErr := strconv.Atoi(b)
	if aErr == nil && bErr == nil {
		return ai < bi
	}
	return a < b
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
