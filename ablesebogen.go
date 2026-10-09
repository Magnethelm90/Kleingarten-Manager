package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-pdf/fpdf"
)

// ablesezeile ist eine Zeile des Ablesebogens: ein Garten mit seinen Zählern und den Vorjahresständen.
type ablesezeile struct {
	Gartennr    string
	Mitgliedsnr string
	Name        string
	WasserNr    string
	StromNr     string
	WasserVJ    *float64
	StromVJ     *float64
}

// ablesezeilenLocked bildet die Zeilen für ein Jahr, sortiert nach Gartennummer (ersatzweise
// Mitgliedsnummer). Der Aufrufer hält s.mu.
func (s *Store) ablesezeilenLocked(year int) ([]ablesezeile, Settings, bool) {
	v, ok := s.viewLocked(year)
	if !ok {
		return nil, Settings{}, false
	}
	rows := make([]ablesezeile, 0, len(v.Paechter))
	for _, p := range v.Paechter {
		a := v.Ablesungen[p.ID]
		rows = append(rows, ablesezeile{
			Gartennr: p.Gartennr, Mitgliedsnr: p.Mitgliedsnr, Name: p.Name,
			WasserNr: p.WasserzaehlerNr, StromNr: p.StromzaehlerNr, WasserVJ: a.WasserVJ, StromVJ: a.StromVJ,
		})
	}
	key := func(r ablesezeile) string {
		if r.Gartennr != "" {
			return r.Gartennr
		}
		return r.Mitgliedsnr
	}
	sort.SliceStable(rows, func(i, j int) bool { return naturalLess(key(rows[i]), key(rows[j])) })
	return rows, v.Settings, true
}

// buildAblesebogen erzeugt den Ablesebogen als A4-Querformat: je Garten eine Zeile mit Zählernummern,
// Vorjahresständen und leeren Feldern zum Eintragen beim Rundgang.
func buildAblesebogen(s Settings, year int, rows []ablesezeile) ([]byte, error) {
	if len(rows) == 0 {
		return nil, errors.New("Es sind keine Pächter angelegt")
	}
	const (
		left   = 10.0
		right  = 287.0
		rowH   = 8.6
		bottom = 192.0
	)
	cols := []struct {
		titel string
		w     float64
	}{
		{"Garten", 15}, {"Name", 56}, {"Wasserzähler-Nr.", 33}, {"Wasser Vorjahr", 27}, {"Wasser aktuell", 30},
		{"Stromzähler-Nr.", 33}, {"Strom Vorjahr", 27}, {"Strom aktuell", 30}, {"Std.", 26},
	}

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("lib", "", fontRegular)
	pdf.AddUTF8FontFromBytes("lib", "B", fontBold)
	pdf.SetMargins(left, 12, 10)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle(fmt.Sprintf("Ablesebogen %d", year), true)
	pdf.SetAuthor(s.VereinName, true)
	pdf.SetCreator(appName, true)
	pdf.AliasNbPages("{nb}")
	pdf.SetFooterFunc(func() {
		pdf.SetY(-9)
		pdf.SetFont("lib", "", 8)
		pdf.SetTextColor(90, 90, 90)
		pdf.CellFormat(right-left, 4, fmt.Sprintf("%s · Ablesebogen %d · Seite %d von {nb}", s.VereinName, year, pdf.PageNo()), "", 0, "R", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	})

	// fit kürzt einen Text so, dass er in die Spalte passt
	fit := func(txt string, w float64) string {
		r := []rune(txt)
		for len(r) > 1 && pdf.GetStringWidth(string(r)) > w-2 {
			r = r[:len(r)-1]
			txt = string(r) + "…"
		}
		return txt
	}
	zahl := func(v *float64) string {
		if v == nil {
			return "–"
		}
		return fmtFlex(*v)
	}

	var y float64
	kopf := func() {
		pdf.AddPage()
		pdf.SetFont("lib", "B", 15)
		pdf.SetXY(left, 12)
		pdf.CellFormat(right-left, 7, fmt.Sprintf("Ablesebogen %d – %s", year, s.VereinName), "", 0, "L", false, 0, "")
		pdf.SetFont("lib", "", 10)
		pdf.SetXY(left, 21)
		pdf.CellFormat(right-left, 6, "Datum der Ablesung: ______________________      Abgelesen von: ______________________________      Eintragen ins Programm: Reiter »Zählerstände«", "", 0, "L", false, 0, "")
		y = 30
		pdf.SetFont("lib", "B", 9)
		pdf.SetFillColor(230, 238, 232)
		pdf.SetDrawColor(110, 110, 110)
		pdf.SetLineWidth(0.25)
		x := left
		for _, c := range cols {
			pdf.SetXY(x, y)
			pdf.CellFormat(c.w, 7.5, c.titel, "1", 0, "C", true, 0, "")
			x += c.w
		}
		y += 7.5
	}
	kopf()
	for _, r := range rows {
		if y+rowH > bottom {
			kopf()
		}
		name := r.Name
		if r.Mitgliedsnr != "" {
			name += "  (" + r.Mitgliedsnr + ")"
		}
		cells := []struct {
			txt, style string
			size       float64
			align      string
		}{
			{r.Gartennr, "B", 11, "C"}, {name, "", 9.5, "L"}, {r.WasserNr, "", 8, "L"}, {zahl(r.WasserVJ), "", 10, "R"}, {"", "", 10, "R"},
			{r.StromNr, "", 8, "L"}, {zahl(r.StromVJ), "", 10, "R"}, {"", "", 10, "R"}, {"", "", 10, "R"},
		}
		x := left
		for i, c := range cells {
			pdf.SetFont("lib", c.style, c.size)
			pdf.SetXY(x, y)
			txt := strings.TrimSpace(c.txt)
			if i == 1 || i == 2 || i == 5 {
				txt = fit(txt, cols[i].w)
			}
			pdf.CellFormat(cols[i].w, rowH, txt, "1", 0, c.align, false, 0, "")
			x += cols[i].w
		}
		y += rowH
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *App) ablesebogenPDF(r *http.Request) ([]byte, int, error) {
	year := a.yearParam(r)
	a.st.mu.Lock()
	rows, settings, ok := a.st.ablesezeilenLocked(year)
	a.st.mu.Unlock()
	if !ok {
		return nil, year, notFound("Dieses Jahr gibt es nicht")
	}
	pdf, err := buildAblesebogen(settings, year, rows)
	if err != nil {
		return nil, year, bad(err.Error())
	}
	return pdf, year, nil
}

// handleAblesebogen liefert den Ablesebogen eines Jahres als PDF zum Ausdrucken.
func (a *App) handleAblesebogen(w http.ResponseWriter, r *http.Request) {
	pdf, year, err := a.ablesebogenPDF(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="Ablesebogen_%d.pdf"`, year))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(pdf)
}

// handleAblesebogenOeffnen legt den Ablesebogen im Druckordner ab und öffnet ihn im PDF-Programm
// (für das Programmfenster, in dem es keine Browser-Tabs gibt).
func (a *App) handleAblesebogenOeffnen(w http.ResponseWriter, r *http.Request) {
	pdf, year, err := a.ablesebogenPDF(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	full, err := a.st.schreibeDruckdatei(fmt.Sprintf("Ablesebogen_%d.pdf", year), pdf)
	if err != nil {
		writeErr(w, err)
		return
	}
	openPathFn(full)
	writeJSON(w, 200, map[string]string{"datei": filepath.ToSlash(filepath.Join(druckOrdner, filepath.Base(full)))})
}
