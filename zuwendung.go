package main

import (
	"fmt"
	"strings"
	"time"
)

// buildZuwendungsbestaetigung setzt die Bestätigung über Geldzuwendungen nach dem amtlichen Muster: bei einer
// Spende als Einzelbestätigung, bei mehreren als Sammelbestätigung mit Anlage. Der Wortlaut folgt dem Muster;
// Feld- und Satzreihenfolge dürfen nicht verändert werden.
func buildZuwendungsbestaetigung(s Settings, g Gemeinnuetzigkeit, l []Spende, heute time.Time) ([]byte, error) {
	sammel := len(l) > 1
	titel := "Bestätigung über Geldzuwendungen"
	if sammel {
		titel = "Sammelbestätigung über Geldzuwendungen"
	}
	pdf := neuesPDF(s, titel, false)
	pdf.AddPage()
	const breite = rightX - leftX
	y := 14.0
	klein := func(txt string) {
		pdf.SetFont("lib", "", 7.5)
		pdf.SetTextColor(80, 80, 80)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(breite, 4, txt, "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		y += 4
	}
	kasten := func(zeilen []string, h float64) {
		pdf.SetDrawColor(120, 120, 120)
		pdf.SetLineWidth(0.25)
		pdf.Rect(leftX, y, breite, h, "D")
		pdf.SetFont("lib", "", 10)
		yy := y + 2
		for i, z := range zeilen {
			stil := ""
			if i == 0 {
				stil = "B"
			}
			pdf.SetFont("lib", stil, 10)
			pdf.SetXY(leftX+2, yy)
			pdf.CellFormat(breite-4, 5, z, "", 0, "L", false, 0, "")
			yy += 5
		}
		y += h + 4
	}
	absatz := func(txt string, groesse float64) {
		pdf.SetFont("lib", "", groesse)
		pdf.SetXY(leftX, y)
		pdf.MultiCell(breite, groesse*0.48, txt, "", "L", false)
		y = pdf.GetY() + 3
	}

	klein("Aussteller (Bezeichnung und Anschrift der steuerbegünstigten Einrichtung)")
	kasten(append([]string{s.VereinName}, strings.Split(g.Anschrift, "\n")...), 4+5*float64(1+len(strings.Split(g.Anschrift, "\n"))))

	pdf.SetFont("lib", "B", 13)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(breite, 6, titel, "", 0, "L", false, 0, "")
	y += 7
	absatz("im Sinne des § 10b des Einkommensteuergesetzes an eine der in § 5 Abs. 1 Nr. 9 des Körperschaftsteuergesetzes "+
		"bezeichneten Körperschaften, Personenvereinigungen oder Vermögensmassen", 9)

	x0 := l[0]
	klein("Name und Anschrift des Zuwendenden:")
	kasten([]string{x0.Name, x0.Strasse, x0.PLZOrt}, 19)

	summe := 0.0
	for _, x := range l {
		summe += x.Betrag
	}
	summe = round2(summe)
	tag := germanDate(x0.Datum)
	tagTitel := "Tag der Zuwendung:"
	if sammel {
		tagTitel = "Zeitraum der Sammelbestätigung:"
		tag = germanDate(l[0].Datum) + " – " + germanDate(l[len(l)-1].Datum)
	}
	spalten := []struct {
		titel, wert string
		w           float64
	}{{"Betrag der Zuwendung – in Ziffern –", fmtEUR(summe), 52}, {"– in Buchstaben –", betragInWorten(summe), breite - 52 - 46}, {tagTitel, tag, 46}}
	x := leftX
	pdf.SetDrawColor(120, 120, 120)
	for _, sp := range spalten {
		pdf.Rect(x, y, sp.w, 15, "D")
		pdf.SetFont("lib", "", 7.5)
		pdf.SetTextColor(80, 80, 80)
		pdf.SetXY(x+1.5, y+1)
		pdf.CellFormat(sp.w-3, 4, sp.titel, "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("lib", "B", 10)
		if pdf.GetStringWidth(sp.wert) <= sp.w-3 {
			pdf.SetXY(x+1.5, y+6)
			pdf.CellFormat(sp.w-3, 6, sp.wert, "", 0, "L", false, 0, "")
		} else { // lange Beträge in Worten umbrechen statt abschneiden
			pdf.SetFont("lib", "B", 8)
			pdf.SetXY(x+1.5, y+5)
			pdf.MultiCell(sp.w-3, 3.4, sp.wert, "", "L", false)
		}
		x += sp.w
	}
	y += 19

	kreuz := func(an bool) string {
		if an {
			return "[X]"
		}
		return "[  ]"
	}
	if sammel {
		absatz("Ob es sich um den Verzicht auf Erstattung von Aufwendungen handelt, ist der Anlage zur Sammelbestätigung zu entnehmen.", 9.5)
	} else {
		absatz(fmt.Sprintf("Es handelt sich um den Verzicht auf Erstattung von Aufwendungen    Ja %s    Nein %s", kreuz(x0.Verzicht), kreuz(!x0.Verzicht)), 9.5)
	}
	y += 1
	if g.Bescheid == "feststellung" {
		absatz(fmt.Sprintf("%s  Die Einhaltung der satzungsmäßigen Voraussetzungen nach den §§ 51, 59, 60 und 61 AO wurde vom Finanzamt %s, "+
			"StNr. %s, mit Bescheid vom %s nach § 60a AO gesondert festgestellt. Wir fördern nach unserer Satzung %s.",
			kreuz(true), g.Finanzamt, g.Steuernummer, germanDate(g.BescheidDatum), g.Zweck), 9.5)
	} else {
		absatz(fmt.Sprintf("%s  Wir sind wegen Förderung %s nach dem Freistellungsbescheid bzw. nach der Anlage zum Körperschaftsteuerbescheid "+
			"des Finanzamtes %s, StNr. %s, vom %s für den letzten Veranlagungszeitraum %s nach § 5 Abs. 1 Nr. 9 des "+
			"Körperschaftsteuergesetzes von der Körperschaftsteuer und nach § 3 Nr. 6 des Gewerbesteuergesetzes von der Gewerbesteuer befreit.",
			kreuz(true), g.Zweck, g.Finanzamt, g.Steuernummer, germanDate(g.BescheidDatum), g.Veranlagung), 9.5)
	}
	if sammel {
		absatz(fmt.Sprintf("Es wird bestätigt, dass die Zuwendungen nur zur Förderung %s verwendet werden.", g.Zweck), 9.5)
		absatz("Es wird bestätigt, dass über die in der Gesamtsumme enthaltenen Zuwendungen keine weiteren Bestätigungen, weder formelle "+
			"Zuwendungsbestätigungen noch Beitragsquittungen oder Ähnliches ausgestellt wurden und werden.", 9.5)
	} else {
		absatz(fmt.Sprintf("Es wird bestätigt, dass die Zuwendung nur zur Förderung %s verwendet wird.", g.Zweck), 9.5)
	}

	// Unterschrift
	y += 10
	pdf.SetFont("lib", "", 10)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(80, 5, s.Ort+", "+heute.Format("02.01.2006"), "", 0, "L", false, 0, "")
	y += 6
	pdf.SetDrawColor(0, 0, 0)
	pdf.Line(leftX, y, leftX+120, y)
	y += 1
	klein("(Ort, Datum und Unterschrift des Zuwendungsempfängers)")
	y += 4

	pdf.SetFont("lib", "B", 8)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(breite, 4, "Hinweis:", "", 0, "L", false, 0, "")
	y += 4.5
	absatz("Wer vorsätzlich oder grob fahrlässig eine unrichtige Zuwendungsbestätigung erstellt oder veranlasst, dass Zuwendungen nicht zu den "+
		"in der Zuwendungsbestätigung angegebenen steuerbegünstigten Zwecken verwendet werden, haftet für die entgangene Steuer "+
		"(§ 10b Abs. 4 EStG, § 9 Abs. 3 KStG, § 9 Nr. 5 GewStG).", 7.5)
	absatz("Diese Bestätigung wird nicht als Nachweis für die steuerliche Berücksichtigung der Zuwendung anerkannt, wenn das Datum des "+
		"Freistellungsbescheides länger als 5 Jahre bzw. das Datum der Feststellung der Einhaltung der satzungsmäßigen Voraussetzungen "+
		"nach § 60a Abs. 1 AO länger als 3 Jahre seit Ausstellung des Bescheides zurückliegt (§ 63 Abs. 5 AO).", 7.5)

	if sammel {
		pdf.AddPage()
		y = 16
		pdf.SetFont("lib", "B", 12)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(breite, 6, "Anlage zur Sammelbestätigung", "", 0, "L", false, 0, "")
		y += 7
		pdf.SetFont("lib", "", 9.5)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(breite, 5, fmt.Sprintf("%s, %s, %s", x0.Name, x0.Strasse, x0.PLZOrt), "", 0, "L", false, 0, "")
		y += 9
		cols := []struct {
			t string
			w float64
		}{{"Datum der Zuwendung", 38}, {"Art der Zuwendung (Geldzuwendung/Mitgliedsbeitrag)", 62}, {"Verzicht auf die Erstattung von Aufwendungen (ja/nein)", 42}, {"Betrag", 28}}
		zeile := func(werte []string, stil string, h float64) {
			x := leftX
			for i, c := range cols {
				pdf.SetFont("lib", stil, 8.5)
				pdf.SetXY(x, y)
				al := "L"
				if i == 3 {
					al = "R"
				}
				if h > 7 {
					pdf.Rect(x, y, c.w, h, "D")
					pdf.SetXY(x+1, y+1)
					pdf.MultiCell(c.w-2, 3.8, werte[i], "", al, false)
				} else {
					pdf.CellFormat(c.w, h, werte[i], "1", 0, al, false, 0, "")
				}
				x += c.w
			}
			y += h
		}
		zeile([]string{cols[0].t, cols[1].t, cols[2].t, cols[3].t}, "B", 13)
		for _, s := range l {
			if y > 270 {
				pdf.AddPage()
				y = 16
			}
			v := "nein"
			if s.Verzicht {
				v = "ja"
			}
			zeile([]string{germanDate(s.Datum), "Geldzuwendung", v, fmtEUR(s.Betrag)}, "", 6.5)
		}
		zeile([]string{"Gesamtsumme", "", "", fmtEUR(summe)}, "B", 6.5)
	}
	return pdfBytes(pdf)
}
