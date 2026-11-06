package main

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"math"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Die Importdateien kommen von außen. Egal wie kaputt sie sind: kein Absturz (Panic).

func FuzzReadXLSX(f *testing.F) {
	ok, _ := writeXLSX("Mitglieder", []float64{10, 10}, [][]xCell{{{"Mitgliedsnr.", stHeader}, {"Name", stHeader}}, {{"1", stNormal}, {"Max", stNormal}}})
	f.Add(ok, "Mitglieder")
	f.Add([]byte("PK\x03\x04kaputt"), "")
	f.Add([]byte{}, "x")
	f.Fuzz(func(t *testing.T, data []byte, sheet string) {
		_, _ = readXLSX(data, sheet)
	})
}

func FuzzParseCSV(f *testing.F) {
	f.Add([]byte("Mitgliedsnr.;Name\n1;Max\n"))
	f.Add([]byte("\xef\xbb\xbfNr,Name\r\n\"a\"\"b\",c\n"))
	f.Add([]byte("\xff\xfe\x00\x00"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseCSV(data)
	})
}

func FuzzParseImport(f *testing.F) {
	f.Add("Mitgliedsnr.\x00Name\x00Gartenfläche", "1\x002\x003", true)
	f.Fuzz(func(t *testing.T, header, row string, raw bool) {
		split := func(s string) []string {
			var out []string
			cur := ""
			for _, r := range s {
				if r == 0 {
					out = append(out, cur)
					cur = ""
				} else {
					cur += string(r)
				}
			}
			return append(out, cur)
		}
		table := [][]string{split(header), split(row), split(row + row)}
		rows, _, _ := parseImport(table, raw, defaultSettings(), nil)
		// Was der Import liefert, muss die normalen Prüfungen bestehen, sonst könnte eine präparierte
		// Datei unbrauchbare Zahlen (z. B. 1e301) in den Datenbestand bringen.
		for _, r := range rows {
			if !validNum(r.Paechter.Gartengroesse) || (r.Paechter.UmlageAbweichend != nil && !validNum(*r.Paechter.UmlageAbweichend)) {
				t.Fatalf("Import lieferte ungültige Stammdaten: %+v", r.Paechter)
			}
			if r.Ablesung != nil {
				if _, err := cleanAblesung(*r.Ablesung); err != nil {
					t.Fatalf("Import lieferte ungültige Ablesung: %+v (%v)", *r.Ablesung, err)
				}
			}
		}
	})
}

func FuzzParseNumber(f *testing.F) {
	for _, s := range []string{"1,5", "1.234,56", "-", "", "1e309", "NaN", "Inf", "0x10", " 12 ", "1,2,3", "٣"} {
		f.Add(s, false)
	}
	f.Fuzz(func(t *testing.T, s string, raw bool) {
		if v, ok := parseNumber(s, raw); ok && (math.IsNaN(v) || math.IsInf(v, 0)) {
			t.Fatalf("parseNumber(%q) lieferte unbrauchbaren Wert %v", s, v)
		}
	})
}

// zipBombe: sehr große, hoch komprimierbare Datei in einem gültigen Zip-Gerüst
func TestReadXLSXGrosseDateiOhneAbsturz(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range []string{"[Content_Types].xml", "xl/workbook.xml", "xl/worksheets/sheet1.xml"} {
		w, _ := zw.Create(n)
		w.Write(bytes.Repeat([]byte("<a>"), 1<<20))
	}
	zw.Close()
	_, _ = readXLSX(buf.Bytes(), "")
}

// Eine als gültig akzeptierte E-Mail-Adresse darf im mailto-Link nichts einschleusen können
// (keine Leerzeichen, Anführungszeichen, ? & # % oder Steuerzeichen), und Betreff/Text werden immer kodiert.
func FuzzEmailUndMailto(f *testing.F) {
	f.Add("anna@example.org", "Betreff & Text?\nZeile")
	f.Add("a@b.c\"&cc=x@y.de", "%0A")
	f.Fuzz(func(t *testing.T, email, text string) {
		// Apostroph ist erlaubt (z. B. o'neil@…): Der Link wird ohne Shell geöffnet und die Adresse zusätzlich kodiert
		if emailGueltig(email) && strings.ContainsAny(email, " \t\r\n\"<>?&#%\\") {
			t.Fatalf("gefährliche Adresse akzeptiert: %q", email)
		}
		if enc := mailtoEscape(text); strings.ContainsAny(enc, " \r\n&?#\"") {
			t.Fatalf("unkodierte Zeichen im mailto-Text: %q", enc)
		}
	})
}

// Hochgeladene Logos kommen von außen: kein Absturz, und was angenommen wird, ist ein echtes PNG.
func FuzzPruefeLogo(f *testing.F) {
	var b bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	_ = png.Encode(&b, img)
	f.Add(b.Bytes())
	f.Add([]byte("\xff\xd8\xff\xe0kaputt"))
	f.Fuzz(func(t *testing.T, data []byte) {
		out, err := pruefeLogo(data)
		if err == nil && !bytes.HasPrefix(out, []byte("\x89PNG")) {
			t.Fatal("angenommenes Logo ist kein PNG")
		}
	})
}

// Termine landen in Kalenderdatei und Aushang: seltsame Zeichen dürfen weder Einträge einschleusen noch die PDF zerstören.
func FuzzTerminKalenderUndAushang(f *testing.F) {
	f.Add("Arbeitseinsatz", "Tor", "Handschuhe", "09:00", "2026-04-04")
	f.Add("A\r\nBEGIN:VEVENT\r\nSUMMARY:x", "O;rt,\\", "\x00\x1f\u202e", "24:61", "2026-02-30")
	f.Add(strings.Repeat("Ä", 300), strings.Repeat("ß", 300), strings.Repeat("\n", 400), "", "")
	f.Fuzz(func(t *testing.T, titel, ort, notiz, von, datum string) {
		tm, err := cleanTermin(Termin{Titel: titel, Ort: ort, Notiz: notiz, Von: von, Datum: datum, Art: "Fest"})
		if err != nil {
			return
		}
		if strings.ContainsAny(tm.Titel+tm.Ort, "\r\n") {
			t.Fatalf("Zeilenumbruch in Titel/Ort: %q %q", tm.Titel, tm.Ort)
		}
		st, err := openStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		app := &App{st: st, sessions: map[string]time.Time{}}
		st.d.Termine = []Termin{tm}
		rec := httptest.NewRecorder()
		app.handleTermineICS(rec, httptest.NewRequest("GET", "/api/termine.ics", nil))
		if n := strings.Count(rec.Body.String(), "BEGIN:VEVENT"); n > 1 {
			t.Fatalf("eingeschleuster Kalendereintrag (%d):\n%s", n, rec.Body.String())
		}
		if _, err := buildJahresplan(defaultSettings(), jahrAusZeit(tm.Datum), []Termin{tm}); err != nil && !strings.Contains(err.Error(), "keine Termine") {
			t.Fatalf("Aushang: %v", err)
		}
	})
}

// Übergabeprotokoll mit beliebigen Texten: nie ein Absturz, immer eine gültige PDF.
func FuzzUebergabeprotokoll(f *testing.F) {
	f.Add("Alt", "Neu", "Schlüssel", "Bemerkung")
	f.Add(strings.Repeat("W", 500), "\u202e\x00", strings.Repeat("S", 400), strings.Repeat("Zeile\n", 400))
	f.Fuzz(func(t *testing.T, alt, neu, schluessel, bem string) {
		u := Uebergabe{Datum: "2025-06-15", Gartennr: "7", AltName: alt, NeuName: neu, AltAnschrift: alt, NeuAnschrift: neu,
			Schluessel: trim(schluessel, 200), Bemerkungen: trim(bem, 1500)}
		pdf, err := buildUebergabeprotokoll(defaultSettings(), u)
		if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF")) {
			t.Fatalf("Protokoll: %v", err)
		}
	})
}

// Einladung, Mängel-Schreiben und Dateinamen dürfen mit beliebigen Eingaben weder abstürzen noch einen
// Dateinamen mit Pfad- oder Steuerzeichen liefern.
func FuzzVersammlungUndMaengel(f *testing.F) {
	f.Add("Jahreshauptversammlung", "Begrüßung\nKassenbericht", "Bitte pünktlich", "Hecke zu hoch", "12")
	f.Add("\u202eexe.fdp", strings.Repeat("x\n", 50), "", "\x00\r\n", "../5")
	f.Fuzz(func(t *testing.T, titel, tops, notiz, beschreibung, garten string) {
		s := defaultSettings()
		if tr, err := cleanTermin(Termin{Datum: "2027-03-20", Von: "10:00", Titel: titel, Art: "Versammlung", Tops: tops, Notiz: notiz}); err == nil {
			if _, err := buildEinladung(s, tr); err != nil {
				t.Fatalf("Einladung: %v", err)
			}
			if _, err := buildAnwesenheit(s, tr, []anwesenheitZeile{{garten, "1", titel}}, len(tops)%2 == 0); err != nil {
				t.Fatalf("Anwesenheitsliste: %v", err)
			}
		}
		if m, err := cleanMangel(Mangel{Gartennr: garten, Datum: "2026-05-01", Beschreibung: beschreibung}); err == nil {
			p := Paechter{Gartennr: m.Gartennr, Name: titel, Strasse: notiz}
			if _, err := buildMaengelschreiben(s, p, []Mangel{m}, "2026-06-01"); err != nil {
				t.Fatalf("Mängel-Schreiben: %v", err)
			}
		}
		for _, r := range safeName(titel+garten, 80) {
			if r < 0x20 || r == 0x7f || strings.ContainsRune(`<>:"/\|?*`, r) {
				t.Fatalf("Dateiname mit %U", r)
			}
		}
	})
}

// Beliebige Bank-Dateien dürfen das Einlesen nicht zum Absturz bringen; erkannte Buchungen sind immer Eingänge
// mit gültigem Datum.
func FuzzKontoauszug(f *testing.F) {
	f.Add([]byte("Buchungstag;Verwendungszweck;Name;Betrag\n01.02.2026;RE 2025-1;A;12,50\n"))
	f.Add([]byte("\"Buchung\",\"Betrag (€)\"\n\"2026-01-01\",\"1.000,00\"\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		bs, _, err := parseKontoCSV(data)
		if err != nil {
			return
		}
		for _, b := range bs {
			if _, err := parseDate(b.Datum); err != nil || b.Betrag <= 0 || len(b.Fingerprint) != 24 {
				t.Fatalf("ungültige Buchung: %+v", b)
			}
			_ = enthaeltNummer(kompakt(b.Zweck), b.Name)
		}
	})
}
