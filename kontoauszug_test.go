package main

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// cp1252 wandelt Text so um, wie ihn die Sparkasse exportiert (Windows-1252 statt UTF-8).
func cp1252(s string) []byte {
	var out []byte
	for _, r := range s {
		if r < 256 {
			out = append(out, byte(r))
		} else {
			out = append(out, '?')
		}
	}
	return out
}

func TestParseKontoCSVBanken(t *testing.T) {
	sparkasse := cp1252(`"Auftragskonto";"Buchungstag";"Valutadatum";"Buchungstext";"Verwendungszweck";"Beguenstigter/Zahlungspflichtiger";"Kontonummer/IBAN";"BIC (SWIFT-Code)";"Betrag";"Waehrung";"Info"
"DE001";"03.02.26";"03.02.26";"GUTSCHRIFT";"Rechnung 2025-1-03 Garten";"Jürgen Müller";"DE02";"X";"123,45";"EUR";"Umsatz gebucht"
"DE001";"04.02.26";"04.02.26";"LASTSCHRIFT";"Kontoführung";"Sparkasse";"";"";"-5,90";"EUR";"Umsatz gebucht"
`)
	dkb := []byte("\ufeff\"Girokonto\";\"DE00 1234\"\n\"Zeitraum:\";\"01.01.2026 - 31.01.2026\"\n\n" +
		"\"Buchungsdatum\";\"Wertstellung\";\"Status\";\"Zahlungspflichtige*r\";\"Zahlungsempfänger*in\";\"Verwendungszweck\";\"Umsatztyp\";\"IBAN\";\"Betrag (€)\"\n" +
		"\"15.01.26\";\"15.01.26\";\"Gebucht\";\"Anna Beispiel\";\"KGV Sonnenschein\";\"Jahresabrechnung 2025-1-04\";\"Eingang\";\"DE11\";\"1.234,50\"\n")
	ing := []byte("Umsatzanzeige;Datei erstellt am: 01.02.2026 10:00\nIBAN;DE00 5000\n\n" +
		"Buchung;Valuta;Auftraggeber/Empfänger;Buchungstext;Verwendungszweck;Saldo;Währung;Betrag;Währung\n" +
		"20.01.2026;20.01.2026;Max Muster;Gutschrift;Gartenrechnung;1.000,00;EUR;88,00;EUR\n")
	for name, c := range map[string]struct {
		data          []byte
		n, aus        int
		datum, nm, zw string
		betrag        float64
	}{
		"Sparkasse": {sparkasse, 1, 1, "2026-02-03", "Jürgen Müller", "Rechnung 2025-1-03 Garten", 123.45},
		"DKB":       {dkb, 1, 0, "2026-01-15", "Anna Beispiel", "Jahresabrechnung 2025-1-04", 1234.5},
		"ING":       {ing, 1, 0, "2026-01-20", "Max Muster", "Gartenrechnung", 88},
	} {
		bs, aus, err := parseKontoCSV(c.data)
		if err != nil || len(bs) != c.n || aus != c.aus {
			t.Fatalf("%s: %v, %d Buchungen, %d Ausgaben", name, err, len(bs), aus)
		}
		b := bs[0]
		if b.Datum != c.datum || b.Name != c.nm || b.Zweck != c.zw || b.Betrag != c.betrag || len(b.Fingerprint) != 24 {
			t.Errorf("%s: %+v", name, b)
		}
	}
	if _, _, err := parseKontoCSV([]byte("Name;Adresse\nA;B\n")); err == nil {
		t.Error("Datei ohne Umsatztabelle sollte abgelehnt werden")
	}
}

func TestEnthaeltNummer(t *testing.T) {
	for _, c := range []struct {
		text, nr string
		want     bool
	}{
		{"RECHNUNG2025-1-03GARTEN", "2025-1-03", true},
		{"RECHNUNG2025-1-031", "2025-1-03", false},
		{"12025-1-03", "2025-1-03", false},
		{"RE2025-1-03", "2025-1-03", true},
		{"2025-1-03", "2025-1-03", true},
		{"2025-1-0", "2025-1-03", false},
		{"XX2025-1-031UND2025-1-03", "2025-1-03", true},
	} {
		if got := enthaeltNummer(c.text, c.nr); got != c.want {
			t.Errorf("%q / %q: %v", c.text, c.nr, got)
		}
	}
}

func TestKontoauszugZuordnenUndBuchen(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	do(h, "POST", "/api/invoices/issue", issueReq{})
	var a, b, c Rechnung // drei offene Rechnungen
	n := 0
	rechnung := func(id string) Rechnung {
		app.st.mu.Lock()
		defer app.st.mu.Unlock()
		for _, x := range app.st.d.Rechnungen {
			if x.ID == id {
				return *x
			}
		}
		return Rechnung{}
	}
	app.st.mu.Lock()
	alle := append([]*Rechnung(nil), app.st.d.Rechnungen...)
	app.st.mu.Unlock()
	for _, p := range alle {
		x := *p
		if x.Status != statusGueltig || x.Result.Gesamt <= 0 {
			continue
		}
		switch n {
		case 0:
			a = x
		case 1:
			b = x
		case 2:
			c = x
		}
		n++
	}
	if c.ID == "" {
		t.Fatal("zu wenige Rechnungen")
	}
	eur := func(v float64) string { return strings.ReplaceAll(fmt.Sprintf("%.2f", v), ".", ",") }
	csvText := "Buchungstag;Verwendungszweck;Beguenstigter/Zahlungspflichtiger;Betrag\n" +
		// passt: Nummer (umbrochen mit Leerzeichen) und voller Betrag
		"10.02.2026;Rechnung " + strings.Replace(a.Nummer, "-", "- ", 1) + ";Irgendwer;" + eur(a.Result.Gesamt) + "\n" +
		// Teilbetrag
		"11.02.2026;" + b.Nummer + " Teil 1;Irgendwer;10,00\n" +
		// nur Name und Betrag
		"12.02.2026;Gartenpacht;" + c.Paechter.Name + ";" + eur(c.Result.Gesamt) + "\n" +
		// nicht zuzuordnen und eine Abbuchung
		"13.02.2026;Spende;Unbekannt;20,00\n14.02.2026;Gebühr;Bank;-3,00\n"
	rec := doMultipart(h, "/api/admin/kontoauszug", "umsaetze.csv", []byte(csvText))
	if rec.Code != 200 {
		t.Fatalf("Vorschau: %d %s", rec.Code, rec.Body.String())
	}
	v := decode[kontoVorschau](t, rec)
	want := []struct{ status, id string }{{"passt", a.ID}, {"teilbetrag", b.ID}, {"vorschlag", c.ID}, {"offen", ""}}
	if len(v.Buchungen) != 4 || v.Ausgaben != 1 {
		t.Fatalf("Buchungen: %d, Ausgaben %d", len(v.Buchungen), v.Ausgaben)
	}
	for i, w := range want {
		if v.Buchungen[i].Status != w.status || v.Buchungen[i].RechnungID != w.id {
			t.Errorf("Zeile %d: %s/%s, erwartet %s/%s (%s)", i, v.Buchungen[i].Status, v.Buchungen[i].RechnungID, w.status, w.id, v.Buchungen[i].Hinweis)
		}
	}
	// buchen: die ersten beiden
	zahl := func(i int) map[string]any {
		x := v.Buchungen[i]
		return map[string]any{"rechnungId": x.RechnungID, "datum": x.Datum, "betrag": x.Betrag, "fingerprint": x.Fingerprint}
	}
	if rec := do(h, "POST", "/api/admin/kontoauszug/buchen", map[string]any{"zahlungen": []any{zahl(0), zahl(1)}}); rec.Code != 200 {
		t.Fatalf("buchen: %d %s", rec.Code, rec.Body.String())
	}
	if x := rechnung(a.ID); x.BezahltAm != "2026-02-10" || x.BezahltBetrag != nil {
		t.Errorf("voll bezahlt: %s %v", x.BezahltAm, x.BezahltBetrag)
	}
	if x := rechnung(b.ID); x.BezahltBetrag == nil || *x.BezahltBetrag != 10 {
		t.Errorf("Teilzahlung: %v", x.BezahltBetrag)
	}
	// derselbe Auszug noch einmal: nichts doppelt
	v2 := decode[kontoVorschau](t, doMultipart(h, "/api/admin/kontoauszug", "umsaetze.csv", []byte(csvText)))
	if v2.Buchungen[0].Status != "doppelt" || v2.Buchungen[1].Status != "doppelt" || v2.Buchungen[2].Status != "vorschlag" {
		t.Errorf("zweites Einlesen: %s %s %s", v2.Buchungen[0].Status, v2.Buchungen[1].Status, v2.Buchungen[2].Status)
	}
	if rec := do(h, "POST", "/api/admin/kontoauszug/buchen", map[string]any{"zahlungen": []any{zahl(0)}}); rec.Code != http.StatusBadRequest {
		t.Errorf("doppelt buchen: %d", rec.Code)
	}
	// alles oder nichts: eine ungültige Zahlung verhindert auch die gültige
	schlecht := zahl(3)
	schlecht["rechnungId"] = "gibtesnicht"
	if rec := do(h, "POST", "/api/admin/kontoauszug/buchen", map[string]any{"zahlungen": []any{zahl(2), schlecht}}); rec.Code != http.StatusBadRequest {
		t.Errorf("ungültige Rechnung: %d", rec.Code)
	}
	if rechnung(c.ID).BezahltAm != "" {
		t.Error("halb gebucht trotz Fehler")
	}
	for name, z := range map[string]map[string]any{
		"Betrag 0":      {"rechnungId": c.ID, "datum": "2026-02-12", "betrag": 0, "fingerprint": strings.Repeat("a", 24)},
		"Datum falsch":  {"rechnungId": c.ID, "datum": "12.02.2026", "betrag": 5, "fingerprint": strings.Repeat("a", 24)},
		"Fingerabdruck": {"rechnungId": c.ID, "datum": "2026-02-12", "betrag": 5, "fingerprint": "x"},
	} {
		if rec := do(h, "POST", "/api/admin/kontoauszug/buchen", map[string]any{"zahlungen": []any{z}}); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	if rec := doMultipart(h, "/api/admin/kontoauszug", "x.csv", []byte("keine Tabelle")); rec.Code != http.StatusBadRequest {
		t.Errorf("unlesbare Datei: %d", rec.Code)
	}
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	if rec := doMultipart(h, "/api/admin/kontoauszug", "umsaetze.csv", []byte(csvText)); rec.Code != 401 {
		t.Errorf("ohne Anmeldung: %d", rec.Code)
	}
}
