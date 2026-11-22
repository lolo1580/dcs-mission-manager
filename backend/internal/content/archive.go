package content

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/nwaples/rardecode/v2"
)

const (
	maxArchiveEntries    = 200000
	maxArchiveTotalBytes = int64(16) << 30
	maxArchiveFileBytes  = int64(8) << 30
	maxMetaBytes         = int64(2) << 20
)

// archiveEntry is the format-neutral view of one archive member.
type archiveEntry struct {
	Name      string
	IsDir     bool
	Size      int64
	Mode      fs.FileMode
	Encrypted bool
	Link      bool
}

// archiveFormat identifies the container by magic bytes, not by extension.
func archiveFormat(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	head := make([]byte, 8)
	n, _ := f.Read(head)
	head = head[:n]
	switch {
	case len(head) >= 4 && string(head[:4]) == "PK\x03\x04",
		len(head) >= 4 && string(head[:4]) == "PK\x05\x06",
		len(head) >= 4 && string(head[:4]) == "PK\x07\x08":
		return "zip", nil
	case len(head) >= 6 && string(head[:6]) == "7z\xbc\xaf\x27\x1c":
		return "7z", nil
	case len(head) >= 4 && string(head[:4]) == "Rar!":
		return "rar", nil
	}
	return "", errors.New("format d’archive non reconnu (ZIP, 7z ou RAR attendu)")
}

// listArchive returns every member and rejects obviously hostile archives.
func listArchive(path string) (string, []archiveEntry, error) {
	format, err := archiveFormat(path)
	if err != nil {
		return "", nil, err
	}
	var entries []archiveEntry
	switch format {
	case "zip":
		zr, err := zip.OpenReader(path)
		if err != nil {
			return "", nil, err
		}
		defer zr.Close()
		for _, f := range zr.File {
			entries = append(entries, archiveEntry{
				Name:      f.Name,
				IsDir:     f.FileInfo().IsDir(),
				Size:      int64(f.UncompressedSize64),
				Mode:      f.Mode(),
				Encrypted: f.Flags&0x1 != 0,
				Link:      f.Mode()&os.ModeSymlink != 0,
			})
		}
	case "7z":
		zr, err := sevenzip.OpenReader(path)
		if err != nil {
			return "", nil, normalizeArchiveError(err)
		}
		defer zr.Close()
		for _, f := range zr.File {
			entries = append(entries, archiveEntry{
				Name:  f.Name,
				IsDir: f.FileInfo().IsDir(),
				Size:  int64(f.UncompressedSize),
				Mode:  f.Mode(),
				Link:  f.Mode()&os.ModeSymlink != 0,
			})
		}
	case "rar":
		files, err := rardecode.List(path)
		if err != nil {
			return "", nil, normalizeArchiveError(err)
		}
		for _, f := range files {
			entries = append(entries, archiveEntry{
				Name:      f.Name,
				IsDir:     f.IsDir,
				Size:      f.UnPackedSize,
				Mode:      f.Mode(),
				Encrypted: f.Encrypted || f.HeaderEncrypted,
				Link:      f.LinkType != rardecode.LinkTypeNone,
			})
		}
	}
	if err := validateArchiveEntries(entries); err != nil {
		return "", nil, err
	}
	return format, entries, nil
}

func normalizeArchiveError(err error) error {
	if errors.Is(err, rardecode.ErrArchiveEncrypted) || errors.Is(err, rardecode.ErrArchivedFileEncrypted) {
		return errors.New("archive protégée par mot de passe : non prise en charge")
	}
	var readErr *sevenzip.ReadError
	if errors.As(err, &readErr) && readErr.Encrypted {
		return errors.New("archive protégée par mot de passe : non prise en charge")
	}
	return err
}

func validateArchiveEntries(entries []archiveEntry) error {
	if len(entries) > maxArchiveEntries {
		return fmt.Errorf("archive trop volumineuse : plus de %d fichiers", maxArchiveEntries)
	}
	var total int64
	for _, entry := range entries {
		if entry.Encrypted {
			return errors.New("archive protégée par mot de passe : non prise en charge")
		}
		if entry.Link {
			return fmt.Errorf("lien symbolique refusé dans l’archive : %s", entry.Name)
		}
		if entry.IsDir && isDotArchiveEntry(entry.Name) {
			continue
		}
		if _, err := safeArchiveName(entry.Name); err != nil {
			return err
		}
		total += entry.Size
		if total > maxArchiveTotalBytes {
			return errors.New("archive décompressée trop volumineuse")
		}
	}
	return nil
}

func isDotArchiveEntry(name string) bool {
	clean := strings.Trim(strings.ReplaceAll(name, "\\", "/"), "/")
	return clean == "" || clean == "."
}

// safeArchiveName normalises a member name and rejects absolute paths, parent
// traversal, Windows reserved names and ADS/colon paths.
func safeArchiveName(name string) (string, error) {
	raw := strings.ReplaceAll(name, "\\", "/")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("nom de fichier vide dans l’archive")
	}
	if strings.Contains(raw, "\x00") || strings.Contains(raw, ":") {
		return "", fmt.Errorf("nom de fichier refusé dans l’archive : %q", name)
	}
	clean := path.Clean(raw)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("chemin hors archive refusé : %q", name)
	}
	if strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("chemin absolu refusé dans l’archive : %q", name)
	}
	for _, part := range strings.Split(clean, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("chemin invalide dans l’archive : %q", name)
		}
		if strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return "", fmt.Errorf("nom Windows invalide dans l’archive : %q", name)
		}
		base := strings.ToUpper(part)
		if dot := strings.IndexByte(base, '.'); dot >= 0 {
			base = base[:dot]
		}
		switch base {
		case "CON", "PRN", "AUX", "NUL",
			"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
			"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
			return "", fmt.Errorf("nom réservé Windows refusé dans l’archive : %q", name)
		}
	}
	return clean, nil
}

// selectedEntry reports whether name is a selected file or lives below a
// selected directory prefix.
func selectedEntry(name string, selected []string) bool {
	if len(selected) == 0 {
		return true
	}
	clean := strings.Trim(strings.ReplaceAll(name, "\\", "/"), "/")
	for _, prefix := range selected {
		prefix = strings.Trim(strings.ReplaceAll(prefix, "\\", "/"), "/")
		if prefix == "" {
			continue
		}
		if clean == prefix || strings.HasPrefix(clean, prefix+"/") {
			return true
		}
	}
	return false
}

// readArchiveMeta reads a handful of small metadata files from an archive. It
// works for solid RAR archives too, because RAR is read sequentially.
func readArchiveMeta(path string, wanted map[string]int64) (map[string][]byte, error) {
	format, err := archiveFormat(path)
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	switch format {
	case "zip":
		zr, err := zip.OpenReader(path)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		for _, f := range zr.File {
			limit, ok := wanted[f.Name]
			if !ok {
				continue
			}
			data, err := readLimitedZip(f, limit)
			if err == nil {
				out[f.Name] = data
			}
		}
	case "7z":
		zr, err := sevenzip.OpenReader(path)
		if err != nil {
			return nil, normalizeArchiveError(err)
		}
		defer zr.Close()
		for _, f := range zr.File {
			limit, ok := wanted[f.Name]
			if !ok {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, _ := io.ReadAll(io.LimitReader(rc, limit))
			rc.Close()
			out[f.Name] = data
		}
	case "rar":
		rr, err := rardecode.OpenReader(path)
		if err != nil {
			return nil, normalizeArchiveError(err)
		}
		defer rr.Close()
		for {
			header, err := rr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return out, normalizeArchiveError(err)
			}
			if header.IsDir {
				continue
			}
			limit, ok := wanted[header.Name]
			if ok {
				data, _ := io.ReadAll(io.LimitReader(rr, limit))
				out[header.Name] = data
				continue
			}
			_, _ = io.Copy(io.Discard, rr)
		}
	}
	return out, nil
}

func readLimitedZip(f *zip.File, limit int64) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, limit))
}

// extractArchive extracts only the selected entry prefixes into dest. Everything
// is written under dest; a partial failure leaves a staging directory the caller
// removes.
func extractArchive(path, dest string, selected []string) error {
	format, entries, err := listArchive(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	switch format {
	case "zip":
		zr, err := zip.OpenReader(path)
		if err != nil {
			return err
		}
		defer zr.Close()
		for _, f := range zr.File {
			if !selectedEntry(f.Name, selected) || f.FileInfo().IsDir() {
				continue
			}
			if err := extractZipFile(f, dest); err != nil {
				return err
			}
		}
	case "7z":
		zr, err := sevenzip.OpenReader(path)
		if err != nil {
			return normalizeArchiveError(err)
		}
		defer zr.Close()
		for _, f := range zr.File {
			if !selectedEntry(f.Name, selected) || f.FileInfo().IsDir() {
				continue
			}
			if err := extract7zFile(f, dest); err != nil {
				return err
			}
		}
	case "rar":
		rr, err := rardecode.OpenReader(path)
		if err != nil {
			return normalizeArchiveError(err)
		}
		defer rr.Close()
		for {
			header, err := rr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return normalizeArchiveError(err)
			}
			if header.IsDir || !selectedEntry(header.Name, selected) {
				_, _ = io.Copy(io.Discard, rr)
				continue
			}
			if err := extractRARFile(rr, header, dest); err != nil {
				return err
			}
		}
	}
	return validateExtracted(dest, entries, selected)
}

func extractZipFile(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	return writeArchiveFile(dest, f.Name, f.UncompressedSize64, rc)
}

func extract7zFile(f *sevenzip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return normalizeArchiveError(err)
	}
	defer rc.Close()
	return writeArchiveFile(dest, f.Name, f.UncompressedSize, rc)
}

func extractRARFile(r io.Reader, header *rardecode.FileHeader, dest string) error {
	if header.UnPackedSize > maxArchiveFileBytes {
		return fmt.Errorf("fichier trop volumineux dans l’archive : %s", header.Name)
	}
	size := header.UnPackedSize
	if size < 0 {
		size = 0
	}
	return writeArchiveFile(dest, header.Name, uint64(size), r)
}

func writeArchiveFile(dest, name string, size uint64, r io.Reader) error {
	rel, err := safeArchiveName(name)
	if err != nil {
		return err
	}
	if int64(size) > maxArchiveFileBytes {
		return fmt.Errorf("fichier trop volumineux dans l’archive : %s", name)
	}
	target, err := safeJoin(dest, rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(out, io.LimitReader(r, maxArchiveFileBytes+1))
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(target)
		return copyErr
	}
	if n > maxArchiveFileBytes {
		_ = os.Remove(target)
		return fmt.Errorf("fichier trop volumineux dans l’archive : %s", name)
	}
	if closeErr != nil {
		_ = os.Remove(target)
		return closeErr
	}
	return nil
}

func validateExtracted(dest string, entries []archiveEntry, selected []string) error {
	var total int64
	for _, entry := range entries {
		if !selectedEntry(entry.Name, selected) || entry.IsDir {
			continue
		}
		total += entry.Size
		if total > maxArchiveTotalBytes {
			return errors.New("archive décompressée trop volumineuse")
		}
	}
	// The real proof is what is on disk. This also catches a decoder that wrote
	// less than the header announced.
	return filepath.WalkDir(dest, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxArchiveFileBytes {
			return errors.New("fichier décompressé trop volumineux")
		}
		return nil
	})
}
