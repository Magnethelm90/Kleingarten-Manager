package main

import (
	"net/http"
	"strings"
)

// Schnell-Aushang: kurze Mitteilung für den Schaukasten (Sperrung, Arbeitseinsatz, Fundsache …) als A4 mit
// Vereinskopf, großer Überschrift und gut lesbarem Text. Wird nicht gespeichert – für Dauerhaftes gibt es das
// Rundschreiben. Text kommt per POST, damit er nicht in Adresszeilen oder Protokollen landet.

type aushangEingabe struct {
	Titel  string `json:"titel"`
	Text   string `json:"text"`
	Datum  string `json:"datum"`  // optional, z. B. »gilt ab«
	Zusatz string `json:"zusatz"` // optional, Zeile unter dem Text, z. B. Ansprechpartner
}

func (a *App) aushangPDF(r *http.Request) ([]byte, string, error) {
	var in aushangEingabe
	if err := readJSON(nil, r, &in); err != nil {
		return nil, "", err
	}
	titel := trim(in.Titel, 80)
	text := strings.TrimSpace(nurDruckbar(strings.ReplaceAll(in.Text, "\r", "")))
	if r := []rune(text); len(r) > 1500 {
		text = string(r[:1500])
	}
	zusatz := trim(in.Zusatz, 120)
	if titel == "" || text == "" {
		return nil, "", bad("Bitte Überschrift und Text eintragen")
	}
	if !gueltigesDatum(in.Datum, true) {
		return nil, "", bad("Ungültiges Datum")
	}
	a.st.mu.Lock()
	s := a.st.d.Settings
	a.st.mu.Unlock()

	pdf := neuesPDF(s, titel, false)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	mr, mg, mb := markenfarbe(s.Farbe)
	breite := rightX - leftX
	y := 50.0
	pdf.SetFillColor(mr, mg, mb)
	pdf.Rect(leftX, y, breite, 2.2, "F")
	y += 10
	// Überschrift: so groß wie möglich, höchstens drei Zeilen
	groesse := 34.0
	for ; groesse > 18; groesse -= 2 {
		pdf.SetFont("lib", "B", groesse)
		if len(pdf.SplitText(titel, breite)) <= 2 {
			break
		}
	}
	pdf.SetFont("lib", "B", groesse)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(leftX, y)
	pdf.MultiCell(breite, groesse*0.45, titel, "", "L", false)
	pdf.SetTextColor(0, 0, 0)
	y = pdf.GetY() + 4
	if in.Datum != "" {
		pdf.SetFont("lib", "B", 15)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(breite, 8, germanDate(in.Datum), "", 0, "L", false, 0, "")
		y += 12
	}
	y += 4
	// Text: Schriftgröße so wählen, dass er auf die Seite passt
	platz := 262 - y
	if zusatz != "" {
		platz -= 14
	}
	ts := 17.0
	zeilenZahl := func() int {
		n := 0
		for _, absatz := range strings.Split(text, "\n") {
			n += max(1, len(pdf.SplitText(absatz, breite)))
		}
		return n
	}
	for ; ts > 10; ts -= 0.5 {
		pdf.SetFont("lib", "", ts)
		if float64(zeilenZahl())*ts*0.5 <= platz {
			break
		}
	}
	pdf.SetFont("lib", "", ts)
	pdf.SetXY(leftX, y)
	pdf.MultiCell(breite, ts*0.5, text, "", "L", false)
	y = pdf.GetY() + 8
	if zusatz != "" && y < 270 {
		pdf.SetFont("lib", "", 12)
		pdf.SetTextColor(70, 70, 70)
		pdf.SetXY(leftX, y)
		pdf.MultiCell(breite, 6, zusatz, "", "L", false)
	}
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("lib", "", 11)
	pdf.SetXY(leftX, 272)
	pdf.CellFormat(breite, 6, "Der Vorstand · "+s.VereinName, "", 0, "R", false, 0, "")
	out, err := pdfBytes(pdf)
	return out, safeName("Aushang_"+titel, 80) + ".pdf", err
}
