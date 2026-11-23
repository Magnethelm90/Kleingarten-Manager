package main

import (
	"strings"
	"testing"
)

func TestGutschrift(t *testing.T) {
	a, h := newTestApp(t)
	a.st.mu.Lock()
	r := &Rechnung{ID: "r1", Jahr: 2026, Nummer: "100-7", Status: statusGueltig, Settings: a.st.d.Settings,
		Paechter: Paechter{Name: "Anna Test", Mitgliedsnr: "7", Strasse: "Weg 1", PLZOrt: "12345 Ort"}}
	r.Result.Gesamt = 100
	g := &Rechnung{ID: "r2", Jahr: 2026, Nummer: "100-8", Status: statusGueltig, Settings: a.st.d.Settings, Paechter: Paechter{Name: "Guthaben", Mitgliedsnr: "8"}}
	g.Result.Gesamt = -20
	a.st.d.Rechnungen = append(a.st.d.Rechnungen, r, g)
	a.st.mu.Unlock()

	if c := do(h, "POST", "/api/admin/rechnungen/r1/gutschrift", map[string]any{"betrag": 10}).Code; c != 400 {
		t.Errorf("ohne Grund: %d", c)
	}
	for _, b := range []float64{0, -5, 100.01} {
		if c := do(h, "POST", "/api/admin/rechnungen/r1/gutschrift", map[string]any{"betrag": b, "grund": "x"}).Code; c != 400 {
			t.Errorf("Betrag %v: %d", b, c)
		}
	}
	if c := do(h, "POST", "/api/admin/rechnungen/r2/gutschrift", map[string]any{"betrag": 5, "grund": "x"}).Code; c != 400 {
		t.Errorf("Guthaben: %d", c)
	}
	if c := do(h, "POST", "/api/admin/rechnungen/gibtesnicht/gutschrift", map[string]any{"betrag": 5, "grund": "x"}).Code; c != 404 {
		t.Errorf("unbekannt: %d", c)
	}
	if c := do(h, "GET", "/api/admin/rechnungen/r1/gutschrift", nil).Code; c != 404 {
		t.Errorf("PDF ohne Gutschrift: %d", c)
	}

	// vor der Zahlung: Betrag sinkt
	if c := do(h, "POST", "/api/admin/rechnungen/r1/gutschrift", map[string]any{"betrag": 30, "grund": "Zähler falsch abgelesen"}).Code; c != 200 {
		t.Fatalf("Gutschrift: %d", c)
	}
	if o := openAmount(r); o != 70 {
		t.Errorf("offen nach Gutschrift: %v", o)
	}
	if r.Gutschriften[0].Nummer != "GS-100-7-1" {
		t.Errorf("Nummer: %q", r.Gutschriften[0].Nummer)
	}
	// Teilzahlung und volle Zahlung beziehen sich auf den neuen Betrag
	do(h, "PUT", "/api/payment/r1", map[string]any{"bezahltAm": "2026-03-01", "bezahltBetrag": 70})
	if o := openAmount(r); o != 0 || r.BezahltBetrag != nil {
		t.Errorf("voll bezahlt (netto): offen %v, betrag %v", o, r.BezahltBetrag)
	}
	// weitere Gutschrift nach der Zahlung: Erstattung wird sichtbar, die Zahlung bleibt stehen
	if c := do(h, "POST", "/api/admin/rechnungen/r1/gutschrift", map[string]any{"betrag": 10, "grund": "Nachlass", "datum": "2026-04-01"}).Code; c != 200 {
		t.Fatalf("zweite Gutschrift: %d", c)
	}
	if o := openAmount(r); o != -10 || paidAmount(r) != 70 {
		t.Errorf("Erstattung: offen %v, bezahlt %v", o, paidAmount(r))
	}
	if r.Gutschriften[1].Nummer != "GS-100-7-2" {
		t.Errorf("zweite Nummer: %q", r.Gutschriften[1].Nummer)
	}
	if c := do(h, "POST", "/api/admin/rechnungen/r1/gutschrift", map[string]any{"betrag": 61, "grund": "zu viel"}).Code; c != 400 {
		t.Errorf("mehr als Rest: %d", c)
	}

	for _, nr := range []string{"", "?nr=1", "?nr=2", "?nr=99"} {
		rec := do(h, "GET", "/api/admin/rechnungen/r1/gutschrift"+nr, nil)
		if rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), "%PDF") {
			t.Errorf("PDF %q: %d", nr, rec.Code)
		}
	}
	// Archiv-Eintrag trägt die Gutschriften
	if e := archiveEntryFrom(r); len(e.Gutschriften) != 2 {
		t.Errorf("Archiv: %+v", e.Gutschriften)
	}
	// Rücknahme nur der jüngsten
	if c := do(h, "DELETE", "/api/admin/rechnungen/r1/gutschrift", nil).Code; c != 200 || len(r.Gutschriften) != 1 {
		t.Errorf("Rücknahme: %d, %d", c, len(r.Gutschriften))
	}
	// Datenschutz: Freitext verschwindet bei Bereinigung
	r.gutschriftGruendeLeeren()
	if r.Gutschriften[0].Grund != fristAbgelaufenText || r.Gutschriften[0].Betrag != 30 {
		t.Errorf("bereinigt: %+v", r.Gutschriften[0])
	}
}
