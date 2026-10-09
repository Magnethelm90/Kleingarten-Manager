package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// buildMitgliederliste erzeugt die Pächterliste als A4-Querformat: Garten, Name, Anschrift, Größe,
// Versandart und Zähler, nach Gartennummer (ersatzweise Mitgliedsnummer) sortiert.
func buildMitgliederliste(s Settings, ps []Paechter) ([]byte, error) {
	if len(ps) == 0 {
		return nil, errors.New("Es sind keine Pächter angelegt")
	}
	ps = append([]Paechter(nil), ps...)
	key := func(p Paechter) string {
		if p.Gartennr != "" {
			return p.Gartennr
		}
		return p.Mitgliedsnr
	}
	sort.SliceStable(ps, func(i, j int) bool { return naturalLess(key(ps[i]), key(ps[j])) })

	const (
		left   = 10.0
		right  = 287.0
		rowH   = 7.2
		bottom = 192.0
	)
	cols := []struct {
		titel string
		w     float64
	}{
		{"Garten", 16}, {"Mitgl.-Nr.", 22}, {"Name", 58}, {"Anschrift", 92}, {"Größe m²", 20}, {"Versand", 31}, {"Zähler Wasser / Strom", 38},
	}
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("lib", "", fontRegular)
	pdf.AddUTF8FontFromBytes("lib", "B", fontBold)
	pdf.SetMargins(left, 12, 10)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle("Pächterliste", true)
	pdf.SetAuthor(s.VereinName, true)
	pdf.SetCreator(appName, true)
	pdf.AliasNbPages("{nb}")
	stand := time.Now().Format("02.01.2006")
	pdf.SetFooterFunc(func() {
		pdf.SetY(-9)
		pdf.SetFont("lib", "", 8)
		pdf.SetTextColor(90, 90, 90)
		pdf.CellFormat(right-left, 4, fmt.Sprintf("%s · Pächterliste, Stand %s · vertraulich – enthält personenbezogene Daten · Seite %d von {nb}", s.VereinName, stand, pdf.PageNo()), "", 0, "R", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	})
	fit := func(txt string, w float64) string {
		r := []rune(txt)
		for len(r) > 1 && pdf.GetStringWidth(string(r)) > w-2 {
			r = r[:len(r)-1]
			txt = string(r) + "…"
		}
		return txt
	}
	var y float64
	kopf := func() {
		pdf.AddPage()
		pdf.SetFont("lib", "B", 15)
		pdf.SetXY(left, 12)
		pdf.CellFormat(right-left, 7, "Pächterliste – "+s.VereinName, "", 0, "L", false, 0, "")
		y = 24
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
	for _, p := range ps {
		if y+rowH > bottom {
			kopf()
		}
		anschrift := strings.TrimSpace(strings.Join(nonEmpty(p.Strasse, p.PLZOrt), ", "))
		zaehler := strings.TrimSpace(strings.Join(nonEmpty(p.WasserzaehlerNr, p.StromzaehlerNr), " / "))
		cells := []struct {
			txt, style string
			align      string
		}{
			{p.Gartennr, "B", "C"}, {p.Mitgliedsnr, "", "C"}, {p.Name, "", "L"}, {anschrift, "", "L"},
			{fmtFlex(p.Gartengroesse), "", "R"}, {p.Versand, "", "L"}, {zaehler, "", "L"},
		}
		x := left
		for i, c := range cells {
			pdf.SetFont("lib", c.style, 9)
			pdf.SetXY(x, y)
			pdf.CellFormat(cols[i].w, rowH, fit(strings.TrimSpace(c.txt), cols[i].w), "1", 0, c.align, false, 0, "")
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

func nonEmpty(xs ...string) []string {
	var out []string
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			out = append(out, strings.TrimSpace(x))
		}
	}
	return out
}

func (a *App) mitgliederlistePDF() ([]byte, error) {
	a.st.mu.Lock()
	var ps []Paechter
	for _, p := range a.st.d.Paechter {
		if !p.Geloescht {
			ps = append(ps, p)
		}
	}
	settings := a.st.d.Settings
	a.st.mu.Unlock()
	pdf, err := buildMitgliederliste(settings, ps)
	if err != nil {
		return nil, bad(err.Error())
	}
	return pdf, nil
}

// handleMitgliederliste liefert die Pächterliste als PDF (nur Admin: enthält Anschriften).
func (a *App) handleMitgliederliste(w http.ResponseWriter, r *http.Request) {
	pdf, err := a.mitgliederlistePDF()
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="Paechterliste.pdf"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(pdf)
}

// handleMitgliederlisteOeffnen legt die Liste im Druckordner ab und öffnet sie im PDF-Programm.
func (a *App) handleMitgliederlisteOeffnen(w http.ResponseWriter, r *http.Request) {
	pdf, err := a.mitgliederlistePDF()
	if err != nil {
		writeErr(w, err)
		return
	}
	full, err := a.st.schreibeDruckdatei("Paechterliste.pdf", pdf)
	if err != nil {
		writeErr(w, err)
		return
	}
	openPathFn(full)
	writeJSON(w, 200, map[string]string{"datei": filepath.ToSlash(filepath.Join(druckOrdner, filepath.Base(full)))})
}
