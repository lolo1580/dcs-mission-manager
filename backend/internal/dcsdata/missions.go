package dcsdata

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dcsmanager/internal/lua"
)

// MissionFile is one .miz in the Saved Games mission folders, described by the
// metadata DCS writes inside it. A .miz is a ZIP; "mission" holds the editor
// data and "options" the per-mission options.
type MissionFile struct {
	// Path is the absolute path of the .miz.
	Path string `json:"path"`
	// Name is the file name without its extension.
	Name string `json:"name"`
	// Theatre is the DCS theatre id the mission runs on (Caucasus, Syria…).
	Theatre string `json:"theatre,omitempty"`
	// Date is the in-game date the mission starts ("1988-08-26").
	Date string `json:"date,omitempty"`
	// StartTime is the in-game start time, in seconds since midnight.
	StartTime int `json:"startTime,omitempty"`
	// Weather is a short human description DCS carries ("Summer. Clean sky, no wind").
	Weather string `json:"weather,omitempty"`
	// TemperatureC is the ground temperature DCS records.
	TemperatureC float64 `json:"temperatureC,omitempty"`
	// Version is the mission format version DCS wrote.
	Version int `json:"version,omitempty"`
	// SizeBytes is the file size on disk.
	SizeBytes int64 `json:"sizeBytes"`
	// ModTime is the file's last modification, in unix ms.
	ModTime int64 `json:"modTime"`
}

// MissionsDir returns the Saved Games\Missions folder.
func MissionsDir(savedGames string) string {
	return filepath.Join(savedGames, "Missions")
}

// LoadMissions lists the .miz found under the Saved Games missions folder,
// reading each mission's metadata. Folders DCS uses for saved missions are
// walked; unreadable or malformed archives are skipped rather than failing the
// whole listing.
func LoadMissions(savedGames string) ([]MissionFile, error) {
	root := MissionsDir(savedGames)
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	var out []MissionFile
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip what cannot be read, keep listing the rest
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".miz") {
			return nil
		}
		if m, ok := readMissionFile(path); ok {
			out = append(out, m)
		}
		return nil
	})
	if walkErr != nil {
		return out, walkErr
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].ModTime > out[j].ModTime })
	return out, nil
}

// readMissionFile reads one .miz and returns its metadata. It reports false when
// the archive cannot be read or carries no "mission" entry.
func readMissionFile(path string) (MissionFile, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return MissionFile{}, false
	}
	out := MissionFile{
		Path:      path,
		Name:      strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		SizeBytes: fi.Size(),
		ModTime:   fi.ModTime().UnixMilli(),
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		return out, true // still list it: a broken .miz is worth showing
	}
	defer zr.Close()

	for _, f := range zr.File {
		// The editor data sits at "mission"; localization files live under l10n/.
		if f.Name != "mission" {
			continue
		}
		data, err := readZipEntry(f)
		if err != nil {
			return out, true
		}
		applyMissionMetadata(&out, data)
		return out, true
	}
	return out, true
}

// readZipEntry reads a whole zip entry, bounded so a hostile archive cannot
// exhaust memory (the editor data is well under this in practice).
func readZipEntry(f *zip.File) ([]byte, error) {
	const maxEntry = 32 << 20
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, maxEntry))
}

// applyMissionMetadata parses the "mission" document and fills the fields the
// listing shows. The full file is large and mostly unit tables; only the
// top-level metadata is read, and a parse failure leaves the fields empty rather
// than dropping the mission.
func applyMissionMetadata(out *MissionFile, data []byte) {
	root, err := lua.Parse(data)
	if err != nil {
		return
	}
	m, _ := root["mission"].(map[string]any)
	if m == nil {
		// Some exports omit the "mission" wrapper.
		m = root
	}

	out.Theatre = str(m["theatre"])
	out.StartTime = int(num(m["start_time"]))
	out.Version = int(num(m["version"]))

	if d, ok := m["date"].(map[string]any); ok {
		day, month, year := int(num(d["Day"])), int(num(d["Month"])), int(num(d["Year"]))
		if year > 0 {
			out.Date = formatDate(year, month, day)
		}
	}

	if w, ok := m["weather"].(map[string]any); ok {
		// DCS stores one name per language; prefer English, fall back to French.
		out.Weather = firstNonEmpty(str(w["name"]), str(w["name_fr"]))
		if season, ok := w["season"].(map[string]any); ok {
			out.TemperatureC = num(season["temperature"])
		}
	}
}

// formatDate renders DCS's numeric date fields as YYYY-MM-DD.
func formatDate(year, month, day int) string {
	b := make([]byte, 0, 10)
	b = appendPad(b, year, 4)
	b = append(b, '-')
	b = appendPad(b, month, 2)
	b = append(b, '-')
	b = appendPad(b, day, 2)
	return string(b)
}

// appendPad writes n zero-padded to width digits.
func appendPad(b []byte, n, width int) []byte {
	if n < 0 {
		n = 0
	}
	digits := make([]byte, 0, width)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	for len(digits) < width {
		digits = append([]byte{'0'}, digits...)
	}
	return append(b, digits...)
}
