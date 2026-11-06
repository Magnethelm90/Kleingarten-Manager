package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// Mitgliederversammlung: Einladung mit Tagesordnung und Anwesenheitsliste zum Unterschreiben, jeweils zu einem
// Termin aus dem Jahresplaner. Die Einladung enthält keine Personendaten; die Anwesenheitsliste nennt alle
// Pächter und ist deshalb nur für den Admin abrufbar.

var standardTops = []string{
	"Begrüßung und Feststellung der Beschlussfähigkeit",
	"Genehmigung der Tagesordnung und des letzten Protokolls",
	"Bericht des Vorstands",
	"Kassenbericht",
	"Bericht der Kassenprüfer",
	"Aussprache und Entlastung des Vorstands",
	"Termine und Vorhaben des kommenden Jahres",
	"Verschiedenes",
}

func (a *App) terminNachID(id string) (Termin, Settings, bool) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for _, t := range a.st.d.Termine {
		if t.ID == id {
			return t, a.st.d.Settings, true
		}
	}
	return Termin{}, a.st.d.Settings, false
}

// tagesordnung liefert die Punkte der Einladung; bei Versammlungen ersatzweise die üblichen Punkte.
func tagesordnung(t Termin) []string {
	if t.Tops != "" {
		return strings.Split(t.Tops, "\n")
	}
	if t.Art == "Versammlung" {
		return standardTops
	}
	return nil
}

func terminWann(t Termin) string {
	d, err := parseDate(t.Datum)
	if err != nil {
		return germanDate(t.Datum)
	}
	s := fmt.Sprintf("%s, %s", langeWochentage[d.Weekday()], d.Format("02.01.2006"))
	if t.Von != "" {
		s += ", " + t.Von
		if t.Bis != "" {
			s += "–" + t.Bis
		}
		s += " Uhr"
	}
	return s
}

func buildEinladung(s Settings, t Termin) ([]byte, error) {
	pdf := neuesPDF(s, "Einladung "+t.Titel, false)
	mr, mg, mb := markenfarbe(s.Farbe)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	y := 40.0
	pdf.SetFont("lib", "", 11)
	pdf.SetTextColor(90, 96, 90)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 6, "Einladung", "", 0, "L", false, 0, "")
	y += 7
	pdf.SetFont("lib", "B", 20)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 10, passendKleinStil(pdf, "B", t.Titel, rightX-leftX, 20, 12), "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	y += 15

	fr, fg, fb := aufgehellt(mr, mg, mb, 0.9)
	zeilen := [][2]string{{"Wann", terminWann(t)}}
	if t.Ort != "" {
		zeilen = append(zeilen, [2]string{"Wo", t.Ort})
	}
	pdf.SetFillColor(fr, fg, fb)
	pdf.RoundedRect(leftX, y, rightX-leftX, 4+float64(len(zeilen))*7, 2, "1234", "F")
	y += 3
	for _, z := range zeilen {
		pdf.SetFont("lib", "B", 11)
		pdf.SetTextColor(mr, mg, mb)
		pdf.SetXY(leftX+5, y)
		pdf.CellFormat(18, 7, z[0], "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetXY(leftX+25, y)
		pdf.CellFormat(rightX-leftX-30, 7, passendKleinStil(pdf, "B", z[1], rightX-leftX-30, 12, 8), "", 0, "L", false, 0, "")
		y += 7
	}
	y += 11

	pdf.SetFont("lib", "", 11)
	pdf.SetXY(leftX, y)
	pdf.MultiCell(rightX-leftX, 5.6, "Liebe Gartenfreundinnen und Gartenfreunde,\n\nder Vorstand lädt alle Mitglieder herzlich zu dieser Veranstaltung ein. Um zahlreiches Erscheinen wird gebeten.", "", "L", false)
	y = pdf.GetY() + 6

	if top := tagesordnung(t); len(top) > 0 {
		pdf.SetFont("lib", "B", 12)
		pdf.SetTextColor(mr, mg, mb)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(rightX-leftX, 7, "Tagesordnung", "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		y += 9
		for i, p := range top {
			if y > 270 {
				pdf.AddPage()
				y = 20
			}
			pdf.SetFont("lib", "B", 11)
			pdf.SetXY(leftX, y)
			pdf.CellFormat(10, 6, fmt.Sprintf("TOP %d", i+1), "", 0, "L", false, 0, "")
			pdf.SetFont("lib", "", 11)
			pdf.SetXY(leftX+16, y)
			pdf.MultiCell(rightX-leftX-16, 6, p, "", "L", false)
			y = pdf.GetY() + 1.5
		}
		y += 4
	}
	if t.Notiz != "" {
		if y > 250 {
			pdf.AddPage()
			y = 20
		}
		pdf.SetFont("lib", "B", 11)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(rightX-leftX, 6, "Hinweise", "", 0, "L", false, 0, "")
		y += 7
		pdf.SetFont("lib", "", 11)
		pdf.SetXY(leftX, y)
		pdf.MultiCell(rightX-leftX, 5.6, t.Notiz, "", "L", false)
		y = pdf.GetY() + 4
	}
	if y > 260 {
		pdf.AddPage()
	}
	pdf.SetFont("lib", "", 11)
	pdf.SetXY(leftX, 266)
	pdf.CellFormat(rightX-leftX, 6, "Mit freundlichen Gartengrüßen", "", 0, "L", false, 0, "")
	pdf.SetXY(leftX, 272)
	pdf.CellFormat(rightX-leftX, 6, "Der Vorstand", "", 0, "L", false, 0, "")
	pdf.SetFont("lib", "", 7.5)
	pdf.SetTextColor(110, 110, 110)
	pdf.SetXY(leftX, 287)
	pdf.CellFormat(rightX-leftX, 4, fmt.Sprintf("%s · %s", s.VereinName, s.Ort), "", 0, "C", false, 0, "")
	return pdfBytes(pdf)
}

type anwesenheitZeile struct{ Garten, Nr, Name string }

func (s *Store) anwesenheitLocked() []anwesenheitZeile {
	var l []anwesenheitZeile
	for _, p := range s.d.Paechter {
		if p.Geloescht || p.AusgeschiedenAm != "" {
			continue
		}
		l = append(l, anwesenheitZeile{p.Gartennr, p.Mitgliedsnr, p.Name})
	}
	key := func(z anwesenheitZeile) string {
		if z.Garten != "" {
			return z.Garten
		}
		return z.Nr
	}
	sort.SliceStable(l, func(i, j int) bool { return naturalLess(key(l[i]), key(l[j])) })
	return l
}

func buildAnwesenheit(s Settings, t Termin, l []anwesenheitZeile) ([]byte, error) {
	if len(l) == 0 {
		return nil, bad("Es sind keine Pächter angelegt")
	}
	pdf := neuesPDF(s, "Anwesenheitsliste "+t.Titel, false)
	pdf.AliasNbPages("{nb}")
	mr, mg, mb := markenfarbe(s.Farbe)
	fr, fg, fb := aufgehellt(mr, mg, mb, 0.9)
	const (
		rowH   = 9.0
		bottom = 258.0
	)
	cols := []struct {
		titel string
		w     float64
	}{{"Garten", 17}, {"Mitgl.-Nr.", 20}, {"Name", 58}, {"Unterschrift", 75}}
	var y float64
	seite := func() {
		pdf.AddPage()
		pdf.SetFont("lib", "B", 16)
		pdf.SetTextColor(mr, mg, mb)
		pdf.SetXY(leftX, 14)
		pdf.CellFormat(rightX-leftX, 8, "Anwesenheitsliste", "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("lib", "B", 11)
		pdf.SetXY(leftX, 23)
		pdf.CellFormat(rightX-leftX, 6, passendKleinStil(pdf, "B", t.Titel, rightX-leftX, 11, 8), "", 0, "L", false, 0, "")
		pdf.SetFont("lib", "", 10)
		pdf.SetXY(leftX, 29)
		pdf.CellFormat(rightX-leftX, 5, strings.Join(nonEmpty(terminWann(t), t.Ort), " · "), "", 0, "L", false, 0, "")
		y = 38
		pdf.SetFillColor(fr, fg, fb)
		pdf.SetDrawColor(120, 120, 120)
		pdf.SetLineWidth(0.25)
		pdf.SetFont("lib", "B", 9.5)
		x := leftX
		for _, c := range cols {
			pdf.SetXY(x, y)
			pdf.CellFormat(c.w, 7.5, c.titel, "1", 0, "L", true, 0, "")
			x += c.w
		}
		y += 7.5
		pdf.SetFont("lib", "", 7.5)
		pdf.SetTextColor(110, 110, 110)
		pdf.SetXY(leftX, 287)
		pdf.CellFormat(rightX-leftX, 4, fmt.Sprintf("%s · Anwesenheitsliste · Seite %d von {nb}", s.VereinName, pdf.PageNo()), "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}
	seite()
	for _, z := range l {
		if y+rowH > bottom {
			seite()
		}
		x := leftX
		for i, txt := range []string{z.Garten, z.Nr, z.Name, ""} {
			pdf.SetFont("lib", "", 10)
			pdf.SetXY(x, y)
			pdf.CellFormat(cols[i].w, rowH, passendKlein(pdf, txt, cols[i].w-2, 10, 7), "1", 0, "L", false, 0, "")
			x += cols[i].w
		}
		y += rowH
	}
	if y+34 > 280 {
		seite()
	}
	y += 8
	pdf.SetFont("lib", "", 10.5)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 6, fmt.Sprintf("Anwesende Mitglieder: ________   von %d Pächtern        Beschlussfähig:  [   ] ja    [   ] nein", len(l)), "", 0, "L", false, 0, "")
	y += 16
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 6, "Versammlungsleitung: ______________________      Protokoll: ______________________", "", 0, "L", false, 0, "")
	return pdfBytes(pdf)
}

func (a *App) einladungPDF(r *http.Request) ([]byte, string, error) {
	t, s, ok := a.terminNachID(r.PathValue("id"))
	if !ok {
		return nil, "", notFound("Termin nicht gefunden")
	}
	pdf, err := buildEinladung(s, t)
	return pdf, safeName("Einladung_"+t.Titel, 80) + ".pdf", err
}

func (a *App) anwesenheitPDF(r *http.Request) ([]byte, string, error) {
	t, s, ok := a.terminNachID(r.PathValue("id"))
	if !ok {
		return nil, "", notFound("Termin nicht gefunden")
	}
	a.st.mu.Lock()
	l := a.st.anwesenheitLocked()
	a.st.mu.Unlock()
	pdf, err := buildAnwesenheit(s, t, l)
	return pdf, safeName("Anwesenheitsliste_"+t.Titel, 80) + ".pdf", err
}
