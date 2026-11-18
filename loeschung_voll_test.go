package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestLoeschungInAllenBereichen löscht im vollen Beispielbestand (alle Bereiche befüllt) jede Person endgültig
// und sucht danach im gesamten Datenbestand nach ihren Angaben. Kontaktdaten und Notizen dürfen nirgends mehr
// stehen; der Name nur noch in Unterlagen mit Aufbewahrungspflicht (Rechnungen, Jahreslisten, Lastschriften,
// bestätigte Spenden). SEPA-Mandate müssen mit der Person verschwinden.
func TestLoeschungInAllenBereichen(t *testing.T) {
	a, h := newTestApp(t)
	if rec := do(h, "POST", "/api/admin/beispieldaten", map[string]any{}); rec.Code >= 300 {
		t.Fatalf("Beispieldaten: %d", rec.Code)
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	d := a.st.d
	type person struct{ id, name, email, tel, notiz string }
	var alle []person
	for i := range d.Paechter {
		p := &d.Paechter[i]
		// eindeutige Marken, damit die Suche nichts verwechselt
		p.Email, p.Telefon, p.Notiz = "marke"+p.ID+"@example.org", "0999-"+p.ID, "Notizmarke-"+p.ID
		alle = append(alle, person{p.ID, p.Name, p.Email, p.Telefon, p.Notiz})
	}
	if len(alle) < 5 {
		t.Fatal("zu wenige Pächter im Beispielbestand")
	}
	for _, p := range alle {
		d.entfernePerson(p.id)
	}
	if len(d.Paechter) != 0 {
		t.Fatalf("%d Stammdatensätze übrig", len(d.Paechter))
	}
	if len(d.Mandate) != 0 {
		t.Errorf("%d SEPA-Mandate (IBAN) nach dem Löschen übrig", len(d.Mandate))
	}
	voll, _ := json.Marshal(d)
	for _, p := range alle {
		for _, s := range []string{p.email, p.tel, p.notiz} {
			if strings.Contains(string(voll), s) {
				t.Errorf("%s: %q steht nach dem Löschen noch im Bestand", p.name, s)
			}
		}
	}
	// Ohne die aufbewahrungspflichtigen Unterlagen darf kein Name mehr vorkommen
	rest := *d
	rest.Rechnungen, rest.Jahre, rest.Lastschriften, rest.Spenden = nil, nil, nil, nil
	ohne, _ := json.Marshal(&rest)
	for _, p := range alle {
		// auch die interne ID darf außerhalb der Aufbewahrungsunterlagen nicht mehr vorkommen
		if i := strings.Index(string(ohne), p.id); i >= 0 {
			t.Errorf("ID von %q steht noch außerhalb der Aufbewahrungsunterlagen: …%s…", p.name, string(ohne)[max(0, i-100):min(len(ohne), i+40)])
		}
		if i := strings.Index(string(ohne), p.name); i >= 0 {
			t.Errorf("Name %q steht noch außerhalb der Aufbewahrungsunterlagen: …%s…", p.name, string(ohne)[max(0, i-100):min(len(ohne), i+60)])
		}
	}
}
