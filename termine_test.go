package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestTermine(t *testing.T) {
	_, h := newTestApp(t)
	for _, bad := range []map[string]any{
		{"datum": "2026-04-04", "titel": ""},
		{"datum": "2026-02-30", "titel": "X"},
		{"datum": "2026-04-04", "titel": "X", "von": "9 Uhr"},
		{"datum": "2026-04-04", "titel": "X", "von": "10:00", "bis": "09:00"},
	} {
		if rec := do(h, "POST", "/api/admin/termine", bad); rec.Code != 400 {
			t.Errorf("ungültiger Termin %v: %d", bad, rec.Code)
		}
	}
	a := decode[Termin](t, do(h, "POST", "/api/admin/termine", map[string]any{
		"datum": "2026-04-04", "von": "09:00", "bis": "13:00", "titel": "Arbeitseinsatz; Teil 1", "art": "Arbeitseinsatz", "ort": "Tor"}))
	b := decode[Termin](t, do(h, "POST", "/api/admin/termine", map[string]any{
		"datum": "2026-03-14", "titel": "Versammlung\r\nBEGIN:VEVENT", "art": "Unsinn"}))
	if b.Art != "Sonstiges" {
		t.Errorf("unbekannte Art: %q", b.Art)
	}
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if len(st.Termine) != 2 || st.Termine[0].ID != b.ID {
		t.Fatalf("Termine nicht nach Datum sortiert: %+v", st.Termine)
	}

	// Kalenderdatei: ganztägig und mit Uhrzeit, keine eingeschleusten Einträge
	ics := do(h, "GET", "/api/termine.ics", nil).Body.String()
	for _, want := range []string{"DTSTART:20260404T090000", "DTEND:20260404T130000", "DTSTART;VALUE=DATE:20260314", "DTEND;VALUE=DATE:20260315",
		`SUMMARY:Arbeitseinsatz\; Teil 1`, "LOCATION:Tor"} {
		if !strings.Contains(ics, want) {
			t.Errorf("ICS ohne %q:\n%s", want, ics)
		}
	}
	if strings.Count(ics, "\r\nBEGIN:VEVENT\r\n") != 2 || strings.Contains(ics, "Versammlung\\n") {
		t.Errorf("eingeschleuster Kalendereintrag:\n%s", ics)
	}

	// Aushang als PDF
	if rec := do(h, "GET", "/api/termine-druck?jahr=2026", nil); rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Errorf("Jahresplan: %d", rec.Code)
	}
	if rec := do(h, "GET", "/api/termine-druck?jahr=2031", nil); rec.Code != 400 {
		t.Errorf("leeres Jahr: %d", rec.Code)
	}

	// ins Folgejahr übernehmen: gleicher Wochentag, keine Doppelten
	r := decode[map[string]int](t, do(h, "POST", "/api/admin/termine-uebernehmen", map[string]int{"von": 2026, "nach": 2027}))
	if r["uebernommen"] != 2 {
		t.Fatalf("übernommen: %v", r)
	}
	if r := decode[map[string]int](t, do(h, "POST", "/api/admin/termine-uebernehmen", map[string]int{"von": 2026, "nach": 2027})); r["uebernommen"] != 0 {
		t.Errorf("zweites Übernehmen legt Doppelte an: %v", r)
	}
	st = decode[stateResp](t, do(h, "GET", "/api/state", nil))
	for _, x := range st.Termine {
		if x.Titel == a.Titel && strings.HasPrefix(x.Datum, "2027") {
			d, _ := parseDate(x.Datum)
			if d.Weekday() != time.Saturday || x.Von != "09:00" || x.ID == a.ID {
				t.Errorf("übernommener Termin: %+v (%s)", x, d.Weekday())
			}
		}
	}

	// ändern und löschen
	a.Titel = "Frühjahrsputz"
	if rec := do(h, "PUT", "/api/admin/termine/"+a.ID, a); rec.Code != 200 {
		t.Errorf("Ändern: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, "DELETE", "/api/admin/termine/"+a.ID, nil); rec.Code != 200 {
		t.Errorf("Löschen: %d", rec.Code)
	}
	if rec := do(h, "DELETE", "/api/admin/termine/"+a.ID, nil); rec.Code != 404 {
		t.Errorf("zweites Löschen: %d", rec.Code)
	}

	// mit Passwort: ansehen darf jeder, ändern nur der Admin
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	if rec := do(h, "GET", "/api/termine.ics", nil); rec.Code != 200 {
		t.Errorf("Kalender ohne Anmeldung: %d", rec.Code)
	}
	if rec := do(h, "DELETE", "/api/admin/termine/"+b.ID, nil); rec.Code != 401 {
		t.Errorf("Löschen ohne Anmeldung: %d", rec.Code)
	}
}

func TestGleicherWochentag(t *testing.T) {
	for _, c := range []struct{ von, nach string }{
		{"2026-04-04", "2027-04-03"}, // Samstag → Samstag
		{"2026-01-01", "2027-01-07"}, // Donnerstag; der 31.12.2026 läge im falschen Jahr
		{"2026-12-31", "2027-12-30"},
		{"2024-02-29", "2025-02-27"},
	} {
		d, _ := parseDate(c.von)
		n := gleicherWochentag(d, d.Year()+1)
		if n.Weekday() != d.Weekday() || n.Year() != d.Year()+1 {
			t.Errorf("%s: %s (%s)", c.von, n.Format("2006-01-02"), n.Weekday())
		}
		if n.Format("2006-01-02") != c.nach {
			t.Errorf("%s → %s, erwartet %s", c.von, n.Format("2006-01-02"), c.nach)
		}
	}
}

func TestWarteliste(t *testing.T) {
	app, h := newTestApp(t)
	if rec := do(h, "POST", "/api/admin/warteliste", map[string]any{"name": "X", "email": "kein-at"}); rec.Code != 400 {
		t.Errorf("ungültige E-Mail: %d", rec.Code)
	}
	alt := time.Now().AddDate(-3, 0, 0).Format("2006-01-02")
	b1 := decode[Bewerber](t, do(h, "POST", "/api/admin/warteliste", map[string]any{"name": "Später", "seit": "2025-05-01"}))
	b2 := decode[Bewerber](t, do(h, "POST", "/api/admin/warteliste", map[string]any{"name": "Früher", "seit": alt, "wunsch": "mit Laube"}))
	l := decode[[]bewerberResp](t, do(h, "GET", "/api/admin/warteliste", nil))
	if len(l) != 2 || l[0].ID != b2.ID || l[0].Platz != 1 || !l[0].Nachfragen {
		t.Fatalf("Reihenfolge oder Nachfrage-Hinweis: %+v", l)
	}
	// Interesse bestätigt: kein Hinweis mehr
	b2.BestaetigtAm = time.Now().Format("2006-01-02")
	do(h, "PUT", "/api/admin/warteliste/"+b2.ID, b2)
	if l := decode[[]bewerberResp](t, do(h, "GET", "/api/admin/warteliste", nil)); l[0].Nachfragen {
		t.Error("nach Bestätigung sollte nicht mehr nachgefragt werden")
	}
	ds := decode[datenschutzInfo](t, do(h, "GET", "/api/admin/datenschutz", nil))
	if ds.WartelisteNachfragen != 0 {
		t.Errorf("Datenschutz-Hinweis: %d", ds.WartelisteNachfragen)
	}

	// Löschen entfernt den Eintrag auch aus den Sicherungen
	app.st.mu.Lock()
	app.st.snapshotBackup("test")
	app.st.mu.Unlock()
	rec := do(h, "DELETE", "/api/admin/warteliste/"+b1.ID, nil)
	if rec.Code != 200 {
		t.Fatalf("Löschen: %d", rec.Code)
	}
	if r := decode[map[string]int](t, rec); r["sicherungenBereinigt"] < 1 {
		t.Errorf("Sicherungen nicht bereinigt: %v", r)
	}
	app.st.mu.Lock()
	for _, f := range app.st.sicherungsDateien() {
		raw, _ := os.ReadFile(f)
		if bytes.Contains(raw, []byte("Später")) {
			t.Errorf("Name noch in Sicherung %s", f)
		}
	}
	app.st.mu.Unlock()
	if rec := do(h, "DELETE", "/api/admin/warteliste/"+b1.ID, nil); rec.Code != 404 {
		t.Errorf("zweites Löschen: %d", rec.Code)
	}
}
