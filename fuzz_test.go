package main

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"math"
	"strings"
	"testing"
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
		if emailGueltig(email) && strings.ContainsAny(email, " \t\r\n\"'<>?&#%\\") {
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
