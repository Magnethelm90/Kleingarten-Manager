package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVereinsakte(t *testing.T) {
	app, h := newTestApp(t)
	var gezeigt []string
	alt := revealPathFn
	revealPathFn = func(p string) { gezeigt = append(gezeigt, p) }
	defer func() { revealPathFn = alt }()
	do(h, "POST", "/api/admin/beispieldaten", nil)
	do(h, "POST", "/api/invoices/issue", issueReq{})
	// Dateien, die nicht hineingehören
	_ = os.WriteFile(filepath.Join(app.st.dir, sperrDateiName), []byte(`{"token":"x"}`), 0o600)
	_ = os.MkdirAll(filepath.Join(app.st.dir, druckOrdner), 0o700)
	_ = os.WriteFile(filepath.Join(app.st.dir, druckOrdner, "alt.pdf"), []byte("%PDF"), 0o600)

	rec := do(h, "POST", "/api/admin/vereinsakte", nil)
	if rec.Code != 200 || len(gezeigt) != 1 {
		t.Fatalf("Vereinsakte: %d %s", rec.Code, rec.Body.String())
	}
	zr, err := zip.OpenReader(gezeigt[0])
	if err != nil {
		t.Fatal(err)
	}
	namen := map[string]bool{}
	rechnungen := 0
	for _, f := range zr.File {
		namen[f.Name] = true
		if dir := os.Getenv("GA_PDF_DIR"); dir != "" && f.Name == "Lies mich zuerst.pdf" {
			if r, err := f.Open(); err == nil {
				raw := make([]byte, f.UncompressedSize64)
				_, _ = io.ReadFull(r, raw)
				_ = os.WriteFile(dir+"/liesmich.pdf", raw, 0o600)
				r.Close()
			}
		}
		if strings.HasPrefix(f.Name, "Daten/Rechnungen/") {
			rechnungen++
		}
		if strings.Contains(f.Name, "..") || strings.HasPrefix(f.Name, "/") {
			t.Errorf("unsicherer Pfad im ZIP: %s", f.Name)
		}
	}
	zr.Close()
	if !namen["Lies mich zuerst.pdf"] || !namen["Daten/"+dataFileName] || rechnungen == 0 {
		t.Errorf("Inhalt unvollständig: %v", namen)
	}
	for n := range namen {
		if strings.Contains(n, "Sicherungen/") || strings.Contains(n, druckOrdner+"/") || strings.HasSuffix(n, sperrDateiName) || strings.Contains(n, vereinsakteOrdner+"/") {
			t.Errorf("gehört nicht in die Akte: %s", n)
		}
	}
	// zweite Akte ersetzt die erste; endgültiges Löschen einer Person entfernt alle
	do(h, "POST", "/api/admin/vereinsakte", nil)
	akten, _ := os.ReadDir(filepath.Join(app.st.dir, vereinsakteOrdner))
	if len(akten) != 1 {
		t.Errorf("%d Akten statt einer", len(akten))
	}
	app.st.entferneVereinsakten()
	if akten, _ := os.ReadDir(filepath.Join(app.st.dir, vereinsakteOrdner)); len(akten) != 0 {
		t.Errorf("Akten nicht entfernt: %d", len(akten))
	}
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	if rec := do(h, "POST", "/api/admin/vereinsakte", nil); rec.Code != 401 {
		t.Errorf("ohne Anmeldung: %d", rec.Code)
	}
}
