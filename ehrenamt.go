package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// Ehrenamts-Nachweis: Bescheinigung über die im Jahr geleisteten Gemeinschaftsstunden (aus den Arbeitseinsätzen und
// nachgetragenen Stunden) und die ausgeübten Ämter. Gedacht als Dank und als Nachweis, etwa für Arbeitgeber, Schule
// oder Ehrenamtskarte. Sie ist keine Zuwendungsbestätigung (Spendenbescheinigung) und sagt nichts über Steuern aus.

type ehrenamtEinsatz struct {
	Datum, Titel string
	Stunden      float64
}

type ehrenamtNachweis struct {
	P       Paechter
	Jahr    int
	Einsatz []ehrenamtEinsatz
	Weitere float64 // Stunden, die nicht zu einem Arbeitseinsatz gehören
	Summe   float64
	Aemter  []Amt
}

// ehrenamtLocked sammelt den Nachweis eines Pächters für ein Jahr. Ämter werden über den Namen zugeordnet
// (die Ämterliste führt Namen als Text) und zählen, wenn die Amtszeit das Jahr berührt.
func (s *Store) ehrenamtLocked(v yearView, p Paechter) ehrenamtNachweis {
	n := ehrenamtNachweis{P: p, Jahr: v.Jahr}
	termine := map[string]Termin{}
	for _, t := range s.d.Termine {
		termine[t.ID] = t
	}
	for _, e := range s.d.Einsatzstunden {
		if e.Jahr != v.Jahr || e.PaechterID != p.ID || e.Stunden <= 0 {
			continue
		}
		t, ok := termine[e.TerminID]
		z := ehrenamtEinsatz{Datum: t.Datum, Titel: t.Titel, Stunden: e.Stunden}
		if !ok {
			z.Titel = "Arbeitseinsatz"
		}
		n.Einsatz = append(n.Einsatz, z)
		n.Summe += e.Stunden
	}
	sort.SliceStable(n.Einsatz, func(i, j int) bool { return n.Einsatz[i].Datum < n.Einsatz[j].Datum })
	if ab := v.Ablesungen[p.ID]; ab.Stunden != nil {
		if rest := round2(*ab.Stunden - n.Summe); rest > 0 {
			n.Weitere = rest
		}
		n.Summe = round2(*ab.Stunden)
		if n.Summe < 0 {
			n.Summe = 0
		}
	}
	von, bis := fmt.Sprintf("%d-01-01", v.Jahr), fmt.Sprintf("%d-12-31", v.Jahr)
	name := strings.ToLower(strings.TrimSpace(p.Name))
	for _, a := range s.d.Aemter {
		if name == "" || strings.ToLower(strings.TrimSpace(a.Name)) != name {
			continue
		}
		if (a.Seit != "" && a.Seit > bis) || (a.Bis != "" && a.Bis < von) {
			continue
		}
		n.Aemter = append(n.Aemter, a)
	}
	return n
}

func (n ehrenamtNachweis) leer() bool { return n.Summe <= 0 && len(n.Aemter) == 0 }

func (a *App) ehrenamtPDF(r *http.Request) ([]byte, string, error) {
	jahr := a.yearParam(r)
	pid := strings.TrimSpace(r.URL.Query().Get("paechterId"))
	a.st.mu.Lock()
	v, ok := a.st.viewLocked(jahr)
	var liste []ehrenamtNachweis
	if ok {
		for _, p := range v.Paechter {
			if pid != "" && p.ID != pid {
				continue
			}
			if pid == "" && (p.Geloescht || p.AusgeschiedenAm != "") {
				continue
			}
			if n := a.st.ehrenamtLocked(v, p); pid != "" || !n.leer() {
				liste = append(liste, n)
			}
		}
	}
	s := v.Settings
	a.st.mu.Unlock()
	if !ok {
		return nil, "", notFound("Dieses Jahr gibt es nicht")
	}
	if len(liste) == 0 {
		if pid != "" {
			return nil, "", notFound("Pächter nicht gefunden")
		}
		return nil, "", bad(fmt.Sprintf("Für %d sind keine Stunden oder Ämter eingetragen", jahr))
	}
	if pid != "" && liste[0].leer() {
		return nil, "", bad(fmt.Sprintf("Für %s sind %d weder Stunden noch Ämter eingetragen", liste[0].P.Name, jahr))
	}
	sort.SliceStable(liste, func(i, j int) bool { return natLess(liste[i].P.Mitgliedsnr, liste[j].P.Mitgliedsnr) })
	pdf, err := buildEhrenamt(s, liste)
	if err != nil {
		return nil, "", err
	}
	name := fmt.Sprintf("Ehrenamt_%d", jahr)
	if pid != "" {
		name += "_" + liste[0].P.Name
	}
	return pdf, safeName(name, 80) + ".pdf", nil
}

func stundenText(x float64) string {
	return strings.ReplaceAll(strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", x), "0"), "."), ".", ",")
}

func buildEhrenamt(s Settings, liste []ehrenamtNachweis) ([]byte, error) {
	pdf := neuesPDF(s, "Bescheinigung ehrenamtliche Tätigkeit", false)
	heute := time.Now().Format("2006-01-02")
	for _, n := range liste {
		ehrenamtSeite(pdf, s, n, heute)
	}
	return pdfBytes(pdf)
}

func ehrenamtSeite(pdf *fpdf.Fpdf, s Settings, n ehrenamtNachweis, heute string) {
	p := n.P
	p.Versand = "" // der Versandvermerk (Post/E-Mail) gehört auf Briefe, nicht auf eine Bescheinigung
	y := briefAnfang(pdf, s, p, heute)
	mr, mg, mb := markenfarbe(s.Farbe)
	pdf.SetFont("lib", "B", 13)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 7, fmt.Sprintf("Bescheinigung über ehrenamtliche Tätigkeit %d", n.Jahr), "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	y += 11
	pdf.SetFont("lib", "", 10.5)
	pdf.SetXY(leftX, y)
	nr := ""
	if n.P.Gartennr != "" {
		nr = ", Garten " + n.P.Gartennr
	}
	pdf.MultiCell(rightX-leftX, 5.4, fmt.Sprintf("Der %s bescheinigt, dass %s (Mitgliedsnr. %s%s) im Jahr %d für den Verein ehrenamtlich tätig war. "+
		"Die Tätigkeit erfolgte unentgeltlich. Wir danken herzlich für den Einsatz.", s.VereinName, n.P.Name, n.P.Mitgliedsnr, nr, n.Jahr), "", "L", false)
	y = pdf.GetY() + 6

	abschnitt := func(titel string) {
		pdf.SetFont("lib", "B", 10.5)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(rightX-leftX, 6, titel, "", 0, "L", false, 0, "")
		y += 7
		pdf.SetDrawColor(185, 190, 182)
		pdf.SetLineWidth(0.2)
		pdf.Line(leftX, y-0.8, rightX, y-0.8)
	}
	neueSeite := func() {
		if y > 262 {
			pdf.AddPage()
			y = 20
		}
	}
	if n.Summe > 0 {
		abschnitt("Gemeinschaftsarbeit")
		pdf.SetFont("lib", "", 10)
		for _, e := range n.Einsatz {
			neueSeite()
			pdf.SetXY(leftX, y)
			pdf.CellFormat(28, 5.5, germanDate(e.Datum), "", 0, "L", false, 0, "")
			pdf.CellFormat(rightX-leftX-28-30, 5.5, e.Titel, "", 0, "L", false, 0, "")
			pdf.CellFormat(30, 5.5, stundenText(e.Stunden)+" Std.", "", 0, "R", false, 0, "")
			y += 5.5
		}
		if n.Weitere > 0 {
			neueSeite()
			pdf.SetXY(leftX, y)
			pdf.CellFormat(rightX-leftX-30, 5.5, "Weitere Stunden (Pflege der Gemeinschaftsflächen, Hilfsdienste)", "", 0, "L", false, 0, "")
			pdf.CellFormat(30, 5.5, stundenText(n.Weitere)+" Std.", "", 0, "R", false, 0, "")
			y += 5.5
		}
		neueSeite()
		pdf.Line(leftX, y+0.5, rightX, y+0.5)
		pdf.SetFont("lib", "B", 10.5)
		pdf.SetXY(leftX, y+1.5)
		pdf.CellFormat(rightX-leftX-30, 6, "Summe", "", 0, "L", false, 0, "")
		pdf.CellFormat(30, 6, stundenText(n.Summe)+" Std.", "", 0, "R", false, 0, "")
		y += 12
	}
	if len(n.Aemter) > 0 {
		neueSeite()
		abschnitt("Ämter im Verein")
		pdf.SetFont("lib", "", 10)
		for _, a := range n.Aemter {
			neueSeite()
			zeit := "seit " + germanDate(a.Seit)
			switch {
			case a.Seit == "" && a.Bis != "":
				zeit = "bis " + germanDate(a.Bis)
			case a.Seit == "":
				zeit = ""
			case a.Bis != "":
				zeit = germanDate(a.Seit) + " bis " + germanDate(a.Bis)
			}
			pdf.SetXY(leftX, y)
			pdf.CellFormat(70, 5.5, a.Amt, "", 0, "L", false, 0, "")
			pdf.CellFormat(rightX-leftX-70, 5.5, zeit, "", 0, "L", false, 0, "")
			y += 5.5
		}
		y += 6
	}
	if y > 240 {
		pdf.AddPage()
		y = 20
	}
	y += 4
	pdf.SetFont("lib", "", 8.5)
	pdf.SetTextColor(90, 90, 90)
	pdf.SetXY(leftX, y)
	pdf.MultiCell(rightX-leftX, 4.2, "Diese Bescheinigung dokumentiert den Einsatz nach den Aufzeichnungen des Vereins. Sie ist keine Zuwendungsbestätigung (Spendenbescheinigung).", "", "L", false)
	y = pdf.GetY() + 18
	pdf.SetTextColor(0, 0, 0)
	pdf.SetDrawColor(120, 120, 120)
	pdf.Line(leftX, y, leftX+60, y)
	pdf.Line(leftX+80, y, leftX+140, y)
	pdf.SetFont("lib", "", 8.5)
	pdf.SetXY(leftX, y+1)
	pdf.CellFormat(60, 4.5, "Ort, Datum", "", 0, "L", false, 0, "")
	pdf.SetXY(leftX+80, y+1)
	pdf.CellFormat(60, 4.5, "Für den Vorstand", "", 0, "L", false, 0, "")
}
