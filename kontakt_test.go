package main

import (
	"strings"
	"testing"
)

func TestEmailGueltig(t *testing.T) {
	for _, s := range []string{"anna@example.org", "a.b-c+d@verein.kleingarten.de", "x@y.de"} {
		if !emailGueltig(s) {
			t.Errorf("%q sollte gültig sein", s)
		}
	}
	for _, s := range []string{"", "anna", "anna@", "@example.org", "anna@example", "anna @example.org", "a@b@c.de",
		"anna@example.org?cc=x@y.de", "anna@.de", "anna@example.", "anna@ex..de", "an<na>@x.de", "a&b@x.de"} {
		if emailGueltig(s) {
			t.Errorf("%q sollte ungültig sein", s)
		}
	}
}

func TestPaechterKontakt(t *testing.T) {
	_, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{
		"mitgliedsnr": "1", "name": "Anna", "gartengroesse": 300, "email": " anna@example.org ", "telefon": "0123 456"}))
	if p.Email != "anna@example.org" || p.Telefon != "0123 456" {
		t.Fatalf("Kontakt beim Anlegen: %+v", p)
	}
	rec := do(h, "PUT", "/api/admin/paechter/"+p.ID, map[string]any{"mitgliedsnr": "1", "name": "Anna", "gartengroesse": 300, "email": "anna(at)example.org"})
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "E-Mail") {
		t.Fatalf("ungültige E-Mail muss abgelehnt werden: %d %s", rec.Code, rec.Body.String())
	}
	// nach Ablauf der Aufbewahrungsfrist verschwinden auch E-Mail und Telefon
	q := p
	anonymisiere(&q, fristAbgelaufenText)
	if q.Email != "" || q.Telefon != "" {
		t.Errorf("Kontakt nach Anonymisierung: %+v", q)
	}
}

func TestImportKontaktspalten(t *testing.T) {
	table := [][]string{
		{"Mitgliedsnr.", "Name", "E-Mail-Adresse", "Telefonnummer"},
		{"1", "Anna", "anna@example.org", "0123"},
		{"2", "Bernd", "bernd at example", ""},
	}
	rows, _, err := parseImport(table, false, defaultSettings(), nil)
	if err != nil || len(rows) != 2 {
		t.Fatalf("Import: %v %v", err, rows)
	}
	if rows[0].Paechter.Email != "anna@example.org" || rows[0].Paechter.Telefon != "0123" {
		t.Errorf("Kontaktspalten nicht erkannt: %+v", rows[0].Paechter)
	}
	if rows[1].Paechter.Email != "" || len(rows[1].Warnungen) != 1 {
		t.Errorf("unlesbare E-Mail muss mit Warnung entfallen statt den Import zu blockieren: %+v %v", rows[1].Paechter, rows[1].Warnungen)
	}
}

// Ein Import, der bestehende Pächter aktualisiert, darf Notiz und Kontaktdaten nicht löschen und muss
// die Gartenhistorie fortschreiben, genau wie das Bearbeiten im Formular.
func TestImportBehaeltNotizUndFuehrtGartenhistorie(t *testing.T) {
	_, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{
		"mitgliedsnr": "1", "name": "Anna", "gartennr": "5", "gartengroesse": 300, "notiz": "Schlüssel bei Nachbarin", "email": "anna@example.org"}))

	rows := []ImportRow{
		{Zeile: 2, Paechter: Paechter{Mitgliedsnr: "1", Name: "Anna", Gartennr: "7"}, Felder: []string{"Gartennr."}},
		{Zeile: 3, Paechter: Paechter{Mitgliedsnr: "2", Name: "Bernd", Gartennr: "8"}, Felder: []string{"Gartennr."}},
	}
	if rec := do(h, "POST", "/api/admin/import/apply", map[string]any{"rows": rows}); rec.Code != 200 {
		t.Fatalf("Import: %d %s", rec.Code, rec.Body.String())
	}
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	for _, q := range st.Paechter {
		if q.ID == p.ID && (q.Notiz != "Schlüssel bei Nachbarin" || q.Email != "anna@example.org" || q.Gartennr != "7") {
			t.Errorf("nach dem Import: %+v", q)
		}
	}
	alt := decode[[]GartenEintrag](t, do(h, "GET", "/api/garten-historie?gartennr=5", nil))
	neu := decode[[]GartenEintrag](t, do(h, "GET", "/api/garten-historie?gartennr=7", nil))
	bernd := decode[[]GartenEintrag](t, do(h, "GET", "/api/garten-historie?gartennr=8", nil))
	if len(alt) != 1 || alt[0].Bis == "" {
		t.Errorf("alter Garten nach Import nicht geschlossen: %+v", alt)
	}
	if len(neu) != 1 || neu[0].Bis != "" || neu[0].PaechterID != p.ID {
		t.Errorf("neuer Garten nach Import nicht eingetragen: %+v", neu)
	}
	if len(bernd) != 1 || bernd[0].Name != "Bernd" {
		t.Errorf("neu importierter Pächter fehlt in der Gartenhistorie: %+v", bernd)
	}
}
