package main

import (
	"strings"
	"testing"
)

func TestWartelisteBriefeUndNachruecker(t *testing.T) {
	a, h := newTestApp(t)
	b1 := decode[Bewerber](t, do(h, "POST", "/api/admin/warteliste", map[string]any{"name": "Erste Absage", "seit": "2024-01-01", "besichtigungAm": "2026-05-01", "besichtigungErgebnis": "kein Interesse"}))
	b2 := decode[Bewerber](t, do(h, "POST", "/api/admin/warteliste", map[string]any{"name": "Zweite Reihe", "seit": "2024-02-01", "anrede": "Frau", "strasse": "Weg 1", "plzOrt": "12345 Ort", "besichtigungAm": "2026-07-01", "besichtigungUm": "17:30"}))
	do(h, "POST", "/api/admin/warteliste", map[string]any{"name": "Dritte Reihe", "seit": "2025-03-01"})

	n := decode[[]Bewerber](t, do(h, "GET", "/api/admin/warteliste-nachruecker", nil))
	if len(n) != 2 || n[0].Name != "Zweite Reihe" || n[0].ID == b1.ID {
		t.Fatalf("Nachrücker: %+v", n)
	}
	for _, art := range []string{"angebot", "nachfrage", "bestaetigung"} {
		rec := do(h, "GET", "/api/admin/warteliste/"+b2.ID+"/brief?art="+art+"&garten=12", nil)
		if rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), "%PDF") {
			t.Errorf("Brief %s: %d", art, rec.Code)
		}
	}
	if rec := do(h, "GET", "/api/admin/warteliste/"+b2.ID+"/brief?art=unsinn", nil); rec.Code != 400 {
		t.Errorf("unbekannte Art: %d", rec.Code)
	}
	if rec := do(h, "GET", "/api/admin/warteliste/gibtesnicht/brief?art=angebot", nil); rec.Code != 404 {
		t.Errorf("unbekannter Eintrag: %d", rec.Code)
	}
	// Cockpit: nur bei laufender Kündigung ohne Nachfolger-Anfrage
	var l []string
	add := func(stufe, titel, text, ziel, knopf string) { l = append(l, titel) }
	a.st.mu.Lock()
	a.st.nachrueckerAufgabeLocked(add)
	a.st.d.Kuendigungen = []Kuendigung{{ID: "k1", Gartennr: "1"}}
	a.st.nachrueckerAufgabeLocked(add)
	a.st.d.Kuendigungen[0].Erledigt = map[string]string{"warteliste": "2026-06-01"}
	a.st.nachrueckerAufgabeLocked(add)
	a.st.mu.Unlock()
	if len(l) != 1 || !strings.Contains(l[0], "Zweite Reihe") {
		t.Errorf("Cockpit: %v", l)
	}
}
