package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestEichStatus(t *testing.T) {
	for _, c := range []struct {
		eich, frist, jahr, bis int
		status                 string
	}{
		{0, 6, 2026, 0, "unbekannt"},
		{2019, 6, 2026, 2025, "abgelaufen"},
		{2020, 6, 2026, 2026, "faellig"},
		{2021, 6, 2026, 2027, "bald"},
		{2024, 6, 2026, 2030, "ok"},
	} {
		bis, st := eichStatus(c.eich, c.frist, c.jahr)
		if bis != c.bis || st != c.status {
			t.Errorf("%d+%d in %d: %d %s, erwartet %d %s", c.eich, c.frist, c.jahr, bis, st, c.bis, c.status)
		}
	}
}

func TestEichungUndZaehlerwechsel(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	d := decode[eichAntwort](t, do(h, "GET", "/api/admin/eichung", nil))
	if d.Fristen.Wasser != 6 || d.Fristen.Strom != 8 || len(d.Zeilen) == 0 || d.Zeilen[0].Status != "faellig" {
		t.Fatalf("Eichung: %+v", d.Fristen)
	}
	pdfAblegen(t, "tauschliste.pdf", do(h, "GET", "/api/admin/eichung/tauschliste", nil).Body.Bytes())
	if rec := do(h, "PUT", "/api/admin/eichung/fristen", map[string]int{"wasser": 0, "strom": 8}); rec.Code != 400 {
		t.Errorf("Frist 0 angenommen: %d", rec.Code)
	}
	id := d.Zeilen[0].PaechterID
	if rec := do(h, "PUT", "/api/admin/eichung/"+id, map[string]any{"art": "wasser", "eichjahr": 1800}); rec.Code != 400 {
		t.Errorf("Eichjahr 1800 angenommen: %d", rec.Code)
	}
	// Zählerwechsel mit neuer Nummer: Eichjahr wird das laufende Jahr
	app.st.mu.Lock()
	jahr := app.st.d.Settings.Jahr
	app.st.mu.Unlock()
	rec := do(h, "PUT", fmt.Sprintf("/api/ablesung/%s?year=%d", id, jahr), map[string]any{"wasserWechsel": map[string]any{"altEnde": 10, "neueNr": "W-NEU", "neuStart": 0}})
	if rec.Code != 200 {
		t.Fatalf("Ablesung: %d %s", rec.Code, rec.Body.String())
	}
	app.st.mu.Lock()
	defer app.st.mu.Unlock()
	for _, p := range app.st.d.Paechter {
		if p.ID == id && (p.WasserzaehlerNr != "W-NEU" || p.WasserEichjahr != time.Now().Year()) {
			t.Errorf("nach Wechsel: %s %d", p.WasserzaehlerNr, p.WasserEichjahr)
		}
	}
}

func TestKassenpruefung(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	app.st.mu.Lock()
	jahr := app.st.d.Settings.Jahr
	app.st.mu.Unlock()
	url := fmt.Sprintf("/api/admin/kassenpruefung?year=%d", jahr)
	x := decode[pruefAnsicht](t, do(h, "GET", url, nil))
	if len(x.Punkte) != len(pruefPunkte) || x.Differenz != nil {
		t.Fatalf("Ansicht: %d Punkte, Differenz %v", len(x.Punkte), x.Differenz)
	}
	x = decode[pruefAnsicht](t, do(h, "POST", fmt.Sprintf("/api/admin/kassenpruefung/stichprobe?year=%d", jahr), nil))
	if want := min(max(5, (x.Ausgaben+9)/10), x.Ausgaben); len(x.Stichprobe) != want {
		t.Errorf("Stichprobe: %d von %d, erwartet %d", len(x.Stichprobe), x.Ausgaben, want)
	}
	konto := x.Kassenbestand + 12.5
	rec := do(h, "PUT", url, map[string]any{"datum": fmt.Sprintf("%d-02-20", jahr+1), "pruefer": "Uwe Brandt, Karin Vogel", "kontostand": konto,
		"punkte": map[string]bool{"belege": true, "unbekannt": true}, "ergebnis": "ohne", "entlastung": true, "stichprobe": []string{"x"}})
	if rec.Code != 200 {
		t.Fatalf("speichern: %d %s", rec.Code, rec.Body.String())
	}
	x = decode[pruefAnsicht](t, rec)
	if x.Differenz == nil || *x.Differenz != 12.5 || x.Pruefung.Punkte["unbekannt"] || !x.Pruefung.Punkte["belege"] || len(x.Pruefung.Stichprobe) == 0 || x.Pruefung.Stichprobe[0] == "x" {
		t.Errorf("gespeichert: %+v", x.Pruefung)
	}
	b := do(h, "GET", fmt.Sprintf("/api/admin/kassenpruefung/bericht?year=%d", jahr), nil).Body.Bytes()
	pdfAblegen(t, "pruefbericht.pdf", b)
	if rec := do(h, "PUT", url, map[string]any{"datum": "gestern"}); rec.Code != 400 {
		t.Errorf("ungültiges Datum angenommen: %d", rec.Code)
	}
}

func TestAemterUndWechsel(t *testing.T) {
	app, h := newTestApp(t)
	a := decode[aemterAntwort](t, do(h, "POST", "/api/admin/aemter", map[string]any{"amt": "Vorsitz", "name": "Anna Alt", "seit": "2022-03-01", "bis": "2026-03-01"}))
	if len(a.Aemter) != 1 {
		t.Fatalf("anlegen: %+v", a)
	}
	if rec := do(h, "POST", "/api/admin/aemter", map[string]any{"amt": "Vorsitz", "name": "X", "seit": "2026-03-01", "bis": "2025-01-01"}); rec.Code != 400 {
		t.Errorf("Ende vor Beginn angenommen: %d", rec.Code)
	}
	alt := a.Aemter[0]
	a = decode[aemterAntwort](t, do(h, "POST", "/api/admin/aemter/"+alt.ID+"/wechsel", map[string]any{"name": "Bernd Neu", "seit": "2026-02-20", "bis": "2030-02-20", "amt": "Kasse"}))
	if len(a.Aemter) != 2 || a.Aemter[0].Name != "Bernd Neu" || a.Aemter[0].Amt != "Vorsitz" || !a.Aemter[1].Ehemalig || a.Aemter[1].Bis != "2026-02-20" {
		t.Fatalf("Wechsel: %+v", a.Aemter)
	}
	neu := a.Aemter[0]
	if rec := do(h, "POST", "/api/admin/aemter/"+alt.ID+"/wechsel", map[string]any{"name": "Y", "seit": "2026-03-01"}); rec.Code != 404 {
		t.Errorf("Wechsel auf ehemaliges Amt: %d", rec.Code)
	}
	if rec := do(h, "PUT", "/api/admin/aemter/"+neu.ID+"/checkliste", map[string]any{"key": "boese", "erledigt": true}); rec.Code != 400 {
		t.Errorf("unbekannter Punkt: %d", rec.Code)
	}
	a = decode[aemterAntwort](t, do(h, "PUT", "/api/admin/aemter/"+neu.ID+"/checkliste", map[string]any{"key": "bank", "erledigt": true}))
	if a.Aemter[0].Checkliste["bank"] == "" {
		t.Error("Haken nicht gesetzt")
	}
	app.st.mu.Lock()
	var titel []string
	app.st.aemterAufgabenLocked(time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local), func(_, t, _, _, _ string) { titel = append(titel, t) })
	app.st.mu.Unlock()
	if len(titel) != 1 || !strings.Contains(titel[0], "6 Punkte offen") {
		t.Errorf("Cockpit: %v", titel)
	}
}

func TestPachtgrenze(t *testing.T) {
	app, h := newTestApp(t)
	app.st.mu.Lock()
	app.st.d.Settings.PachtJeQm = 0.15
	app.st.mu.Unlock()
	g := decode[pachtgrenzeAnsicht](t, do(h, "PUT", "/api/admin/pachtgrenze", map[string]any{"erwerbsobstbauJeQm": 0.035, "quelle": "GA"}))
	if g.Hoechst == nil || *g.Hoechst != 0.14 || !g.Ueber || *g.Auslastung != 107.1 {
		t.Fatalf("Grenze: %+v", g)
	}
	if rec := do(h, "PUT", "/api/admin/pachtgrenze", map[string]any{"erwerbsobstbauJeQm": -1}); rec.Code != 400 {
		t.Errorf("negativ angenommen: %d", rec.Code)
	}
	g = decode[pachtgrenzeAnsicht](t, do(h, "PUT", "/api/admin/pachtgrenze", map[string]any{"erwerbsobstbauJeQm": 0}))
	if g.Hoechst != nil || g.Ueber {
		t.Errorf("ohne Wert: %+v", g)
	}
}

func TestEichjahrBleibtBeimZaehler(t *testing.T) {
	s := &Store{d: newData()}
	s.d.Paechter = []Paechter{{ID: "alt", WasserzaehlerNr: "W-1", WasserEichjahr: 2021, StromzaehlerNr: "S-1", StromEichjahr: 2019, Geloescht: true}}
	neu := Paechter{ID: "neu", WasserzaehlerNr: "W-1", StromzaehlerNr: "S-9"}
	s.eichjahrUebernehmenLocked(&neu)
	if neu.WasserEichjahr != 2021 || neu.StromEichjahr != 0 {
		t.Errorf("übernommen: Wasser %d, Strom %d", neu.WasserEichjahr, neu.StromEichjahr)
	}
}

func TestBarkasse(t *testing.T) {
	app, h := newTestApp(t)
	app.st.mu.Lock()
	jahr := app.st.d.Settings.Jahr
	app.st.ensureYear(jahr - 1)
	app.st.mu.Unlock()
	u := func(p string, y int) string { return fmt.Sprintf("/api/admin/barkasse%s?year=%d", p, y) }
	do(h, "PUT", u("/anfang", jahr-1), map[string]any{"betrag": 50})
	do(h, "POST", u("", jahr-1), map[string]any{"datum": fmt.Sprintf("%d-06-01", jahr-1), "text": "Fest", "betrag": 20})
	b := decode[barkasseAnsicht](t, do(h, "GET", u("", jahr), nil))
	if !b.AnfangVJ || b.Anfang != 70 {
		t.Fatalf("Anfang aus Vorjahr: %+v", b)
	}
	b = decode[barkasseAnsicht](t, do(h, "POST", u("", jahr), map[string]any{"datum": fmt.Sprintf("%d-03-01", jahr), "text": "Erde", "betrag": -12.5}))
	b = decode[barkasseAnsicht](t, do(h, "POST", u("", jahr), map[string]any{"datum": fmt.Sprintf("%d-02-01", jahr), "text": "Pfand", "betrag": 5}))
	if b.Bestand != 62.5 || len(b.Buchungen) != 2 || b.Buchungen[0].BelegNr != 2 || b.Laufend[0] != 75 {
		t.Fatalf("Buchungen: %+v", b)
	}
	// gelöschte Belegnummern werden nicht neu vergeben
	b = decode[barkasseAnsicht](t, do(h, "DELETE", "/api/admin/barkasse/"+b.Buchungen[0].ID+fmt.Sprintf("?year=%d", jahr), nil))
	b = decode[barkasseAnsicht](t, do(h, "POST", u("", jahr), map[string]any{"datum": fmt.Sprintf("%d-04-01", jahr), "text": "X", "betrag": 1}))
	if b.Buchungen[len(b.Buchungen)-1].BelegNr != 3 {
		t.Errorf("Belegnummer nach Löschen: %+v", b.Buchungen)
	}
	if rec := do(h, "POST", u("", jahr), map[string]any{"datum": "2025-01-01", "text": "Null", "betrag": 0}); rec.Code != 400 {
		t.Errorf("Betrag 0 angenommen: %d", rec.Code)
	}
	// Kassenprüfung: gezähltes Bargeld gegen den Bestand des Kassenbuchs
	x := decode[pruefAnsicht](t, do(h, "GET", fmt.Sprintf("/api/admin/kassenpruefung?year=%d", jahr), nil))
	x = decode[pruefAnsicht](t, do(h, "PUT", fmt.Sprintf("/api/admin/kassenpruefung?year=%d", jahr), map[string]any{"kontostand": x.Kassenbestand, "barkasse": 60.5}))
	if x.BarSoll == nil || *x.BarSoll != 58.5 || x.Differenz == nil || *x.Differenz != 2 {
		t.Errorf("Prüfung mit Barkasse: soll %v, diff %v", x.BarSoll, x.Differenz)
	}
	pdfAblegen(t, "kassenbuch.pdf", do(h, "GET", u("/kassenbuch", jahr), nil).Body.Bytes())
}

func TestLaubenkataster(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "gartennr": "7", "name": "Anna", "gartengroesse": 300})
	do(h, "POST", "/api/admin/bauten", map[string]any{"gartennr": "7", "art": "Laube", "flaeche": 20})
	a := decode[bautenAntwort](t, do(h, "POST", "/api/admin/bauten", map[string]any{"gartennr": "7", "art": "Überdachter Freisitz", "flaeche": 6, "genehmigt": "nein"}))
	if len(a.Gaerten) != 1 || a.Gaerten[0].Laube != 26 || !a.Gaerten[0].Ueber || a.Gaerten[0].Ungenehmigt != 1 || a.Gaerten[0].Name != "Anna" {
		t.Fatalf("Kataster: %+v", a.Gaerten)
	}
	if rec := do(h, "POST", "/api/admin/bauten", map[string]any{"gartennr": "7", "art": "Laube", "flaeche": -1}); rec.Code != 400 {
		t.Errorf("negative Fläche angenommen: %d", rec.Code)
	}
	app.st.mu.Lock()
	var titel []string
	app.st.bautenAufgabenLocked(time.Now(), func(_, t, _, _, _ string) { titel = append(titel, t) })
	app.st.mu.Unlock()
	if len(titel) != 2 {
		t.Errorf("Cockpit: %v", titel)
	}
	// Bestandsschutz hebt die Warnung auf
	laube := a.Gaerten[0].Bauten[0]
	laube.Bestandsschutz = true
	a = decode[bautenAntwort](t, do(h, "POST", "/api/admin/bauten", laube))
	if a.Gaerten[0].Ueber {
		t.Error("trotz Bestandsschutz zu groß")
	}
	pdfAblegen(t, "laubenkataster.pdf", do(h, "GET", "/api/admin/bauten/liste", nil).Body.Bytes())
}

func TestSitzungsprotokoll(t *testing.T) {
	app, h := newTestApp(t)
	tr := decode[Termin](t, do(h, "POST", "/api/admin/termine", map[string]any{"datum": "2026-03-14", "titel": "Jahreshauptversammlung", "art": "Versammlung", "von": "18:00"}))
	d := decode[sitzungAntwort](t, do(h, "GET", "/api/admin/termine/"+tr.ID+"/protokoll", nil))
	if !d.Neu || len(d.Protokoll.Punkte) != len(standardTops) || d.Protokoll.Beginn != "18:00" {
		t.Fatalf("Vorlage: %+v", d.Protokoll)
	}
	p := d.Protokoll
	p.Punkte[5].Text = "Der Vorstand wird entlastet."
	p.Punkte[5].Beschluss = &Abstimmung{Titel: "Entlastung des Vorstands 2025", Ja: 30, Nein: 1, Enthaltung: 4}
	p.Punkte[6].Beschluss = &Abstimmung{Titel: "Wasserpreis 2,10 €", Ja: 20, Nein: 15}
	d = decode[sitzungAntwort](t, do(h, "PUT", "/api/admin/termine/"+tr.ID+"/protokoll", p))
	app.st.mu.Lock()
	n := len(app.st.d.Beschluesse)
	app.st.mu.Unlock()
	id := d.Protokoll.Punkte[5].BeschlussID
	if n != 2 || id == "" || d.Nummern[id] != "MV 2026/1" || d.Nummern[d.Protokoll.Punkte[6].BeschlussID] != "MV 2026/2" {
		t.Fatalf("Beschlussbuch: %d %v", n, d.Nummern)
	}
	// Abstimmung entfernen löscht den Beschluss, Änderung behält die Nummer
	p = d.Protokoll
	p.Punkte[6].Beschluss = nil
	p.Punkte[5].Beschluss.Ja = 31
	d = decode[sitzungAntwort](t, do(h, "PUT", "/api/admin/termine/"+tr.ID+"/protokoll", p))
	app.st.mu.Lock()
	bs := append([]Beschluss(nil), app.st.d.Beschluesse...)
	app.st.mu.Unlock()
	if len(bs) != 1 || bs[0].Ja != 31 || bs[0].Nummer != "MV 2026/1" || d.Protokoll.Punkte[6].BeschlussID != "" {
		t.Fatalf("nach Änderung: %+v", bs)
	}
	// fremde Beschluss-IDs werden nicht übernommen
	p = d.Protokoll
	p.Punkte[0].BeschlussID = "fremd"
	d = decode[sitzungAntwort](t, do(h, "PUT", "/api/admin/termine/"+tr.ID+"/protokoll", p))
	if d.Protokoll.Punkte[0].BeschlussID != "" {
		t.Error("fremde ID übernommen")
	}
	pdfAblegen(t, "protokoll.pdf", do(h, "GET", "/api/admin/termine/"+tr.ID+"/protokoll/pdf", nil).Body.Bytes())
	if rec := do(h, "DELETE", "/api/admin/termine/"+tr.ID, nil); rec.Code != 400 {
		t.Errorf("Termin mit Protokoll gelöscht: %d", rec.Code)
	}
}
