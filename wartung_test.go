package main

import "testing"

func TestWartung(t *testing.T) {
	_, h := newTestApp(t)
	r := decode[wartungErgebnis](t, do(h, "GET", "/api/admin/wartung", nil))
	if len(r.Zeilen) < 6 || r.Stufe == "" {
		t.Fatalf("Ergebnis: %+v", r)
	}
	if r.Zeilen[0].Titel != "Datenordner" || r.Zeilen[0].Stufe != "ok" {
		t.Errorf("Datenordner: %+v", r.Zeilen[0])
	}
	if groesseKurz(5<<20) != "5 MB" || groesseKurz(100) != "1 KB" {
		t.Errorf("groesseKurz: %s %s", groesseKurz(5<<20), groesseKurz(100))
	}
}
