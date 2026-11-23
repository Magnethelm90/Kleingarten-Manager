package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func pdfAblegen(t *testing.T, name string, b []byte) {
	t.Helper()
	if !bytes.HasPrefix(b, []byte("%PDF")) {
		t.Fatalf("%s: kein PDF: %.200s", name, b)
	}
	if dir := os.Getenv("GA_PDF_DIR"); dir != "" {
		_ = os.WriteFile(filepath.Join(dir, name), b, 0o600)
	}
}

func TestKuendigungsFrist(t *testing.T) {
	r := KuendigungsRegel{}.mitStandard()
	for eingang, want := range map[string]string{
		"2026-03-01": "2026-11-30",
		"2026-08-31": "2026-11-30",
		"2026-09-01": "2027-11-30",
		"2026-12-15": "2027-11-30",
	} {
		if got := r.fristgerecht(eingang); got != want {
			t.Errorf("%s: %s, erwartet %s", eingang, got, want)
		}
	}
	if (KuendigungsRegel{Zum: "13-40"}).mitStandard().Zum != "11-30" {
		t.Error("ungültige Regel nicht ersetzt")
	}
}

func TestKuendigungMitFahrplan(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	app.st.mu.Lock()
	app.st.d.Kuendigungen = nil
	app.st.mu.Unlock()
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	var p Paechter
	for _, x := range st.Paechter {
		if x.Gartennr == "3" { // hat in den Beispieldaten einen Torschlüssel
			p = x
		}
	}
	rec := do(h, "POST", "/api/admin/kuendigungen", map[string]any{"paechterId": p.ID, "eingang": "2026-09-10", "zum": "2026-11-30", "von": "paechter", "grund": "Umzug"})
	if rec.Code != 200 {
		t.Fatalf("anlegen: %d %s", rec.Code, rec.Body.String())
	}
	k := decode[kuendigungAnsicht](t, rec)
	if !k.ZuFrueh || k.Fristgemaess != "2027-11-30" || k.Offen != len(kuendigungsSchritte) {
		t.Errorf("Ansicht: zuFrüh %v, fristgemäß %s, offen %d", k.ZuFrueh, k.Fristgemaess, k.Offen)
	}
	if rec := do(h, "POST", "/api/admin/kuendigungen", map[string]any{"paechterId": p.ID, "eingang": "2026-09-10", "zum": "2026-11-30"}); rec.Code != 400 {
		t.Errorf("zweite Kündigung: %d", rec.Code)
	}
	// Bestätigung hakt den ersten Schritt ab und nennt den Schlüssel
	rec = do(h, "GET", "/api/admin/kuendigungen/"+k.ID+"/bestaetigung", nil)
	pdfAblegen(t, "kuendigung.pdf", rec.Body.Bytes())
	// Schlüssel zurück, Wertermittlung angelegt: beides erkennt das Programm
	app.st.mu.Lock()
	for i := range app.st.d.Schluessel {
		if app.st.d.Schluessel[i].PaechterID == p.ID {
			app.st.d.Schluessel[i].PaechterID = ""
		}
	}
	app.st.d.Wertermittlungen = append(app.st.d.Wertermittlungen, Wertermittlung{ID: "w1", Gartennr: "3", Datum: "2026-10-01"})
	app.st.mu.Unlock()
	do(h, "POST", "/api/admin/kuendigungen/"+k.ID+"/schritt", map[string]any{"schritt": "raeumung", "erledigt": true})
	if rec := do(h, "POST", "/api/admin/kuendigungen/"+k.ID+"/schritt", map[string]any{"schritt": "quatsch", "erledigt": true}); rec.Code != 400 {
		t.Errorf("unbekannter Schritt: %d", rec.Code)
	}
	liste := decode[struct {
		Kuendigungen []kuendigungAnsicht `json:"kuendigungen"`
	}](t, do(h, "GET", "/api/admin/kuendigungen", nil)).Kuendigungen
	erledigt := map[string]string{}
	for _, x := range liste[0].Schritte {
		erledigt[x.Key] = x.Erledigt
	}
	if erledigt["bestaetigung"] == "" || erledigt["raeumung"] == "" || erledigt["wertermittlung"] != "automatisch" || erledigt["schluessel"] != "automatisch" || erledigt["warteliste"] != "" {
		t.Errorf("Schritte: %v", erledigt)
	}
	// Cockpit
	app.st.mu.Lock()
	var texte []string
	app.st.kuendigungAufgabenLocked(time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC), func(stufe, titel, text, ziel, knopf string) { texte = append(texte, stufe+" "+titel+" "+text) })
	ak, _ := app.st.auskunftLocked(p.ID)
	app.st.d.kuendigungPersonEntfernen(p.ID)
	rest := len(app.st.d.Kuendigungen)
	app.st.mu.Unlock()
	if len(texte) != 1 || !strings.HasPrefix(texte[0], "warn") || !strings.Contains(texte[0], "Nachfolger angefragt") {
		t.Errorf("Cockpit: %v", texte)
	}
	if len(ak.Kuendigung) != 1 || rest != 0 {
		t.Errorf("Datenschutz: Auskunft %d, nach dem Löschen %d", len(ak.Kuendigung), rest)
	}
}

func TestPachtvertrag(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	p := st.Paechter[0]
	rec := do(h, "GET", "/api/admin/pachtvertrag?paechter="+p.ID+"&beginn=2024-04-01", nil)
	if rec.Code != 200 {
		t.Fatalf("Pachtvertrag: %d %s", rec.Code, rec.Body.String())
	}
	pdfAblegen(t, "pachtvertrag.pdf", rec.Body.Bytes())
	if rec := do(h, "GET", "/api/admin/pachtvertrag?paechter=gibtsnicht", nil); rec.Code != 404 {
		t.Errorf("unbekannter Pächter: %d", rec.Code)
	}
	// eigene Bestimmungen
	do(h, "PUT", "/api/admin/vertragsbestimmungen", map[string]string{"text": "§ Hunde\nHunde sind an der Leine zu führen."})
	app.st.mu.Lock()
	text := app.st.vertragsbestimmungenLocked()
	app.st.mu.Unlock()
	if !strings.Contains(text, "Hunde") {
		t.Errorf("Bestimmungen: %q", text)
	}
	// in die Gartenakte
	rec = do(h, "POST", "/api/admin/pachtvertrag/ablegen?paechter="+p.ID, nil)
	doc := decode[GartenDokument](t, rec)
	if doc.Art != "Pachtvertrag" || doc.PaechterID != p.ID || doc.Gartennr != p.Gartennr {
		t.Fatalf("Dokument: %+v", doc)
	}
	if raw, err := os.ReadFile(filepath.Join(app.st.dir, filepath.FromSlash(doc.Datei))); err != nil || !bytes.HasPrefix(raw, []byte("%PDF")) {
		t.Errorf("Datei: %v", err)
	}
}

func TestBeschluesseVertraegeHaushalt(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	// Beschlüsse: fortlaufende Nummer je Gremium und Jahr
	for i := 0; i < 2; i++ {
		do(h, "POST", "/api/admin/beschluesse", map[string]any{"datum": "2030-04-01", "gremium": "Vorstand", "titel": fmt.Sprint("Test ", i)})
	}
	bs := decode[[]Beschluss](t, do(h, "GET", "/api/admin/beschluesse", nil))
	if bs[0].Datum != "2030-04-01" || bs[0].Nummer != "V 2030/2" && bs[1].Nummer != "V 2030/2" {
		t.Errorf("Nummern: %+v", bs[:2])
	}
	if rec := do(h, "POST", "/api/admin/beschluesse", map[string]any{"datum": "2030-04-01"}); rec.Code != 400 {
		t.Errorf("ohne Titel: %d", rec.Code)
	}
	rec := do(h, "GET", "/api/admin/beschlussbuch", nil)
	pdfAblegen(t, "beschlussbuch.pdf", rec.Body.Bytes())

	// Verträge: Verlängerung und Kündigungstermin
	v := Vertrag{Ende: "2020-12-31", FristMonate: 3, Verlaengert: 12, Kosten: 10, Turnus: "monatlich"}
	a := v.ansicht(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if a.LaeuftBis != "2026-12-31" || a.KuendigenBis != "2026-09-30" || a.Jaehrlich != 120 {
		t.Errorf("Ansicht: %+v", a)
	}
	if rec := do(h, "POST", "/api/admin/vertraege", map[string]any{"bezeichnung": "x", "fristMonate": -1}); rec.Code != 400 {
		t.Errorf("negative Frist: %d", rec.Code)
	}
	app.st.mu.Lock()
	var texte []string
	app.st.vertragAufgabenLocked(time.Now(), func(stufe, titel, text, ziel, knopf string) { texte = append(texte, titel) })
	app.st.mu.Unlock()
	if !strings.Contains(fmt.Sprint(texte), "Stromliefervertrag Anlage: bis") {
		t.Errorf("Cockpit Verträge: %v", texte)
	}

	// Haushaltsplan: Soll speichern, Ist aus dem Kassenbericht
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	jahr := fmt.Sprint(st.CurrentYear)
	rec = do(h, "PUT", "/api/admin/haushalt?year="+jahr, map[string]any{"einnahmen": map[string]float64{"Pacht": 200}, "ausgaben": map[string]float64{"Instandhaltung": 100}})
	hh := decode[haushaltAnsicht](t, rec)
	var pacht, instand haushaltZeile
	for _, z := range hh.Einnahmen {
		if z.Posten == "Pacht" {
			pacht = z
		}
	}
	for _, z := range hh.Ausgaben {
		if z.Posten == "Instandhaltung" {
			instand = z
		}
	}
	if pacht.Soll == nil || *pacht.Soll != 200 || pacht.Ist <= 0 || instand.Ist != 145.5 {
		t.Errorf("Haushalt: Pacht %+v, Instandhaltung %+v", pacht, instand)
	}
	if rec := do(h, "PUT", "/api/admin/haushalt?year="+jahr, map[string]any{"einnahmen": map[string]float64{"Pacht": -5}}); rec.Code != 400 {
		t.Errorf("negativer Plan: %d", rec.Code)
	}
	pdfAblegen(t, "haushalt.pdf", do(h, "GET", "/api/admin/haushalt/pdf?year="+jahr, nil).Body.Bytes())

	// Schwund: Beispieldaten haben einen Hauptzähler mit gut 14 % mehr als die Gärten
	sw := decode[schwundAntwort](t, do(h, "GET", "/api/admin/schwund?year="+jahr, nil))
	var w schwundWerte
	for _, j := range sw.Jahre {
		if fmt.Sprint(j.Jahr) == jahr {
			w = j.Wasser
		}
	}
	if w.Prozent == nil || *w.Prozent < 10 || w.Kosten == nil || *w.Kosten <= 0 {
		t.Fatalf("Schwund: %+v", w)
	}
	app.st.mu.Lock()
	// alle Stände da: dann meldet sich das Cockpit
	for _, p := range app.st.d.Paechter {
		j := app.st.d.Jahre[jahr]
		a := j.Ablesungen[p.ID]
		if a.WasserAkt == nil && a.WasserVJ != nil {
			x := *a.WasserVJ + 1
			a.WasserAkt = &x
		}
		if a.StromAkt == nil && a.StromVJ != nil {
			x := *a.StromVJ + 1
			a.StromAkt = &x
		}
		if a.Stunden == nil {
			x := 12.0
			a.Stunden = &x
		}
		j.Ablesungen[p.ID] = a
	}
	texte = nil
	app.st.schwundAufgabenLocked(time.Now(), func(stufe, titel, text, ziel, knopf string) { texte = append(texte, titel) })
	app.st.mu.Unlock()
	if len(texte) != 1 || !strings.Contains(texte[0], "Wasserschwund") {
		t.Errorf("Cockpit Schwund: %v", texte)
	}
}
