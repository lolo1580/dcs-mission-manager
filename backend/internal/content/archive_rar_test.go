package content

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRAR3AndRAR5Extraction(t *testing.T) {
	for _, name := range []string{"testfile.rar3.rar", "testfile.rar5.rar", "testfile.rar5.solid.rar"} {
		src := filepath.Join("testdata", name)
		format, entries, err := listArchive(src)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if format != "rar" || len(entries) == 0 {
			t.Fatalf("%s: format=%q entries=%d", name, format, len(entries))
		}
		dest := t.TempDir()
		if err := extractArchive(src, dest, nil); err != nil {
			t.Fatalf("%s extract: %v", name, err)
		}
		count := 0
		_ = filepath.Walk(dest, func(_ string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				count++
			}
			return nil
		})
		if count == 0 {
			t.Fatalf("%s: no file extracted", name)
		}
	}
}
