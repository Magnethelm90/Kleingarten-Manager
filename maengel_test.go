package main

import (
	"bytes"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMaengelAblauf(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	liste := decode[[]mangelAnsicht](t, do(h, "GET", "/api/admin/maengel", nil))
	if len(liste) != 4 {
		t.Fatalf("Demo-Mängel: %d", len(liste))
	}
	heute := time.Now().Format("2006-01-02")
	// ungültige Eingaben
	for name, body := range map[string]map[string]any{
		"ohne Beschreibung": {"gartennr": "3", "datum": heute, "beschreibung": "  "},
		"ohne Garten":       {"datum": heute, "beschreibung": "x"},
		"falsches Datum":    {"gartennr": "3", "datum": "gestern", "beschreibung": "x"},
		"falsche Frist":     {"gartennr": "3", "datum": heute, "beschreibung": "x", "frist": "bald"},
		"freier Garten":     {"gartennr": "99", "datum": heute, "beschreibung": "x"},
	} {
		if rec := do(h, "POST", "/api/admin/maengel", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	rec := do(h, "POST", "/api/admin/maengel", map[string]any{"gartennr": "4", "datum": heute, "art": "Unbekannt", "beschreibung": "Kompost\nüberquillt  "})
	if rec.Code != 200 {
		t.Fatalf("anlegen: %d %s", rec.Code, rec.Body.String())
	}
	m := decode[Mangel](t, rec)
	if m.Art != "Sonstiges" || m.Beschreibung != "Kompost überquillt" || m.PaechterID == "" {
		t.Errorf("Mangel: %+v", m)
	}
	// erledigen
	m.ErledigtAm = heute
	if rec := do(h, "PUT", "/api/admin/maengel/"+m.ID, m); rec.Code != 200 {
		t.Fatalf("erledigen: %d", rec.Code)
	}
	if rec := do(h, "PUT", "/api/admin/maengel/gibtesnicht", m); rec.Code != 404 {
		t.Errorf("unbekannt: %d", rec.Code)
	}

	// PDFs
	dir := os.Getenv("GA_PDF_DIR")
	for name, pfad := range map[string]string{"begehung": "/api/admin/begehungsliste", "schreiben": "/api/admin/maengelschreiben?gartennr=3"} {
		rec := do(h, "GET", pfad, nil)
		if rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
		if dir != "" {
			_ = os.WriteFile(dir+"/maengel_"+name+".pdf", rec.Body.Bytes(), 0o600)
		}
	}
	if rec := do(h, "GET", "/api/admin/maengelschreiben?gartennr=4", nil); rec.Code != 400 {
		t.Errorf("Schreiben ohne offene Mängel: %d", rec.Code)
	}
	if rec := do(h, "GET", "/api/admin/maengelschreiben?gartennr=99", nil); rec.Code != 400 {
		t.Errorf("Schreiben für freien Garten: %d", rec.Code)
	}
	if rec := do(h, "GET", "/api/admin/maengelschreiben", nil); rec.Code != 400 {
		t.Errorf("Schreiben ohne Garten: %d", rec.Code)
	}
	if rec := do(h, "GET", "/api/admin/maengelschreiben?gartennr=3&frist=2030-01-31", nil); rec.Code != 200 {
		t.Errorf("Schreiben mit Frist: %d", rec.Code)
	}

	// Datenschutz: Auskunft enthält Mängel, endgültiges Löschen entfernt sie
	var pid string
	app.st.mu.Lock()
	if p, ok := app.st.aktiverPaechterLocked("3"); ok {
		pid = p.ID
	}
	a, _ := app.st.auskunftLocked(pid)
	app.st.mu.Unlock()
	if len(a.Maengel) != 2 {
		t.Errorf("Auskunft: %d Mängel statt 2", len(a.Maengel))
	}
	do(h, "DELETE", "/api/admin/paechter/"+pid, nil)
	if rec := do(h, "DELETE", "/api/admin/paechter/"+pid+"/endgueltig", nil); rec.Code != 200 {
		t.Fatalf("endgültig löschen: %d %s", rec.Code, rec.Body.String())
	}
	for _, x := range decode[[]mangelAnsicht](t, do(h, "GET", "/api/admin/maengel", nil)) {
		if x.PaechterID == pid {
			t.Errorf("Mangel blieb nach dem Löschen der Person: %+v", x)
		}
	}
	// Mangel löschen
	if rec := do(h, "DELETE", "/api/admin/maengel/"+m.ID, nil); rec.Code != 200 {
		t.Errorf("löschen: %d", rec.Code)
	}

	// nur für den Admin
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	for _, pfad := range []string{"/api/admin/maengel", "/api/admin/begehungsliste", "/api/admin/maengelschreiben?gartennr=7"} {
		if rec := do(h, "GET", pfad, nil); rec.Code != 401 {
			t.Errorf("%s ohne Anmeldung: %d", pfad, rec.Code)
		}
	}
	if rec := do(h, "POST", "/api/admin/maengel", map[string]any{"gartennr": "7", "datum": heute, "beschreibung": "x"}); rec.Code != 401 {
		t.Errorf("anlegen ohne Anmeldung: %d", rec.Code)
	}
	if strings.Contains(do(h, "GET", "/api/state", nil).Body.String(), "Hecke zur Wegseite") {
		t.Error("Mängel dürfen nicht im öffentlichen Zustand stehen")
	}
}
