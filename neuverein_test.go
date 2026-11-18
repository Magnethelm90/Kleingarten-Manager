package main

import (
	"strings"
	"testing"
	"time"
)

func TestNeuerVereinMusterwerte(t *testing.T) {
	a, h := newTestApp(t)
	heute := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	a.st.mu.Lock()
	m := a.st.musterwerteLocked(heute)
	a.st.mu.Unlock()
	if len(m) != 4 {
		t.Fatalf("frische Installation: %v", m)
	}
	// Beispieldaten: keine Warnung; nach dem Entfernen wieder Neuzustand
	if rec := do(h, "POST", "/api/admin/beispieldaten", nil); rec.Code != 200 {
		t.Fatalf("Beispieldaten: %d %s", rec.Code, rec.Body.String())
	}
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if len(st.Musterwerte) != 0 {
		t.Fatalf("Beispieldaten melden Musterwerte: %v", st.Musterwerte)
	}
	if rec := do(h, "DELETE", "/api/admin/beispieldaten", nil); rec.Code != 200 {
		t.Fatalf("entfernen: %d", rec.Code)
	}
	st = decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if len(st.Musterwerte) == 0 || !st.StartjahrAenderbar {
		t.Fatalf("nach dem Entfernen: %v %v", st.Musterwerte, st.StartjahrAenderbar)
	}
	// eigene Angaben: keine Warnung mehr (Rechnungsdatum über das Startjahr passend gesetzt)
	s := st.Settings
	s.VereinName, s.IBAN, s.Absenderzeile = "KGV Am Mühlbach e. V.", "DE89 3704 0044 0532 0130 00", "KGV Am Mühlbach e. V. · Mühlweg 1 · 04109 Leipzig"
	if rec := do(h, "PUT", "/api/admin/settings", s); rec.Code != 200 {
		t.Fatalf("settings: %d %s", rec.Code, rec.Body.String())
	}
	jahr := time.Now().Year()
	if rec := do(h, "POST", "/api/admin/startjahr", map[string]int{"jahr": jahr}); rec.Code != 200 {
		t.Fatalf("startjahr: %d %s", rec.Code, rec.Body.String())
	}
	st = decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if st.CurrentYear != jahr || st.Year != jahr || len(st.Years) != 1 || st.Settings.Rechnungsdatum < time.Now().Format("2006-01-02") {
		t.Fatalf("Startjahr: %d/%d %v Rechnungsdatum %s", st.CurrentYear, st.Year, st.Years, st.Settings.Rechnungsdatum)
	}
	if len(st.Musterwerte) != 0 {
		t.Fatalf("noch Musterwerte: %v", st.Musterwerte)
	}
	// außerhalb des Bereichs
	if rec := do(h, "POST", "/api/admin/startjahr", map[string]int{"jahr": jahr + 5}); rec.Code != 400 {
		t.Fatalf("Jahr +5: %d", rec.Code)
	}
	// nach der ersten Ablesung nicht mehr änderbar
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Erika", "gartengroesse": 300}))
	do(h, "PUT", "/api/ablesung/"+p.ID, map[string]any{"wasserVJ": 1})
	if rec := do(h, "POST", "/api/admin/startjahr", map[string]int{"jahr": jahr - 1}); rec.Code != 400 || !strings.Contains(rec.Body.String(), "Jahreswechsel") {
		t.Fatalf("nach Ablesung: %d %s", rec.Code, rec.Body.String())
	}
}
