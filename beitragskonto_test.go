package main

import "testing"

func TestBeitragskonto(t *testing.T) {
	app, h := newTestApp(t)
	app.st.mu.Lock()
	app.st.d.Paechter = append(app.st.d.Paechter, Paechter{ID: "pk", Name: "Kontotest", Gartennr: "9", Mitgliedsnr: "9-01"})
	teil := 40.0
	app.st.d.Rechnungen = append(app.st.d.Rechnungen,
		&Rechnung{ID: "r1", Jahr: 2025, PaechterID: "pk", Nummer: "2025-1", Status: "gueltig", Result: Result{Gesamt: 100}, BezahltAm: "2026-01-05"},
		&Rechnung{ID: "r2", Jahr: 2026, PaechterID: "pk", Nummer: "2026-1", Status: "gueltig", Result: Result{Gesamt: 100}, BezahltAm: "2026-02-01", BezahltBetrag: &teil, Mahnungen: []Mahnung{{Stufe: 1}}},
		&Rechnung{ID: "r3", Jahr: 2026, PaechterID: "pk", Nummer: "2026-2", Status: "gueltig", Result: Result{Gesamt: -30}},
		&Rechnung{ID: "r4", Jahr: 2024, PaechterID: "pk", Nummer: "2024-1", Status: "ersetzt", Result: Result{Gesamt: 999}},
		&Rechnung{ID: "r5", Jahr: 2026, PaechterID: "anderer", Nummer: "x", Status: "gueltig", Result: Result{Gesamt: 5}})
	app.st.d.Vorauszahlungen = append(app.st.d.Vorauszahlungen, Vorauszahlung{ID: "v1", PaechterID: "pk", Jahr: 2026, Betrag: 25, Gebucht: true})
	app.st.mu.Unlock()
	k := decode[beitragskonto](t, do(h, "GET", "/api/admin/paechter/pk/konto", nil))
	if len(k.Zeilen) != 3 {
		t.Fatalf("erwartet 3 gültige Rechnungen, war %d", len(k.Zeilen))
	}
	if k.Forderung != 60 || k.Guthaben != 30 || k.Saldo != 30 {
		t.Fatalf("Forderung %v Guthaben %v Saldo %v", k.Forderung, k.Guthaben, k.Saldo)
	}
	if k.Zeilen[0].Jahr != 2026 || len(k.Vorauszahlung) != 1 || k.Vorauszahlung[0].Summe != 25 {
		t.Fatalf("Sortierung/Vorauszahlung: %+v %+v", k.Zeilen, k.Vorauszahlung)
	}
	if rec := do(h, "GET", "/api/admin/paechter/gibtsnicht/konto", nil); rec.Code != 404 {
		t.Fatalf("unbekannt: %d", rec.Code)
	}
	if rec := do(h, "GET", "/api/admin/paechter/pk/konto/pdf", nil); rec.Code != 200 || rec.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("PDF: %d", rec.Code)
	}
}
