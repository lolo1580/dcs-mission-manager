package content

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"dcsmanager/internal/dcsdata"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testService(t *testing.T) (*Service, string) {
	t.Helper()
	sg := t.TempDir()
	svc := New(sg, t.TempDir())
	svc.SetDCSRunning(func() (bool, error) { return false, nil })
	return svc, sg
}

func TestModActionsAndQuarantine(t *testing.T) {
	svc, sg := testService(t)
	writeFile(t, filepath.Join(sg, "Mods", "aircraft", "A-4E-C", "entry.lua"), `declare_plugin("A-4E-C")`)
	writeFile(t, filepath.Join(sg, "Mods.disabled", "tech", "B", "entry.lua"), `declare_plugin("B")`)

	if _, err := svc.ActionMod(ModActionRequest{Action: "disable", Category: "aircraft", Name: "A-4E-C"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(sg, "Mods.disabled", "aircraft", "A-4E-C", "entry.lua")); err != nil {
		t.Fatal("mod was not disabled", err)
	}
	if _, err := svc.ActionMod(ModActionRequest{Action: "enable", Category: "aircraft", Name: "A-4E-C"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(sg, "Mods", "aircraft", "A-4E-C", "entry.lua")); err != nil {
		t.Fatal("mod was not enabled", err)
	}

	result, err := svc.ActionMod(ModActionRequest{Action: "quarantine", Category: "aircraft", Name: "A-4E-C"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ID == "" {
		t.Fatal("quarantine ID is empty")
	}
	if _, err := os.Stat(filepath.Join(sg, "Mods", "aircraft", "A-4E-C")); !os.IsNotExist(err) {
		t.Fatal("mod still active after quarantine")
	}
	items, err := svc.Quarantine()
	if err != nil || len(items) != 1 || items[0].Kind != "mod" {
		t.Fatalf("quarantine = %+v, %v", items, err)
	}
	if _, err := svc.ActionMod(ModActionRequest{Action: "restore", QuarantineID: result.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(sg, "Mods", "aircraft", "A-4E-C", "entry.lua")); err != nil {
		t.Fatal("mod was not restored", err)
	}
}

func TestModActionRefusedWhileDCSRuns(t *testing.T) {
	svc, sg := testService(t)
	writeFile(t, filepath.Join(sg, "Mods", "aircraft", "A", "entry.lua"), "x")
	svc.SetDCSRunning(func() (bool, error) { return true, nil })
	_, err := svc.ActionMod(ModActionRequest{Action: "disable", Category: "aircraft", Name: "A"})
	if !errors.Is(err, ErrDCSRunning) {
		t.Fatalf("err = %v, want ErrDCSRunning", err)
	}
}

func TestClassifyFlatLiveryWithDependency(t *testing.T) {
	svc, sg := testService(t)
	writeFile(t, filepath.Join(sg, "Liveries", "J-3080", "description.lua"), `
name = "J-3080"
livery = {
  {"nose", 0, "../J-3XXX PS/PS_nose", false},
}
`)
	writeFile(t, filepath.Join(sg, "Liveries", "J-3080", "other.dds"), "x")
	writeFile(t, filepath.Join(sg, "Liveries", "J-3XXX PS", "description.lua"), `name = "Shared"`)
	writeFile(t, filepath.Join(sg, "Liveries", "J-3XXX PS", "PS_nose.dds"), "x")

	scan, err := dcsdata.ScanLiveries(sg)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, livery := range scan.Liveries {
		ids[livery.Name] = livery.ID
	}
	if _, err := svc.ActionLivery(LiveryActionRequest{Action: "classify", IDs: []string{ids["J-3080"]}, Aircraft: "F-5E-3"}); err == nil {
		t.Fatal("classify should refuse to leave a dependency behind")
	}
	if _, err := svc.ActionLivery(LiveryActionRequest{
		Action:   "classify",
		IDs:      []string{ids["J-3080"], ids["Shared"]},
		Aircraft: "F-5E-3",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(sg, "Liveries", "F-5E-3", "J-3080", "description.lua")); err != nil {
		t.Fatal("livery was not classified", err)
	}
	if _, err := os.Stat(filepath.Join(sg, "Liveries", "F-5E-3", "J-3XXX PS", "PS_nose.dds")); err != nil {
		t.Fatal("dependency was not classified", err)
	}
}

func TestClassifyPreflightLeavesFilesInPlace(t *testing.T) {
	svc, sg := testService(t)
	for _, name := range []string{"A", "B"} {
		writeFile(t, filepath.Join(sg, "Liveries", name, "description.lua"), `name = "`+name+`"`)
	}
	// The target for B already exists, so neither A nor B may move.
	writeFile(t, filepath.Join(sg, "Liveries", "F-5E-3", "B", "description.lua"), `name = "Existing"`)
	scan, err := dcsdata.ScanLiveries(sg)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, livery := range scan.Liveries {
		ids[livery.Name] = livery.ID
	}
	if _, err := svc.ActionLivery(LiveryActionRequest{
		Action:   "classify",
		IDs:      []string{ids["A"], ids["B"]},
		Aircraft: "F-5E-3",
	}); err == nil {
		t.Fatal("classify should fail on an existing target")
	}
	if _, err := os.Stat(filepath.Join(sg, "Liveries", "A", "description.lua")); err != nil {
		t.Fatal("A was moved despite the failed preflight")
	}
	if _, err := os.Stat(filepath.Join(sg, "Liveries", "F-5E-3", "A")); !os.IsNotExist(err) {
		t.Fatal("A target was created")
	}
}

func TestLiveryQuarantineNeedsForceAndRestores(t *testing.T) {
	svc, sg := testService(t)
	writeFile(t, filepath.Join(sg, "Liveries", "A", "description.lua"), `
name = "A"
livery = {
  {"x", 0, "../B/tex", false},
}
`)
	writeFile(t, filepath.Join(sg, "Liveries", "A", "a.dds"), "x")
	writeFile(t, filepath.Join(sg, "Liveries", "B", "description.lua"), `name = "B"`)
	writeFile(t, filepath.Join(sg, "Liveries", "B", "tex.dds"), "x")
	scan, err := dcsdata.ScanLiveries(sg)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, livery := range scan.Liveries {
		if livery.Name == "A" {
			id = livery.ID
		}
	}
	if _, err := svc.ActionLivery(LiveryActionRequest{Action: "quarantine", ID: id}); err == nil {
		t.Fatal("quarantine should require force when other liveries depend on this one")
	}
	result, err := svc.ActionLivery(LiveryActionRequest{Action: "quarantine", ID: id, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	items, err := svc.Quarantine()
	if err != nil || len(items) != 1 {
		t.Fatalf("quarantine = %+v, %v", items, err)
	}
	if _, err := svc.ActionLivery(LiveryActionRequest{Action: "restore", QuarantineID: result.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(sg, "Liveries", "A", "description.lua")); err != nil {
		t.Fatal("livery was not restored", err)
	}
}

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestZIPImportInspectCommitAndReplace(t *testing.T) {
	svc, sg := testService(t)
	archive := makeZip(t, map[string]string{
		"Mods/aircraft/TestMod/entry.lua": `declare_plugin("TestMod")`,
		"Mods/aircraft/TestMod/text.txt":  "one",
	})
	preview, err := svc.InspectUpload(bytes.NewReader(archive), "test.zip")
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Items) != 1 || preview.Items[0].Kind != "mod" {
		t.Fatalf("preview = %+v", preview)
	}
	if preview.Items[0].Category != "aircraft" || preview.Items[0].Name != "TestMod" {
		t.Fatalf("item = %+v", preview.Items[0])
	}
	if _, err := svc.CommitImport(preview.ID, []ImportSelection{{ID: preview.Items[0].ID}}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(sg, "Mods", "aircraft", "TestMod", "text.txt")); err != nil {
		t.Fatal("mod was not installed", err)
	}

	archive2 := makeZip(t, map[string]string{
		"Mods/aircraft/TestMod/entry.lua": `declare_plugin("TestMod")`,
		"Mods/aircraft/TestMod/text.txt":  "two",
	})
	preview2, err := svc.InspectUpload(bytes.NewReader(archive2), "test.zip")
	if err != nil {
		t.Fatal(err)
	}
	if len(preview2.Items[0].Conflicts) == 0 {
		t.Fatal("preview should report the existing target")
	}
	if _, err := svc.CommitImport(preview2.ID, []ImportSelection{{ID: preview2.Items[0].ID}}, false); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
	if _, err := svc.CommitImport(preview2.ID, []ImportSelection{{ID: preview2.Items[0].ID}}, true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(sg, "Mods", "aircraft", "TestMod", "text.txt"))
	if err != nil || string(data) != "two" {
		t.Fatalf("replaced mod content = %q, %v", data, err)
	}
	items, err := svc.Quarantine()
	if err != nil || len(items) != 1 {
		t.Fatalf("replacement should quarantine the old mod: %+v, %v", items, err)
	}
}

func TestZIPImportRejectsTraversal(t *testing.T) {
	svc, _ := testService(t)
	archive := makeZip(t, map[string]string{
		"../evil.txt": "x",
	})
	if _, err := svc.InspectUpload(bytes.NewReader(archive), "evil.zip"); err == nil {
		t.Fatal("traversal archive was accepted")
	}
}

func TestSafeJoinAndElement(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{".", "..", "../x", "a/../../x"} {
		if _, err := safeJoin(root, rel); err == nil {
			t.Errorf("safeJoin accepted %q", rel)
		}
	}
	for _, name := range []string{"CON", "nul.txt", "a:b", "a?b", "..", ""} {
		if err := safeElement(name); err == nil {
			t.Errorf("safeElement accepted %q", name)
		}
	}
	for _, name := range []string{"F-16C_50", "A-4E-C", "Tacview"} {
		if err := safeElement(name); err != nil {
			t.Errorf("safeElement rejected %q: %v", name, err)
		}
	}
}

func TestCommitImportRejectsDuplicateTargets(t *testing.T) {
	svc, sg := testService(t)
	archive := makeZip(t, map[string]string{
		"Mods/aircraft/A/entry.lua": "a",
		"Mods/tech/B/entry.lua":     "b",
	})
	preview, err := svc.InspectUpload(bytes.NewReader(archive), "two.zip")
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Items) != 2 {
		t.Fatalf("items = %+v", preview.Items)
	}
	selections := []ImportSelection{
		{ID: preview.Items[0].ID, Category: "aircraft", Name: "Same"},
		{ID: preview.Items[1].ID, Category: "aircraft", Name: "Same"},
	}
	if _, err := svc.CommitImport(preview.ID, selections, false); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
	if _, err := os.Stat(filepath.Join(sg, "Mods", "aircraft", "Same")); !os.IsNotExist(err) {
		t.Fatal("duplicate-target import wrote a target")
	}
}

func TestCancelImportRemovesStaging(t *testing.T) {
	svc, _ := testService(t)
	archive := makeZip(t, map[string]string{
		"Mods/aircraft/Cancel/entry.lua": "x",
	})
	preview, err := svc.InspectUpload(bytes.NewReader(archive), "cancel.zip")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(svc.importRoot(), preview.ID)
	if _, err := os.Stat(dir); err != nil {
		t.Fatal("import staging missing")
	}
	if err := svc.CancelImport(preview.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("import staging was not removed")
	}
}

func Test7zImportInspect(t *testing.T) {
	svc, _ := testService(t)
	f, err := os.Open(filepath.Join("testdata", "testmod.7z"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	preview, err := svc.InspectUpload(f, "testmod.7z")
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Items) != 1 || preview.Items[0].Kind != "mod" || preview.Items[0].Name != "Test7z" {
		t.Fatalf("preview = %+v", preview)
	}
	if preview.Items[0].Category != "aircraft" {
		t.Fatalf("category = %q", preview.Items[0].Category)
	}
}
