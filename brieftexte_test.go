package main

import (
	"strings"
	"testing"
)

func TestBrieftexte(t *testing.T) {
	a, h := newTestApp(t)
	// Standard mit Platzhalter
	var b Brieftexte
	if got := b.text("maengelErst", map[string]string{"Frist": "01.05.2027"}); !strings.Contains(got, "bis zum 01.05.2027") {
		t.Fatalf("Platzhalter: %q", got)
	}
	// eigener Text über die eigene Route
	if rec := do(h, "PUT", "/api/admin/brieftexte/gruss", map[string]string{"text": "Herzliche Grüße aus der Anlage"}); rec.Code != 200 {
		t.Fatalf("speichern: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, "PUT", "/api/admin/brieftexte/gibtsnicht", map[string]string{"text": "x"}); rec.Code != 404 {
		t.Fatalf("unbekannter Schlüssel: %d", rec.Code)
	}
	l := decode[[]brieftextZeile](t, do(h, "GET", "/api/admin/brieftexte", nil))
	if len(l) != len(brieftextVorlagen) {
		t.Fatalf("%d Zeilen", len(l))
	}
	for _, z := range l {
		if z.Key == "gruss" && (!z.Angepasst || z.Text != "Herzliche Grüße aus der Anlage") {
			t.Fatalf("Gruß: %+v", z)
		}
	}
	// Das normale Einstellungsformular darf die Brieftexte nicht überschreiben
	a.st.mu.Lock()
	s := a.st.d.Settings
	a.st.mu.Unlock()
	s.Brieftexte = Brieftexte{}
	s.WasserPreis += 1
	if rec := do(h, "PUT", "/api/admin/settings", s); rec.Code != 200 {
		t.Fatalf("Einstellungen: %d", rec.Code)
	}
	a.st.mu.Lock()
	if a.st.d.Settings.Brieftexte.Gruss != "Herzliche Grüße aus der Anlage" {
		t.Error("Einstellungsformular hat den Brieftext überschrieben")
	}
	a.st.mu.Unlock()
	// Verlauf nennt die Änderung, Zurücknehmen stellt den Standard wieder her
	v := decode[[]einstellungsZeile](t, do(h, "GET", "/api/admin/einstellungen/verlauf", nil))
	gefunden := -1
	for _, z := range v {
		for _, x := range z.Aenderungen {
			if strings.Contains(x, "Grußformel") {
				gefunden = z.Index
			}
		}
	}
	if gefunden < 0 {
		t.Fatalf("Verlauf ohne Brieftext-Änderung: %+v", v)
	}
	// Gleicher Text wie Standard = zurück zum Standard
	do(h, "PUT", "/api/admin/brieftexte/gruss", map[string]string{"text": "Mit freundlichen Gartengrüßen"})
	a.st.mu.Lock()
	if a.st.d.Settings.Brieftexte.Gruss != "" {
		t.Error("Standardtext muss als leer gespeichert werden")
	}
	a.st.mu.Unlock()
}

// Alle Briefe entstehen auch mit ungewöhnlichen eigenen Texten (Prozentzeichen, leere Zeilen, sehr lang).
func TestBrieftexteInAllenBriefen(t *testing.T) {
	a, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", map[string]any{})
	do(h, "POST", "/api/invoices/issue", issueReq{})
	for _, v := range brieftextVorlagen {
		text := "Eigener Text mit 100 % Prozent und {Frist} {Seit} {Garten}.\n\n" + strings.Repeat("Lang ", 1000)
		if rec := do(h, "PUT", "/api/admin/brieftexte/"+v.Key, map[string]string{"text": text}); rec.Code != 400 {
			t.Fatalf("%s: zu langer Text muss abgelehnt werden: %d", v.Key, rec.Code)
		}
		// genau so lang wie erlaubt (mit eingesetzten Platzhaltern etwas länger) muss jeder Brief noch passen
		text = strings.TrimSpace(string([]rune(text)[:v.grenze()]))
		if rec := do(h, "PUT", "/api/admin/brieftexte/"+v.Key, map[string]string{"text": text}); rec.Code != 200 {
			t.Fatalf("%s: %d %s", v.Key, rec.Code, rec.Body.String())
		}
	}
	a.st.mu.Lock()
	if !strings.Contains(a.st.d.Settings.Brieftexte.text("mahnung2", nil), "100 % Prozent") {
		t.Error("Text nicht gespeichert")
	}
	var r *Rechnung
	for _, x := range a.st.d.Rechnungen {
		if x.Status == statusGueltig && openAmount(x) > 0 {
			r = x
			break
		}
	}
	a.st.mu.Unlock()
	if r == nil {
		t.Fatal("keine offene Rechnung zum Mahnen")
	}
	{
		a.st.mu.Lock()
		set := r.Settings
		set.Brieftexte = a.st.d.Settings.Brieftexte
		a.st.mu.Unlock()
		for stufe := 1; stufe <= 3; stufe++ {
			if _, err := buildMahnung(set, r.Paechter, r, stufe, ""); err != nil {
				t.Errorf("Mahnung %d: %v", stufe, err)
			}
		}
		kopie := *r
		kopie.Settings = set
		_, text := rechnungsMail(&kopie)
		if !strings.Contains(text, "Eigener Text") {
			t.Errorf("E-Mail ohne eigene Grußformel: %q", text[max(0, len(text)-200):])
		}
	}
	a.st.mu.Lock()
	s := a.st.d.Settings
	b := Bewerber{ID: "b", Name: "Hans", Seit: "2025-01-01"}
	a.st.mu.Unlock()
	for _, art := range []string{"angebot", "nachfrage", "bestaetigung"} {
		if _, err := buildWartelistenBrief(s, b, art, "7", "2026-05-01"); err != nil {
			t.Errorf("Warteliste %s: %v", art, err)
		}
	}
	if _, err := buildMaengelschreiben(s, Paechter{Name: "X", Gartennr: "1"}, []Mangel{{Art: "Sonstiges", Beschreibung: "Test", Frist: "2026-05-01"}}, "2026-06-01", "zweit"); err != nil {
		t.Errorf("Mängelschreiben: %v", err)
	}
}

// Probebrief: für jede Vorlage ein PDF mit dem eingegebenen Text, ohne ihn zu speichern.
func TestBrieftextProbebrief(t *testing.T) {
	a, h := newTestApp(t)
	for _, v := range brieftextVorlagen {
		rec := do(h, "POST", "/api/admin/brieftexte/"+v.Key+"/probe", map[string]string{"text": "Probetext {Frist} {Seit} 50 %"})
		if rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), "%PDF") {
			t.Errorf("%s: %d %.200s", v.Key, rec.Code, rec.Body.String())
		}
	}
	if rec := do(h, "POST", "/api/admin/brieftexte/mahnung2/probe", map[string]string{"text": strings.Repeat("x", 401)}); rec.Code != 400 {
		t.Errorf("zu lang: %d", rec.Code)
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if a.st.d.Settings.Brieftexte != (Brieftexte{}) {
		t.Error("Probebrief darf nichts speichern")
	}
}
