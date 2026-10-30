package main

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"
)

// wechselAnlegen legt einen Pächter mit Vorjahresständen an und gibt seine ID zurück.
func wechselAnlegen(t *testing.T, h http.Handler) string {
	t.Helper()
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{
		"mitgliedsnr": "7", "gartennr": "7", "name": "Alt Pächter", "strasse": "Weg 1", "plzOrt": "12345 Ort",
		"gartengroesse": 300, "wasserzaehlerNr": "W-7", "stromzaehlerNr": "S-7",
	}))
	if rec := do(h, "PUT", "/api/ablesung/"+p.ID, map[string]any{"wasserVJ": 100, "stromVJ": 1000, "versicherung": 5}); rec.Code != 200 {
		t.Fatalf("Ablesung: %d %s", rec.Code, rec.Body.String())
	}
	return p.ID
}

func wechselReqFuer(altID string) map[string]any {
	return map[string]any{
		"altId": altID, "datum": "2025-06-15", "wasserStand": 130, "stromStand": 1200, "stunden": 4,
		"neu":     map[string]any{"mitgliedsnr": "7", "anrede": "Frau", "name": "Neu Pächterin", "strasse": "Allee 2", "plzOrt": "12345 Ort", "versand": "Postversand"},
		"abloese": 1500, "schluessel": "2 Torschlüssel, 1 Laubenschlüssel", "bemerkungen": "Laube in gutem Zustand.",
	}
}

func TestPaechterwechsel(t *testing.T) {
	app, h := newTestApp(t)
	altID := wechselAnlegen(t, h)
	bew := decode[Bewerber](t, do(h, "POST", "/api/admin/warteliste", map[string]any{"name": "Neu Pächterin", "seit": "2024-01-10"}))

	req := wechselReqFuer(altID)
	if rec := do(h, "POST", "/api/admin/paechterwechsel", req); rec.Code != 400 || !strings.Contains(rec.Body.String(), "behält sie bis zum Jahreswechsel") {
		t.Fatalf("gleiche Mitgliedsnummer muss abgelehnt werden: %d %s", rec.Code, rec.Body.String())
	}
	req["neu"].(map[string]any)["mitgliedsnr"] = "7a"
	req["wasserStand"] = 90
	if rec := do(h, "POST", "/api/admin/paechterwechsel", req); rec.Code != 400 {
		t.Fatalf("Stand unter Vorjahr: %d", rec.Code)
	}
	req["wasserStand"] = 130
	req["wartelisteId"] = bew.ID
	rec := do(h, "POST", "/api/admin/paechterwechsel", req)
	if rec.Code != 200 {
		t.Fatalf("Wechsel: %d %s", rec.Code, rec.Body.String())
	}
	res := decode[struct {
		Uebergabe Uebergabe `json:"uebergabe"`
		Neu       Paechter  `json:"neu"`
	}](t, rec)
	neu := res.Neu
	if neu.Gartennr != "7" || neu.Gartengroesse != 300 || neu.WasserzaehlerNr != "W-7" || neu.StromzaehlerNr != "S-7" {
		t.Errorf("neuer Pächter übernimmt Garten und Zähler nicht: %+v", neu)
	}
	if rec := do(h, "POST", "/api/admin/paechterwechsel", req); rec.Code != 400 {
		t.Errorf("zweiter Wechsel für denselben Pächter: %d", rec.Code)
	}

	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	var alt Paechter
	for _, p := range st.Paechter {
		if p.ID == altID {
			alt = p
		}
	}
	if alt.AusgeschiedenAm != "2025-06-15" {
		t.Fatalf("bisheriger Pächter nicht als ausgeschieden markiert: %+v", alt)
	}
	r := st.Results[altID]
	// 30 m³ und 200 kWh, 8 fehlende Pflichtstunden; keine Beiträge, aber die Versicherung bleibt
	s := st.Settings
	want := round2(round2(s.WasserGrundpreis+30*s.WasserPreis) + round2(s.EnergieGrundpreis+200*s.EnergiePreis) + (s.Pflichtstunden-4)*s.NachzahlungJeStd + 5)
	if !r.Vollstaendig || r.PachtGarten != 0 || r.Mitgliedsbeitrag != 0 || r.Umlage != 0 || r.Gesamt != want {
		t.Errorf("Schlussrechnung: %+v, erwartet Gesamt %v", r, want)
	}
	if a := st.Ablesungen[neu.ID]; a.WasserVJ == nil || *a.WasserVJ != 130 || *a.StromVJ != 1200 || a.Schluss != "" {
		t.Errorf("neuer Pächter startet nicht mit den Übergabeständen: %+v", a)
	}
	if r := st.Results[neu.ID]; r.PachtGarten == 0 {
		t.Error("neuer Pächter muss die Beiträge fürs Folgejahr zahlen")
	}
	// Bearbeiten der Stammdaten lässt den Vermerk bestehen
	alt.Strasse = "Neuer Weg 5"
	alt.AusgeschiedenAm = ""
	if rec := do(h, "PUT", "/api/admin/paechter/"+altID, alt); rec.Code != 200 || decode[Paechter](t, rec).AusgeschiedenAm != "2025-06-15" {
		t.Errorf("Bearbeiten löscht den Ausgeschieden-Vermerk: %d %s", rec.Code, rec.Body.String())
	}
	// Ändern der Zählerstände über die normale Eingabe lässt die Schlussrechnung bestehen
	do(h, "PUT", "/api/ablesung/"+altID, map[string]any{"wasserVJ": 100, "wasserAkt": 131, "stromVJ": 1000, "stromAkt": 1200, "stunden": 4, "versicherung": 5})
	app.st.mu.Lock()
	if app.st.d.Jahre["2025"].Ablesungen[altID].Schluss == "" {
		t.Error("Schluss-Vermerk ging beim Speichern der Ablesung verloren")
	}
	if len(app.st.d.Warteliste) != 0 {
		t.Error("Eintrag auf der Warteliste wurde nicht entfernt")
	}
	var hist []GartenEintrag
	for _, e := range app.st.d.GartenHistorie {
		if e.Gartennr == "7" {
			hist = append(hist, e)
		}
	}
	app.st.mu.Unlock()
	if len(hist) != 2 || hist[0].Bis != "2025-06-15" || hist[1].Seit != "2025-06-15" || hist[1].PaechterID != neu.ID {
		t.Errorf("Gartenhistorie: %+v", hist)
	}

	// Schlussrechnung ausstellen
	if rec := do(h, "POST", "/api/invoices/issue", issueReq{IDs: []string{altID}}); rec.Code != 200 || strings.Contains(rec.Body.String(), `"reason"`) {
		t.Fatalf("Ausstellen: %d %s", rec.Code, rec.Body.String())
	}
	st = decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if info, ok := st.Issued[altID]; !ok || info.Geaendert {
		t.Errorf("Schlussrechnung ausgestellt, darf nicht als geändert gelten: %+v", info)
	}

	// Übergabeprotokoll
	if l := decode[[]Uebergabe](t, do(h, "GET", "/api/admin/uebergaben", nil)); len(l) != 1 || l[0].AltName != "Alt Pächter" || l[0].NeuName != "Neu Pächterin" {
		t.Fatalf("Übergaben: %+v", l)
	}
	pdf := do(h, "GET", "/api/admin/uebergabe/"+res.Uebergabe.ID+"/protokoll", nil)
	if pdf.Code != 200 || !bytes.HasPrefix(pdf.Body.Bytes(), []byte("%PDF")) {
		t.Fatalf("Protokoll: %d", pdf.Code)
	}
	if rec := do(h, "GET", "/api/admin/uebergabe/gibtsnicht/protokoll", nil); rec.Code != 404 {
		t.Errorf("unbekanntes Protokoll: %d", rec.Code)
	}

	// Jahreswechsel: der bisherige Pächter kommt in den Papierkorb, der neue wird weitergeführt
	if rec := do(h, "POST", "/api/admin/jahreswechsel", nil); rec.Code != 200 {
		t.Fatalf("Jahreswechsel: %d %s", rec.Code, rec.Body.String())
	}
	st = decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if len(st.Paechter) != 1 || st.Paechter[0].ID != neu.ID {
		t.Fatalf("nach dem Jahreswechsel: %+v", st.Paechter)
	}
	if a := st.Ablesungen[neu.ID]; a.WasserVJ == nil || *a.WasserVJ != 130 {
		t.Errorf("Vorjahresstand des neuen Pächters: %+v", a)
	}
	if _, ok := st.Ablesungen[altID]; ok {
		t.Error("ausgeschiedener Pächter darf im neuen Jahr keine Ablesung haben")
	}
	alt2 := decode[[]Paechter](t, do(h, "GET", "/api/admin/paechter-papierkorb", nil))
	if len(alt2) != 1 || alt2[0].ID != altID {
		t.Fatalf("Papierkorb: %+v", alt2)
	}
	// das abgeschlossene Jahr enthält ihn noch (Rechnung nachdruckbar)
	old := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	gefunden := false
	for _, p := range old.Paechter {
		gefunden = gefunden || p.ID == altID
	}
	if !gefunden {
		t.Error("abgeschlossenes Jahr ohne den ausgeschiedenen Pächter")
	}
	// Wiederherstellen holt ihn ohne Garten zurück, der Garten bleibt beim Nachfolger
	if rec := do(h, "POST", "/api/admin/paechter/"+altID+"/wiederherstellen", nil); rec.Code != 200 {
		t.Fatalf("Wiederherstellen: %d %s", rec.Code, rec.Body.String())
	}
	st = decode[stateResp](t, do(h, "GET", "/api/state", nil))
	for _, p := range st.Paechter {
		if p.ID == altID && (p.Gartennr != "" || p.AusgeschiedenAm != "") {
			t.Errorf("wiederhergestellter Pächter: %+v", p)
		}
	}

	// endgültig löschen entfernt den Namen auch im Übergabeprotokoll
	do(h, "DELETE", "/api/admin/paechter/"+altID, nil)
	if rec := do(h, "DELETE", "/api/admin/paechter/"+altID+"/endgueltig", nil); rec.Code != 200 {
		t.Fatalf("endgültig: %d %s", rec.Code, rec.Body.String())
	}
	l := decode[[]Uebergabe](t, do(h, "GET", "/api/admin/uebergaben", nil))
	if l[0].AltName != geloeschtText || l[0].AltAnschrift != "" || l[0].NeuName != "Neu Pächterin" {
		t.Errorf("Übergabe nach dem Löschen: %+v", l[0])
	}
}

func TestPaechterwechselNachAusgestellterRechnung(t *testing.T) {
	_, h := newTestApp(t)
	altID := wechselAnlegen(t, h)
	do(h, "PUT", "/api/ablesung/"+altID, map[string]any{"wasserVJ": 100, "wasserAkt": 110, "stromVJ": 1000, "stromAkt": 1100, "stunden": 12})
	do(h, "POST", "/api/invoices/issue", issueReq{IDs: []string{altID}})
	req := wechselReqFuer(altID)
	req["neu"].(map[string]any)["mitgliedsnr"] = "8"
	if rec := do(h, "POST", "/api/admin/paechterwechsel", req); rec.Code != 400 || !strings.Contains(rec.Body.String(), "Jahreswechsel") {
		t.Errorf("Wechsel trotz ausgestellter Jahresrechnung: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPaechterwechselNurAdmin(t *testing.T) {
	_, h := newTestApp(t)
	altID := wechselAnlegen(t, h)
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	req := wechselReqFuer(altID)
	req["neu"].(map[string]any)["mitgliedsnr"] = "8"
	for _, c := range []struct{ m, u string }{
		{"POST", "/api/admin/paechterwechsel"}, {"GET", "/api/admin/uebergaben"}, {"GET", "/api/admin/warteliste"},
		{"POST", "/api/admin/termine"}, {"POST", "/api/admin/termine-uebernehmen"},
	} {
		if rec := do(h, c.m, c.u, req); rec.Code != 401 {
			t.Errorf("%s %s ohne Anmeldung: %d", c.m, c.u, rec.Code)
		}
	}
}

func TestSchlussSettings(t *testing.T) {
	s := defaultSettings() // Rechnungsdatum 2026-01-10, Zahlungsziel 2026-02-15 (36 Tage)
	jetzt := time.Date(2025, 7, 1, 12, 0, 0, 0, time.UTC)
	if got := schlussSettings(s, Ablesung{}, jetzt); got.Rechnungsdatum != s.Rechnungsdatum {
		t.Error("normale Rechnung darf ihr Datum nicht ändern")
	}
	got := schlussSettings(s, Ablesung{Schluss: "2025-06-15"}, jetzt)
	if got.Rechnungsdatum != "2025-07-01" || got.Zahlungsziel != "2025-08-06" {
		t.Errorf("Schlussrechnung vor dem Rechnungsdatum: %s / %s", got.Rechnungsdatum, got.Zahlungsziel)
	}
	if got := schlussSettings(s, Ablesung{Schluss: "2025-12-20"}, time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)); got.Rechnungsdatum != s.Rechnungsdatum {
		t.Error("nach dem regulären Rechnungsdatum gilt das normale Datum")
	}
	s.Zahlungsziel = s.Rechnungsdatum
	if got := schlussSettings(s, Ablesung{Schluss: "2025-06-15"}, jetzt); got.Zahlungsziel != "2025-07-15" {
		t.Errorf("mindestens 14 Tage Zahlungsfrist: %s", got.Zahlungsziel)
	}
}

func TestUebergabeprotokollLangeBemerkung(t *testing.T) {
	u := Uebergabe{Datum: "2025-06-15", Gartennr: "7", AltName: "A", NeuName: "B", Bemerkungen: strings.Repeat("Sehr lange Bemerkung zum Zustand. ", 45)}
	pdf, err := buildUebergabeprotokoll(defaultSettings(), u)
	if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("lange Bemerkung: %v", err)
	}
}

func TestUebergabeFristBereinigung(t *testing.T) {
	d := newData()
	d.Uebergaben = []Uebergabe{{Datum: "2014-05-01", AltName: "A", NeuName: "B", AltAnschrift: "x", Bemerkungen: "y"}, {Datum: "2020-05-01", AltName: "C", NeuName: "D"}}
	_, jahre, _ := d.bereinigeAbgelaufene(2025)
	if jahre != 1 || d.Uebergaben[0].AltName != fristAbgelaufenText || d.Uebergaben[0].Bemerkungen != "" || d.Uebergaben[1].AltName != "C" {
		t.Errorf("Bereinigung der Übergaben: %d %+v", jahre, d.Uebergaben)
	}
	if _, jahre, _ := d.bereinigeAbgelaufene(2025); jahre != 0 {
		t.Error("Bereinigung muss idempotent sein")
	}
}
