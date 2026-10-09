package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func ablesebogenSeiten(pdf []byte) int { return len(seitenRegexp.FindAll(pdf, -1)) }

func TestAblesebogenSeitenUndSortierung(t *testing.T) {
	app, h := newTestApp(t)
	for _, g := range []string{"10", "2", "1"} {
		do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "M" + g, "gartennr": g, "name": "Pächter " + g, "wasserzaehlerNr": "W" + g, "gartengroesse": 100})
	}
	app.st.mu.Lock()
	rows, _, ok := app.st.ablesezeilenLocked(2025)
	app.st.mu.Unlock()
	if !ok || len(rows) != 3 || rows[0].Gartennr != "1" || rows[1].Gartennr != "2" || rows[2].Gartennr != "10" {
		t.Fatalf("Zeilen nach Gartennummer sortiert erwartet, bekommen: %+v", rows)
	}

	rec := do(h, "GET", "/api/ablesebogen?year=2025", nil)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("Ablesebogen: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if n := ablesebogenSeiten(rec.Body.Bytes()); n != 1 {
		t.Errorf("3 Gärten passen auf 1 Seite, Seiten: %d", n)
	}
	if rec := do(h, "GET", "/api/ablesebogen?year=1999", nil); rec.Code != 404 {
		t.Errorf("unbekanntes Jahr: %d, erwartet 404", rec.Code)
	}
}

func TestAblesebogenBlaettertUm(t *testing.T) {
	s := defaultSettings()
	mk := func(n int) []ablesezeile {
		rows := make([]ablesezeile, n)
		for i := range rows {
			rows[i] = ablesezeile{Gartennr: fmt.Sprint(i + 1), Name: "Pächter mit einem ziemlich langen Namen, der nicht in die Spalte passt und gekürzt werden muss", WasserVJ: fp(10)}
		}
		return rows
	}
	for _, c := range []struct{ rows, seiten int }{{1, 1}, {17, 1}, {18, 2}, {34, 2}, {35, 3}, {120, 8}} {
		pdf, err := buildAblesebogen(s, 2025, mk(c.rows))
		if err != nil {
			t.Fatal(err)
		}
		if n := ablesebogenSeiten(pdf); n != c.seiten {
			t.Errorf("%d Zeilen: %d Seiten, erwartet %d", c.rows, n, c.seiten)
		}
	}
	if _, err := buildAblesebogen(s, 2025, nil); err == nil {
		t.Error("ohne Pächter muss es eine verständliche Fehlermeldung geben")
	}
}

func TestAblesebogenOeffnen(t *testing.T) {
	app, h := newTestApp(t)
	var geoeffnet []string
	orig := openPathFn
	openPathFn = func(p string) { geoeffnet = append(geoeffnet, p) }
	t.Cleanup(func() { openPathFn = orig })

	if rec := do(h, "POST", "/api/ablesebogen/oeffnen?year=2025", nil); rec.Code != 400 {
		t.Errorf("ohne Pächter: %d, erwartet 400", rec.Code)
	}
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "A", "gartengroesse": 100})
	if rec := do(h, "POST", "/api/ablesebogen/oeffnen?year=2025", nil); rec.Code != 200 {
		t.Fatalf("öffnen: %d %s", rec.Code, rec.Body)
	}
	want := filepath.Join(app.st.dir, "Druck", "Ablesebogen_2025.pdf")
	if len(geoeffnet) != 1 || geoeffnet[0] != want {
		t.Fatalf("geöffnet: %v, erwartet %s", geoeffnet, want)
	}
	if fi, err := os.Stat(want); err != nil || fi.Size() == 0 {
		t.Errorf("Datei fehlt: %v", err)
	}
}
