package content

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// safeJoin resolves rel under root and refuses anything that escapes it or names
// the root itself.
func safeJoin(root, rel string) (string, error) {
	root = filepath.Clean(root)
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", errors.New("chemin vide")
	}
	clean := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
	if clean == root {
		return "", errors.New("la racine elle-même n’est pas une cible valide")
	}
	if !strings.HasPrefix(clean, root+string(os.PathSeparator)) {
		return "", errors.New("chemin hors de la racine autorisée")
	}
	return clean, nil
}

// safeElement validates a single path element supplied by the client.
func safeElement(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return errors.New("nom invalide")
	}
	if strings.ContainsAny(name, `/\:*?"<>|`) {
		return errors.New("nom invalide")
	}
	if strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") {
		return errors.New("nom Windows invalide")
	}
	base := strings.ToUpper(name)
	if dot := strings.IndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	switch base {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return errors.New("nom réservé Windows")
	}
	return nil
}

// moveDir moves a directory, falling back to a copy when source and target are
// on different volumes. A target that already exists is never overwritten.
func moveDir(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return errors.New("la cible existe déjà")
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyDir(src, dst); err != nil {
		_ = os.RemoveAll(dst)
		return err
	}
	return os.RemoveAll(src)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("lien symbolique refusé: " + path)
		}
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}

// newID returns a timestamp-prefixed random identifier used for quarantine and
// import staging folders.
func newID() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "item"
	}
	return hex.EncodeToString(buf)
}

type quarantineManifest struct {
	Version       int    `json:"version"`
	Kind          string `json:"kind"` // mod or livery
	Category      string `json:"category,omitempty"`
	Name          string `json:"name,omitempty"`
	Aircraft      string `json:"aircraft,omitempty"`
	RelPath       string `json:"relPath,omitempty"`
	OriginalState string `json:"originalState,omitempty"` // active or disabled
	CreatedAt     string `json:"createdAt"`
}
