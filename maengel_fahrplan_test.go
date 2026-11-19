package main

import (
	"strings"
	"testing"
)

func TestMaengelFahrplan(t *testing.T) {
	a, h := newTestApp(t)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Anna Frist", "gartennr": "7", "gartengroesse": 100})
	m := decode[Mangel](t, do(h, "POST", "/api/admin/maengel", map[string]any{"gartennr": "7", "datum": "2026-05-01", "art": "Hecke und Bäume", "beschreibung": "Hecke zu hoch", "frist": "2026-05-20"}))

	cockpit := func() string {
		var l []string
		a.st.mu.Lock()
		a.st.maengelFahrplanLocked("2026-06-15", func(stufe, titel, text, ziel, knopf string) { l = append(l, stufe+": "+titel) })
		a.st.mu.Unlock()
		return strings.Join(l, "|")
	}
	if c := cockpit(); !strings.Contains(c, "Mängelfrist abgelaufen") {
		t.Fatalf("Stufe 0: %q", c)
	}
	// Schreiben mit Stufe liefert PDF; ungültige Stufe fällt auf die Aufforderung zurück
	for _, st := range []string{"erst", "erinnerung", "zweit", "unsinn"} {
		if rec := do(h, "GET", "/api/admin/maengelschreiben?gartennr=7&stufe="+st, nil); rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), "%PDF") {
			t.Errorf("PDF %s: %d", st, rec.Code)
		}
	}
	for _, bad := range []map[string]string{{"gartennr": "7", "stufe": "x", "frist": "2026-07-01"}, {"gartennr": "7", "stufe": "erst", "frist": ""}, {"gartennr": "99", "stufe": "erst", "frist": "2026-07-01"}} {
		if rec := do(h, "POST", "/api/admin/maengel-schreiben", bad); rec.Code < 400 {
			t.Errorf("%v: %d", bad, rec.Code)
		}
	}
	// Erinnerung verschickt: Frist wird neu gesetzt, danach Ruhe bis zum Ablauf der neuen Frist
	if rec := do(h, "POST", "/api/admin/maengel-schreiben", map[string]string{"gartennr": "7", "stufe": "erinnerung", "frist": "2026-06-10"}); rec.Code != 200 {
		t.Fatalf("Stufe vermerken: %d %s", rec.Code, rec.Body)
	}
	if c := cockpit(); !strings.Contains(c, "Garten 7: Frist nach der Erinnerung verstrichen") || strings.Contains(c, "Mängelfrist abgelaufen") {
		t.Errorf("Stufe 1: %q", c)
	}
	// Bearbeiten darf den Fahrplan nicht löschen
	do(h, "PUT", "/api/admin/maengel/"+m.ID, map[string]any{"gartennr": "7", "datum": "2026-05-01", "art": "Hecke und Bäume", "beschreibung": "Hecke zu hoch (geändert)", "frist": "2026-06-10"})
	a.st.mu.Lock()
	hoch := a.st.d.Maengel[0].hoechsteStufe()
	a.st.mu.Unlock()
	if hoch != 1 {
		t.Errorf("Fahrplan nach Bearbeiten: %d", hoch)
	}
	do(h, "POST", "/api/admin/maengel-schreiben", map[string]string{"gartennr": "7", "stufe": "zweit", "frist": "2026-06-12"})
	if c := cockpit(); !strings.HasPrefix(c, "err: Garten 7: letzte Frist verstrichen") {
		t.Errorf("Stufe 2: %q", c)
	}
	// erledigt: kein Hinweis mehr
	do(h, "PUT", "/api/admin/maengel/"+m.ID, map[string]any{"gartennr": "7", "datum": "2026-05-01", "art": "Hecke und Bäume", "beschreibung": "Hecke zu hoch", "frist": "2026-06-12", "erledigtAm": "2026-06-13"})
	if c := cockpit(); c != "" {
		t.Errorf("erledigt: %q", c)
	}
}
