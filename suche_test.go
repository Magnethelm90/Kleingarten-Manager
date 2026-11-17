package main

import "testing"

func TestGesamtsuche(t *testing.T) {
	_, h := newTestApp(t)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "7", "name": "Gerd Sieben", "gartennr": "7", "gartengroesse": 100})
	do(h, "POST", "/api/admin/maengel", map[string]any{"gartennr": "7", "datum": "2026-05-01", "art": "Hecke und Bäume", "beschreibung": "Hecke zur Wegseite zu hoch"})
	do(h, "POST", "/api/admin/termine", map[string]any{"titel": "Frühjahrsputz", "datum": "2026-04-04", "art": "Arbeitseinsatz"})
	do(h, "POST", "/api/admin/warteliste", map[string]any{"name": "Anna Müller", "seit": "2026-01-01", "anrede": "Frau"})
	hol := func(q string) []sucheTreffer {
		return decode[[]sucheTreffer](t, do(h, "GET", "/api/admin/suche?q="+q, nil))
	}
	if r := hol("hecke%20hoch"); len(r) != 1 || r[0].Art != "Mangel" {
		t.Errorf("Mangel: %+v", r)
	}
	if r := hol("fruehjahrsputz"); len(r) != 1 || r[0].Art != "Termin" {
		t.Errorf("Umlaut: %+v", r)
	}
	if r := hol("mueller"); len(r) != 1 || r[0].Art != "Warteliste" {
		t.Errorf("Warteliste: %+v", r)
	}
	if r := hol("a"); len(r) != 0 {
		t.Errorf("zu kurz: %+v", r)
	}
	if r := hol("gibtesnicht"); r == nil || len(r) != 0 {
		t.Errorf("leer muss [] sein: %+v", r)
	}
}
