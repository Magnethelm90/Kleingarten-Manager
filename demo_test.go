package main

import (
	"os"
	"strings"
	"testing"
)

func TestBeispieldaten(t *testing.T) {
	app, h := newTestApp(t)
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if !st.Leer || st.Demo {
		t.Fatalf("frischer Datenbestand: leer=%v demo=%v", st.Leer, st.Demo)
	}
	if rec := do(h, "DELETE", "/api/admin/beispieldaten", nil); rec.Code != 400 {
		t.Errorf("Entfernen ohne Beispieldaten: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/admin/beispieldaten", nil); rec.Code != 200 {
		t.Fatalf("Laden: %d %s", rec.Code, rec.Body.String())
	}
	st = decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if !st.Demo || st.Leer || len(st.Paechter) != len(demoListe) {
		t.Fatalf("nach dem Laden: demo=%v leer=%v %d Pächter", st.Demo, st.Leer, len(st.Paechter))
	}
	nrs, vollst := map[string]bool{}, 0
	for _, p := range st.Paechter {
		if nrs[p.Mitgliedsnr] {
			t.Errorf("Mitgliedsnummer doppelt: %s", p.Mitgliedsnr)
		}
		nrs[p.Mitgliedsnr] = true
		if _, err := cleanPaechter(p); err != nil {
			t.Errorf("Beispiel-Pächter ungültig: %+v: %v", p, err)
		}
		r := st.Results[p.ID]
		if r.Vollstaendig {
			vollst++
			if r.Gesamt <= 0 || r.Gesamt > 500 {
				t.Errorf("unplausibler Beispielbetrag %s: %v", p.Name, r.Gesamt)
			}
		} else if r.Status != StatusFehlt {
			t.Errorf("%s: %s", p.Name, r.Status)
		}
	}
	if vollst != len(demoListe)-2 {
		t.Errorf("%d vollständige Beispiel-Pächter, erwartet %d (zwei zeigen fehlende Angaben)", vollst, len(demoListe)-2)
	}
	// ein zweites Mal geht nicht, und in eigene Daten schon gar nicht
	if rec := do(h, "POST", "/api/admin/beispieldaten", nil); rec.Code != 400 {
		t.Errorf("zweites Laden: %d", rec.Code)
	}

	// ausprobieren: Rechnungen ausstellen, Passwort setzen
	do(h, "POST", "/api/invoices/issue", issueReq{})
	app.st.mu.Lock()
	var datei string
	if len(app.st.d.Rechnungen) > 0 {
		datei = app.st.d.Rechnungen[0].Datei
	}
	app.st.mu.Unlock()
	full, err := app.rechnungsDatei(datei)
	if err != nil {
		t.Fatalf("keine Rechnung ausgestellt: %v", err)
	}
	if _, err := os.Stat(full); err != nil {
		t.Fatal(err)
	}
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	if rec := do(h, "DELETE", "/api/admin/beispieldaten", nil); rec.Code != 401 {
		t.Errorf("Entfernen ohne Anmeldung: %d", rec.Code)
	}
	c := do(h, "POST", "/api/admin/login", map[string]string{"password": "geheim123"}).Result().Cookies()[0]

	if rec := do(h, "DELETE", "/api/admin/beispieldaten", nil, c); rec.Code != 200 {
		t.Fatalf("Entfernen: %d %s", rec.Code, rec.Body.String())
	}
	st = decode[stateResp](t, do(h, "GET", "/api/state", nil, c))
	if st.Demo || !st.Leer || len(st.Paechter) != 0 || len(st.Archive) != 0 || strings.Contains(st.Settings.VereinName, "Sonnenschein") {
		t.Fatalf("nach dem Entfernen: demo=%v leer=%v %d Pächter, %d Rechnungen, %q", st.Demo, st.Leer, len(st.Paechter), len(st.Archive), st.Settings.VereinName)
	}
	if !st.HasPassword || !st.LoggedIn {
		t.Error("Admin-Passwort muss beim Entfernen der Beispieldaten erhalten bleiben")
	}
	if _, err := os.Stat(full); err == nil {
		t.Error("Rechnungsdatei aus den Beispieldaten wurde nicht gelöscht")
	}
	// danach lassen sich die Beispieldaten erneut laden
	if rec := do(h, "POST", "/api/admin/beispieldaten", nil, c); rec.Code != 200 {
		t.Errorf("erneutes Laden: %d", rec.Code)
	}
}

func TestBeispieldatenNichtInEigeneDaten(t *testing.T) {
	_, h := newTestApp(t)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Echt", "gartengroesse": 100})
	if rec := do(h, "POST", "/api/admin/beispieldaten", nil); rec.Code != 400 {
		t.Errorf("Beispieldaten in vorhandene Daten: %d", rec.Code)
	}
	if st := decode[stateResp](t, do(h, "GET", "/api/state", nil)); st.Leer {
		t.Error("Datenbestand mit Pächter gilt als leer")
	}
}
