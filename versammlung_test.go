package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestVersammlungPDFs(t *testing.T) {
	_, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	tr := decode[Termin](t, do(h, "POST", "/api/admin/termine", map[string]any{
		"datum": "2027-03-20", "von": "10:00", "titel": "Jahreshauptversammlung 2027", "art": "Versammlung", "ort": "Vereinsheim",
		"tops": "  Begrüßung \n\n Kassenbericht\r\nWahl des   Vorstands  ",
	}))
	if tr.Tops != "Begrüßung\nKassenbericht\nWahl des Vorstands" {
		t.Errorf("Tagesordnung nicht bereinigt: %q", tr.Tops)
	}
	for _, pfad := range []string{"einladung", "anwesenheit"} {
		rec := do(h, "GET", "/api/admin/termine/"+tr.ID+"/"+pfad, nil)
		if rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
			t.Fatalf("%s: %d %s", pfad, rec.Code, rec.Body.String())
		}
		if os.Getenv("GA_PDF_DIR") != "" {
			_ = os.WriteFile(os.Getenv("GA_PDF_DIR")+"/versammlung_"+pfad+".pdf", rec.Body.Bytes(), 0o600)
		}
		if do(h, "GET", "/api/admin/termine/gibtesnicht/"+pfad, nil).Code != 404 {
			t.Errorf("%s: unbekannter Termin sollte 404 liefern", pfad)
		}
	}
	// ohne Tagesordnung: Standardpunkte nur bei Versammlungen
	if len(tagesordnung(Termin{Art: "Versammlung"})) != len(standardTops) || tagesordnung(Termin{Art: "Fest"}) != nil {
		t.Error("Standard-Tagesordnung")
	}
	// viele Pächter: Anwesenheitsliste über mehrere Seiten
	for i := 0; i < 60; i++ {
		do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "X" + strings.Repeat("1", i%5) + string(rune('A'+i%26)) + string(rune('a'+i/26)), "gartennr": "9" + string(rune('A'+i%26)) + string(rune('a'+i/26)), "name": "Test Pächter", "gartengroesse": 100})
	}
	if rec := do(h, "GET", "/api/admin/termine/"+tr.ID+"/anwesenheit", nil); rec.Code != 200 {
		t.Errorf("lange Liste: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/admin/termine/"+tr.ID+"/einladung/oeffnen", nil); rec.Code != 200 {
		t.Errorf("öffnen: %d", rec.Code)
	}
	// nur für den Admin
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	for _, pfad := range []string{"einladung", "anwesenheit"} {
		if rec := do(h, "GET", "/api/admin/termine/"+tr.ID+"/"+pfad, nil); rec.Code != 401 {
			t.Errorf("%s ohne Anmeldung: %d", pfad, rec.Code)
		}
	}
}

// Eine lange Tagesordnung wird auf der nächsten Seite fortgesetzt statt abgeschnitten.
func TestEinladungLangeTagesordnung(t *testing.T) {
	var tops []string
	for i := 0; i < 20; i++ {
		tops = append(tops, strings.Repeat("Sehr langer Tagesordnungspunkt mit viel Text ", 3))
	}
	tr, err := cleanTermin(Termin{Datum: "2027-03-20", Titel: "JHV", Art: "Versammlung", Tops: strings.Join(tops, "\n"), Notiz: strings.Repeat("Hinweis ", 30)})
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := buildEinladung(defaultSettings(), tr)
	if err != nil || bytes.Count(pdf, []byte("/Type /Page\n")) < 2 {
		t.Errorf("lange Tagesordnung: %v, Seiten: %d", err, bytes.Count(pdf, []byte("/Type /Page\n")))
	}
}

func TestSafeNameUnsichtbareZeichen(t *testing.T) {
	for _, in := range []string{"Einladung_‮fdp.exe", "a\x7fb", "x/../y", "Ga\"rten\r\n1"} {
		got := safeName(in, 80)
		for _, r := range got {
			if r < 0x20 || r == 0x7f || r == '‮' || strings.ContainsRune(`/\"`, r) {
				t.Errorf("%q → %q enthält %U", in, got, r)
			}
		}
	}
}
