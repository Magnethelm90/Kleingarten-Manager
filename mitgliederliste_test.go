package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestMitgliederlisteSeiten(t *testing.T) {
	s := defaultSettings()
	mk := func(n int) []Paechter {
		ps := make([]Paechter, n)
		for i := range ps {
			ps[i] = Paechter{Gartennr: fmt.Sprint(n - i), Mitgliedsnr: fmt.Sprint(i), Name: "Pächter mit einem ziemlich langen Namen, der gekürzt werden muss", Strasse: "Gartenweg 1", PLZOrt: "12345 Ort", Gartengroesse: 300}
		}
		return ps
	}
	for _, c := range []struct{ n, seiten int }{{1, 1}, {22, 1}, {23, 2}, {44, 2}, {45, 3}} {
		pdf, err := buildMitgliederliste(s, mk(c.n))
		if err != nil {
			t.Fatal(err)
		}
		if got := ablesebogenSeiten(pdf); got != c.seiten {
			t.Errorf("%d Pächter: %d Seiten, erwartet %d", c.n, got, c.seiten)
		}
	}
	if _, err := buildMitgliederliste(s, nil); err == nil {
		t.Error("leere Liste sollte abgelehnt werden")
	}
}

func TestMitgliederlisteApi(t *testing.T) {
	app, h := newTestApp(t)
	var geoeffnet []string
	orig := openPathFn
	openPathFn = func(p string) { geoeffnet = append(geoeffnet, p) }
	t.Cleanup(func() { openPathFn = orig })

	if rec := do(h, "GET", "/api/admin/paechterliste", nil); rec.Code != 400 {
		t.Errorf("ohne Pächter: %d, erwartet 400", rec.Code)
	}
	id := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Anna", "gartengroesse": 100})).ID
	rec := do(h, "GET", "/api/admin/paechterliste", nil)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("Liste: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	// Pächter im Papierkorb stehen nicht auf der Liste
	do(h, "DELETE", "/api/admin/paechter/"+id, nil)
	if rec := do(h, "GET", "/api/admin/paechterliste", nil); rec.Code != 400 {
		t.Errorf("nur gelöschte Pächter: %d, erwartet 400", rec.Code)
	}
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "2", "name": "Berta", "gartengroesse": 100})
	if rec := do(h, "POST", "/api/admin/paechterliste/oeffnen", nil); rec.Code != 200 {
		t.Fatalf("Öffnen: %d", rec.Code)
	}
	if len(geoeffnet) != 1 || filepath.Dir(geoeffnet[0]) != filepath.Join(app.st.dir, druckOrdner) {
		t.Fatalf("geöffnet: %v", geoeffnet)
	}
	if _, err := os.Stat(geoeffnet[0]); err != nil {
		t.Errorf("Datei fehlt: %v", err)
	}
	// Die Liste enthält Anschriften: mit Admin-Passwort darf sie nur Admins ausliefern
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	for _, path := range []string{"/api/admin/paechterliste"} {
		if rec := do(h, "GET", path, nil); rec.Code != 401 {
			t.Errorf("%s ohne Anmeldung: %d, erwartet 401", path, rec.Code)
		}
	}
	if rec := do(h, "POST", "/api/admin/paechterliste/oeffnen", nil); rec.Code != 401 {
		t.Errorf("Öffnen ohne Anmeldung: %d, erwartet 401", rec.Code)
	}
}
