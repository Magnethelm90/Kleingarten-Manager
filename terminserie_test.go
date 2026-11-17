package main

import (
	"testing"
	"time"
)

func tag(s string) time.Time { d, _ := parseDate(s); return d }

func TestSerienDaten(t *testing.T) {
	// erster Dienstag im Monat: 2026-01-06, 02-03, 03-03
	g := serienDaten(tag("2026-01-06"), tag("2026-03-31"), rhythmusMonatWT)
	if len(g) != 3 || g[1].Format("2006-01-02") != "2026-02-03" || g[2].Format("2006-01-02") != "2026-03-03" {
		t.Errorf("erster Dienstag: %v", g)
	}
	// letzter Freitag: 2026-01-30, 02-27
	g = serienDaten(tag("2026-01-30"), tag("2026-02-28"), rhythmusMonatWT)
	if len(g) != 2 || g[1].Format("2006-01-02") != "2026-02-27" {
		t.Errorf("letzter Freitag: %v", g)
	}
	// 31. des Monats: Monate ohne 31. entfallen
	g = serienDaten(tag("2026-01-31"), tag("2026-04-30"), rhythmusMonat)
	if len(g) != 2 {
		t.Errorf("31.: %v", g)
	}
	if g := serienDaten(tag("2026-01-01"), tag("2026-01-29"), rhythmus14Tage); len(g) != 3 {
		t.Errorf("14 Tage: %v", g)
	}
	if g := serienDaten(tag("2026-01-01"), tag("2040-01-01"), rhythmusWoche); len(g) > maxSerie {
		t.Errorf("Obergrenze: %d", len(g))
	}
}

func TestTerminSerieRoute(t *testing.T) {
	a, h := newTestApp(t)
	rec := do(h, "POST", "/api/admin/termine-serie", map[string]any{"titel": "Vorstandssitzung", "datum": "2026-01-06", "art": "Vorstand", "rhythmus": "monatlich_wochentag", "bis": "2026-06-30"})
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	a.st.mu.Lock()
	n := len(a.st.d.Termine)
	a.st.mu.Unlock()
	if n != 6 {
		t.Errorf("Termine: %d", n)
	}
	for _, body := range []map[string]any{
		{"titel": "x", "datum": "2026-01-06", "rhythmus": "täglich", "bis": "2026-06-30"},
		{"titel": "x", "datum": "2026-01-06", "rhythmus": "woechentlich", "bis": "2025-01-01"},
		{"titel": "", "datum": "2026-01-06", "rhythmus": "woechentlich", "bis": "2026-06-30"},
	} {
		if c := do(h, "POST", "/api/admin/termine-serie", body).Code; c != 400 {
			t.Errorf("ungültig angenommen: %v → %d", body, c)
		}
	}
}
