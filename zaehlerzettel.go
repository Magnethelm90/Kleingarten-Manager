package main

import (
	"fmt"
	"net/http"
)

// Zählerstand-Zettel zum Einwerfen: je Garten ein Zettel (drei auf einer A4-Seite, zum Ausschneiden) mit Zählernummern
// und Vorjahresständen. Der Pächter trägt den neuen Stand ein und wirft den Zettel in den Vereinsbriefkasten.

func (a *App) zaehlerzettelPDF(r *http.Request) ([]byte, string, error) {
	year := a.yearParam(r)
	frist := r.URL.Query().Get("frist")
	if frist != "" {
		if _, err := parseDate(frist); err != nil {
			return nil, "", bad("Bitte ein gültiges Datum für die Abgabe wählen")
		}
	}
	nurOffen := r.URL.Query().Get("nur") == "offen"
	a.st.mu.Lock()
	rows, s, ok := a.st.ablesezeilenLocked(year)
	v, _ := a.st.viewLocked(year)
	if ok && nurOffen {
		ids := map[string]string{} // Mitgliedsnr → Pächter-ID
		for _, p := range v.Paechter {
			ids[p.Mitgliedsnr] = p.ID
		}
		offen := rows[:0:0]
		for _, z := range rows {
			ab := v.Ablesungen[ids[z.Mitgliedsnr]]
			if ab.WasserAkt == nil || ab.StromAkt == nil {
				offen = append(offen, z)
			}
		}
		rows = offen
	}
	a.st.mu.Unlock()
	if !ok {
		return nil, "", notFound("Dieses Jahr gibt es nicht")
	}
	if len(rows) == 0 {
		if nurOffen {
			return nil, "", bad("Für alle Gärten sind die Zählerstände schon eingetragen")
		}
		return nil, "", bad("Es sind keine Pächter angelegt")
	}
	pdf, err := buildZaehlerzettel(s, year, frist, rows)
	return pdf, fmt.Sprintf("Zaehlerzettel_%d.pdf", year), err
}

func buildZaehlerzettel(s Settings, year int, frist string, rows []ablesezeile) ([]byte, error) {
	pdf := neuesPDF(s, fmt.Sprintf("Zählerstände %d – Zettel zum Einwerfen", year), false)
	mr, mg, mb := markenfarbe(s.Farbe)
	const hoehe = 99.0 // drei Zettel je Seite
	links, breite := 15.0, 180.0
	zahl := func(p *float64) string {
		if p == nil {
			return ""
		}
		return fmtFlex(*p)
	}
	leer := func(x string) string {
		if x == "" {
			return "–"
		}
		return x
	}
	for i, z := range rows {
		if i%3 == 0 {
			pdf.AddPage()
		}
		oben := float64(i%3) * hoehe
		// Schnittlinie unter dem Zettel
		if i%3 < 2 {
			pdf.SetDrawColor(170, 170, 170)
			pdf.SetDashPattern([]float64{2, 2}, 0)
			pdf.Line(5, oben+hoehe, 205, oben+hoehe)
			pdf.SetDashPattern([]float64{}, 0)
			pdf.SetFont("lib", "", 7)
			pdf.SetTextColor(150, 150, 150)
			pdf.SetXY(5, oben+hoehe-3.5)
			pdf.CellFormat(200, 3, "hier abschneiden", "", 0, "R", false, 0, "")
		}
		pdf.SetDrawColor(0, 0, 0)
		pdf.SetTextColor(0, 0, 0)
		y := oben + 12
		pdf.SetFont("lib", "B", 14)
		pdf.SetTextColor(mr, mg, mb)
		pdf.SetXY(links, y)
		pdf.CellFormat(breite*0.62, 7, fmt.Sprintf("Zählerstände %d – Garten %s", year, leer(z.Gartennr)), "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("lib", "", 9)
		pdf.SetXY(links+breite*0.62, y)
		pdf.CellFormat(breite*0.38, 7, s.VereinName, "", 0, "R", false, 0, "")
		y += 8
		pdf.SetFont("lib", "", 10.5)
		pdf.SetXY(links, y)
		pdf.CellFormat(breite, 6, z.Name+"  ·  Mitgliedsnr. "+leer(z.Mitgliedsnr), "", 0, "L", false, 0, "")
		y += 10

		// Tabelle: Zähler | Zähler-Nr. | Stand im Vorjahr | neuer Stand
		spalten := []struct {
			titel string
			w     float64
		}{{"Zähler", 34}, {"Zähler-Nr.", 46}, {"Stand Vorjahr", 40}, {"Neuer Stand (bitte eintragen)", 60}}
		pdf.SetFillColor(240, 244, 238)
		pdf.SetFont("lib", "B", 9)
		x := links
		for _, sp := range spalten {
			pdf.SetXY(x, y)
			pdf.CellFormat(sp.w, 7, sp.titel, "1", 0, "L", true, 0, "")
			x += sp.w
		}
		y += 7
		for _, zeile := range [][4]string{{"Wasser (m³)", leer(z.WasserNr), zahl(z.WasserVJ), ""}, {"Strom (kWh)", leer(z.StromNr), zahl(z.StromVJ), ""}} {
			x = links
			for k, sp := range spalten {
				pdf.SetFont("lib", map[bool]string{true: "B", false: ""}[k == 0], 10)
				pdf.SetXY(x, y)
				align := "L"
				if k == 2 {
					align = "R"
				}
				pdf.CellFormat(sp.w, 11, zeile[k], "1", 0, align, false, 0, "")
				x += sp.w
			}
			y += 11
		}
		y += 6
		pdf.SetFont("lib", "", 9.5)
		pdf.SetXY(links, y)
		pdf.CellFormat(breite/2, 6, "Abgelesen am: ______________________", "", 0, "L", false, 0, "")
		pdf.SetXY(links+breite/2, y)
		pdf.CellFormat(breite/2, 6, "Unterschrift: ______________________", "", 0, "L", false, 0, "")
		y += 9
		hinweis := "Bitte den Zettel ausgefüllt in den Briefkasten des Vereins werfen."
		if frist != "" {
			hinweis = "Bitte den Zettel ausgefüllt bis zum " + germanDate(frist) + " in den Briefkasten des Vereins werfen."
		}
		pdf.SetFont("lib", "B", 9.5)
		pdf.SetXY(links, y)
		pdf.CellFormat(breite, 5, hinweis, "", 0, "L", false, 0, "")
	}
	return pdfBytes(pdf)
}
