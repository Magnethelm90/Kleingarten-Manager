package main

import (
	"fmt"
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
	// Zählerstände wandern beim Wechsel mit
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Erika", "gartengroesse": 300}))
	do(h, "PUT", "/api/ablesung/"+p.ID, map[string]any{"wasserVJ": 1, "wasserAkt": 5, "stromVJ": 1, "stromAkt": 9, "stunden": 10})
	if rec := do(h, "POST", "/api/admin/startjahr", map[string]int{"jahr": jahr - 1}); rec.Code != 200 {
		t.Fatalf("mit Ablesung: %d %s", rec.Code, rec.Body.String())
	}
	st = decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if st.Year != jahr-1 || st.Ablesungen[p.ID].WasserAkt == nil || *st.Ablesungen[p.ID].WasserAkt != 5 {
		t.Fatalf("Ablesung nicht mitgewandert: %d %+v", st.Year, st.Ablesungen[p.ID])
	}
	// nach der ersten Rechnung nicht mehr änderbar
	if rec := do(h, "POST", "/api/invoices/issue?year="+fmt.Sprint(jahr-1), map[string]any{"ids": []string{p.ID}}); rec.Code != 200 {
		t.Fatalf("ausstellen: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, "POST", "/api/admin/startjahr", map[string]int{"jahr": jahr}); rec.Code != 400 || !strings.Contains(rec.Body.String(), "Jahreswechsel") {
		t.Fatalf("nach Rechnung: %d %s", rec.Code, rec.Body.String())
	}
	// Nach dem Jahreswechsel liegt das neue Rechnungsdatum in der Zukunft: keine Warnung im laufenden Betrieb
	if rec := do(h, "POST", "/api/admin/jahreswechsel", nil); rec.Code != 200 {
		t.Fatalf("Jahreswechsel: %d %s", rec.Code, rec.Body.String())
	}
	a.st.mu.Lock()
	rd, _ := parseDate(a.st.d.Settings.Rechnungsdatum)
	for _, tage := range []int{-200, -60, 0} {
		if m := a.st.musterwerteLocked(rd.AddDate(0, 0, tage)); len(m) != 0 {
			t.Errorf("nach Jahreswechsel, %d Tage: %v", tage, m)
		}
	}
	a.st.mu.Unlock()
}
