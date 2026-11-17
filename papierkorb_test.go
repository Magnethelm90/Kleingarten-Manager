package main

import (
	"fmt"
	"testing"
	"time"
)

func TestEintragPapierkorb(t *testing.T) {
	a, h := newTestApp(t)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "7", "name": "Gerd Sieben", "gartennr": "7", "gartengroesse": 100})
	m := decode[Mangel](t, do(h, "POST", "/api/admin/maengel", map[string]any{"gartennr": "7", "datum": "2026-05-01", "art": "Hecke und Bäume", "beschreibung": "zu hoch"}))
	b := decode[Beschluss](t, do(h, "POST", "/api/admin/beschluesse", map[string]any{"datum": "2030-04-01", "gremium": "Vorstand", "titel": "Zaun"}))
	tm := decode[Termin](t, do(h, "POST", "/api/admin/termine", map[string]any{"titel": "Arbeitseinsatz", "datum": "2030-05-01", "art": "Arbeitseinsatz"}))
	for _, u := range []string{"/api/admin/maengel/" + m.ID, "/api/admin/beschluesse/" + b.ID, "/api/admin/termine/" + tm.ID} {
		if c := do(h, "DELETE", u, nil).Code; c != 200 {
			t.Fatalf("%s: %d", u, c)
		}
	}
	liste := decode[[]papierkorbZeile](t, do(h, "GET", "/api/admin/papierkorb", nil))
	if len(liste) != 3 || liste[0].NochTage != papierkorbTage {
		t.Fatalf("Papierkorb: %+v", liste)
	}
	// Beschluss wurde ersetzt: neue Nummer darf nicht doppelt sein
	neu := decode[Beschluss](t, do(h, "POST", "/api/admin/beschluesse", map[string]any{"datum": "2030-04-02", "gremium": "Vorstand", "titel": "Neu"}))
	for _, z := range liste {
		if c := do(h, "POST", "/api/admin/papierkorb/"+z.ID+"/wiederherstellen", nil).Code; c != 200 {
			t.Fatalf("Wiederherstellen %s: %d", z.Art, c)
		}
	}
	if c := do(h, "POST", "/api/admin/papierkorb/"+liste[0].ID+"/wiederherstellen", nil).Code; c != 404 {
		t.Errorf("zweites Wiederherstellen: %d", c)
	}
	a.st.mu.Lock()
	nummern := map[string]bool{}
	for _, x := range a.st.d.Beschluesse {
		if nummern[x.Nummer] {
			t.Errorf("Nummer doppelt: %s", x.Nummer)
		}
		nummern[x.Nummer] = true
	}
	if len(a.st.d.Maengel) != 1 || len(a.st.d.Termine) != 1 || len(a.st.d.Beschluesse) != 2 || len(a.st.d.Papierkorb) != 0 {
		t.Errorf("Bestand: %d/%d/%d/%d", len(a.st.d.Maengel), len(a.st.d.Termine), len(a.st.d.Beschluesse), len(a.st.d.Papierkorb))
	}
	_ = neu
	a.st.mu.Unlock()
}

func TestEintragPapierkorbAufraeumen(t *testing.T) {
	a, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Anna Korb", "gartennr": "1", "gartengroesse": 100}))
	m := decode[Mangel](t, do(h, "POST", "/api/admin/maengel", map[string]any{"gartennr": "1", "datum": "2026-05-01", "art": "Müll", "beschreibung": "x"}))
	do(h, "DELETE", "/api/admin/maengel/"+m.ID, nil)
	a.st.mu.Lock()
	if len(a.st.d.Papierkorb) != 1 || a.st.d.Papierkorb[0].PaechterID != p.ID {
		t.Fatalf("Eintrag: %+v", a.st.d.Papierkorb)
	}
	a.st.d.Papierkorb[0].GeloeschtAm = time.Now().AddDate(0, 0, -papierkorbTage-1).Format("2006-01-02")
	a.st.d.Papierkorb = append(a.st.d.Papierkorb, PapierkorbEintrag{ID: "x", Art: "mangel", GeloeschtAm: time.Now().Format("2006-01-02"), PaechterID: p.ID, Daten: []byte("{}")})
	if !a.st.papierkorbAufraeumenLocked(time.Now()) || len(a.st.d.Papierkorb) != 1 {
		t.Errorf("Aufräumen: %d", len(a.st.d.Papierkorb))
	}
	if !a.st.d.entfernePerson(p.ID) || len(a.st.d.Papierkorb) != 0 {
		t.Errorf("Person nicht aus dem Papierkorb entfernt: %s", fmt.Sprint(a.st.d.Papierkorb))
	}
	a.st.mu.Unlock()
}

func TestLoeschenRueckgaengig(t *testing.T) {
	_, h := newTestApp(t)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Eins", "gartennr": "1", "gartengroesse": 100})
	m := decode[Mangel](t, do(h, "POST", "/api/admin/maengel", map[string]any{"gartennr": "1", "datum": "2026-05-01", "art": "Hecke und Bäume", "beschreibung": "zu hoch"}))
	r := decode[map[string]any](t, do(h, "DELETE", "/api/admin/maengel/"+m.ID, nil))
	pk, _ := r["papierkorbId"].(string)
	if pk == "" {
		t.Fatalf("keine Papierkorb-ID: %v", r)
	}
	if rec := do(h, "POST", "/api/admin/papierkorb/"+pk+"/wiederherstellen", nil); rec.Code != 200 {
		t.Fatalf("Rückgängig: %d", rec.Code)
	}
	if l := decode[[]mangelAnsicht](t, do(h, "GET", "/api/admin/maengel", nil)); len(l) != 1 || l[0].ID != m.ID {
		t.Errorf("Mangel nicht zurück: %+v", l)
	}
}
