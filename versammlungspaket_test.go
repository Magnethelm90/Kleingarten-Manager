package main

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestVersammlungspaket(t *testing.T) {
	_, h := newTestApp(t)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Anna Paket", "gartennr": "1", "gartengroesse": 100})
	ver := decode[Termin](t, do(h, "POST", "/api/admin/termine", map[string]any{"datum": "2026-03-14", "titel": "Jahreshauptversammlung", "art": "Versammlung", "von": "10:00"}))
	fest := decode[Termin](t, do(h, "POST", "/api/admin/termine", map[string]any{"datum": "2026-06-14", "titel": "Sommerfest", "art": "Fest"}))

	if rec := do(h, "GET", "/api/admin/termine/"+fest.ID+"/paket", nil); rec.Code != 400 {
		t.Errorf("Fest: %d", rec.Code)
	}
	if rec := do(h, "GET", "/api/admin/termine/gibtesnicht/paket", nil); rec.Code != 404 {
		t.Errorf("unbekannter Termin: %d", rec.Code)
	}
	rec := do(h, "GET", "/api/admin/termine/"+ver.ID+"/paket", nil)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("Paket: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 4 { // ohne erfasste Kassenprüfung kein Prüfbericht
		t.Fatalf("%d Dateien statt 4", len(zr.File))
	}
	for _, f := range zr.File {
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		_ = r.Close()
		if !strings.HasSuffix(f.Name, ".pdf") || !bytes.HasPrefix(b, []byte("%PDF")) {
			t.Errorf("%s ist kein PDF", f.Name)
		}
	}
	// mit Kassenprüfung kommt der Prüfbericht dazu
	a, h2 := newTestApp(t)
	do(h2, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Anna Paket", "gartennr": "1", "gartengroesse": 100})
	v2 := decode[Termin](t, do(h2, "POST", "/api/admin/termine", map[string]any{"datum": "2026-03-14", "titel": "JHV", "art": "Versammlung"}))
	a.st.mu.Lock()
	a.st.d.Kassenpruefungen = map[string]Kassenpruefung{yearKey(a.st.d.Settings.Jahr): {}}
	a.st.mu.Unlock()
	rec = do(h2, "GET", "/api/admin/termine/"+v2.ID+"/paket", nil)
	zr, _ = zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if rec.Code != 200 || len(zr.File) != 5 {
		t.Errorf("mit Prüfung: %d, %d Dateien", rec.Code, len(zr.File))
	}
}
