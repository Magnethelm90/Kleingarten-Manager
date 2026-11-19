package main

import (
	"net/http"
	"testing"
	"time"
)

func TestNutzungDrittelregelung(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	heute := time.Now().Format("2006-01-02")
	for name, body := range map[string]map[string]any{
		"Summe passt nicht": {"gartennr": "3", "datum": heute, "anbau": 10, "erholung": 10, "bebauung": 10},
		"negativ":           {"gartennr": "3", "datum": heute, "anbau": -5, "erholung": 60, "bebauung": 45},
		"freier Garten":     {"gartennr": "99", "datum": heute, "anbau": 40, "erholung": 40, "bebauung": 20},
		"ohne Datum":        {"gartennr": "3", "anbau": 40, "erholung": 40, "bebauung": 20},
	} {
		if rec := do(h, "POST", "/api/admin/nutzung", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	gestern := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	do(h, "POST", "/api/admin/nutzung", map[string]any{"gartennr": "3", "datum": gestern, "anbau": 40, "erholung": 40, "bebauung": 20})
	n := decode[Nutzung](t, do(h, "POST", "/api/admin/nutzung", map[string]any{"gartennr": "3", "datum": heute, "anbau": 20, "erholung": 60, "bebauung": 20}))
	do(h, "POST", "/api/admin/nutzung", map[string]any{"gartennr": "4", "datum": heute, "anbau": 33.4, "erholung": 46.6, "bebauung": 20})
	l := decode[[]nutzungZeile](t, do(h, "GET", "/api/admin/nutzung", nil))
	unter := map[string]bool{}
	for _, z := range l {
		unter[z.Gartennr] = z.Unter
		if z.Gartennr == "3" && (z.Letzte == nil || z.Letzte.ID != n.ID) {
			t.Errorf("Garten 3 zeigt nicht die jüngste Schätzung: %+v", z.Letzte)
		}
	}
	if !unter["3"] || unter["4"] || len(l) != len(demoListe) {
		t.Errorf("Drittelregelung: %v (%d Gärten)", unter, len(l))
	}
	// Begehungsliste zeigt die letzte Schätzung
	app.st.mu.Lock()
	b := app.st.begehungLocked()
	app.st.mu.Unlock()
	for _, z := range b {
		if z.Garten == "3" && z.Anbau != "zuletzt 20" {
			t.Errorf("Begehungsliste: %q", z.Anbau)
		}
	}
	if rec := do(h, "DELETE", "/api/admin/nutzung/"+n.ID, nil); rec.Code != 200 {
		t.Errorf("löschen: %d", rec.Code)
	}
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	if rec := do(h, "GET", "/api/admin/nutzung", nil); rec.Code != 401 {
		t.Errorf("ohne Anmeldung: %d", rec.Code)
	}
}
