package main

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// Lageplan: freie Gärten, Vergabe eines freien Gartens direkt von der Warteliste und der Lageplan als Aushang.
// Ein Garten ist frei, wenn er im Lageplan steht, aber kein aktiver Pächter ihn hat (z. B. nach einer Kündigung).

const (
	lpBreite = 120 // Kachelgröße im Programm in Bildpunkten (wie LP_W/LP_H in app.js)
	lpHoehe  = 64
)

// belegteGaertenLocked liefert die Gartennummern mit aktivem Pächter. Der Aufrufer hält s.mu.
func (s *Store) belegteGaertenLocked() map[string]Paechter {
	out := map[string]Paechter{}
	for _, p := range s.d.Paechter {
		if nr := strings.TrimSpace(p.Gartennr); nr != "" && !p.Geloescht && p.AusgeschiedenAm == "" {
			out[nr] = p
		}
	}
	return out
}

// handleGartenVergeben macht aus einem Eintrag der Warteliste den Pächter eines freien Gartens.
func (a *App) handleGartenVergeben(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Gartennr        string  `json:"gartennr"`
		Mitgliedsnr     string  `json:"mitgliedsnr"`
		Gartengroesse   float64 `json:"gartengroesse"`
		Versand         string  `json:"versand"`
		WasserzaehlerNr string  `json:"wasserzaehlerNr"`
		StromzaehlerNr  string  `json:"stromzaehlerNr"`
		Seit            string  `json:"seit"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in.Gartennr = trim(in.Gartennr, 20)
	if in.Gartennr == "" {
		writeErr(w, bad("Bitte die Gartennummer angeben"))
		return
	}
	if in.Seit == "" {
		in.Seit = time.Now().Format("2006-01-02")
	}
	if _, err := parseDate(in.Seit); err != nil {
		writeErr(w, bad("Das Datum der Übergabe ist ungültig"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if p, belegt := a.st.belegteGaertenLocked()[in.Gartennr]; belegt {
		writeErr(w, bad("Garten "+in.Gartennr+" ist an "+p.Name+" verpachtet. Für einen Wechsel bitte »Wechsel« beim Pächter benutzen"))
		return
	}
	wi := -1
	for i := range a.st.d.Warteliste {
		if a.st.d.Warteliste[i].ID == id {
			wi = i
		}
	}
	if wi < 0 {
		writeErr(w, notFound("Eintrag auf der Warteliste nicht gefunden"))
		return
	}
	b := a.st.d.Warteliste[wi]
	versand := in.Versand
	if versand == "" {
		versand = "Postversand"
		if b.Email != "" {
			versand = "Emailsendung"
		}
	}
	p, err := cleanPaechter(Paechter{
		Mitgliedsnr: in.Mitgliedsnr, Gartennr: in.Gartennr, Anrede: b.Anrede, Name: b.Name, Strasse: b.Strasse, PLZOrt: b.PLZOrt,
		Versand: versand, Email: b.Email, Telefon: b.Telefon, Gartengroesse: in.Gartengroesse,
		WasserzaehlerNr: in.WasserzaehlerNr, StromzaehlerNr: in.StromzaehlerNr,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	if p.Gartengroesse <= 0 {
		writeErr(w, bad("Bitte die Gartengröße eintragen"))
		return
	}
	if a.st.nrTaken(p.Mitgliedsnr, "") {
		writeErr(w, bad("Die Mitgliedsnummer "+p.Mitgliedsnr+" gibt es schon"))
		return
	}
	p.ID = newID()
	a.st.d.Paechter = append(a.st.d.Paechter, p)
	a.st.ensureYear(a.st.d.Settings.Jahr).Ablesungen[p.ID] = Ablesung{}
	a.st.gartenOeffnenAb(p.ID, p.Gartennr, p.Mitgliedsnr, p.Name, in.Seit)
	a.st.d.Warteliste = append(a.st.d.Warteliste[:wi], a.st.d.Warteliste[wi+1:]...)
	a.st.audit("Garten %s von der Warteliste vergeben: %s %s", p.Gartennr, p.Mitgliedsnr, p.Name)
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, p)
}

type lageGarten struct {
	Nr, Name string
	Frei     bool
	X, Y     float64
}

// lageplanGaerten stellt die Gärten für den Ausdruck zusammen: gespeicherte Positionen oder, ohne Karte,
// ein Raster nach Gartennummer. Der Aufrufer hält s.mu.
func (s *Store) lageplanGaertenLocked(mitNamen bool) []lageGarten {
	belegt := s.belegteGaertenLocked()
	name := func(nr string) string {
		if p, ok := belegt[nr]; ok && mitNamen {
			return p.Name
		}
		return ""
	}
	var out []lageGarten
	if len(s.d.Lageplan) > 0 {
		for nr, pos := range s.d.Lageplan {
			_, ok := belegt[nr]
			out = append(out, lageGarten{Nr: nr, Name: name(nr), Frei: !ok, X: float64(pos.X), Y: float64(pos.Y)})
		}
		return out
	}
	nrs := make([]string, 0, len(belegt))
	for nr := range belegt {
		nrs = append(nrs, nr)
	}
	sort.SliceStable(nrs, func(i, j int) bool { return naturalLess(nrs[i], nrs[j]) })
	for i, nr := range nrs {
		out = append(out, lageGarten{Nr: nr, Name: name(nr), X: float64(20 + (i%8)*(lpBreite+20)), Y: float64(20 + (i/8)*(lpHoehe+30))})
	}
	return out
}

// buildLageplan zeichnet den Lageplan als Aushang (A4 quer), maßstäblich zur Anordnung im Programm. Der
// Abrechnungsstand erscheint bewusst nicht: wer was bezahlt hat, gehört nicht an die Pinnwand.
func buildLageplan(s Settings, gs []lageGarten, mitNamen bool) ([]byte, error) {
	if len(gs) == 0 {
		return nil, bad("Es gibt noch keine Gärten mit Gartennummer")
	}
	const (
		links, rechts = 12.0, 285.0
		oben, unten   = 30.0, 192.0
	)
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, g := range gs {
		minX, minY = math.Min(minX, g.X), math.Min(minY, g.Y)
		maxX, maxY = math.Max(maxX, g.X+lpBreite), math.Max(maxY, g.Y+lpHoehe)
	}
	// ein Bildpunkt im Programm = faktor mm, höchstens so groß, dass eine Kachel 40 mm breit wird
	faktor := math.Min(math.Min((rechts-links)/(maxX-minX), (unten-oben)/(maxY-minY)), 40.0/lpBreite)
	versatzX := links + ((rechts-links)-(maxX-minX)*faktor)/2
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("lib", "", fontRegular)
	pdf.AddUTF8FontFromBytes("lib", "B", fontBold)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle("Lageplan", true)
	pdf.SetAuthor(s.VereinName, true)
	pdf.SetCreator(appName, true)
	pdf.AddPage()
	mr, mg, mb := markenfarbe(s.Farbe)
	fr, fg, fb := aufgehellt(mr, mg, mb, 0.88)
	pdf.SetFont("lib", "B", 16)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(links, 11)
	pdf.CellFormat(rechts-links, 8, "Lageplan – "+s.VereinName, "", 0, "L", false, 0, "")
	pdf.SetFillColor(mr, mg, mb)
	pdf.Rect(links, 21, rechts-links, 0.8, "F")
	pdf.SetTextColor(0, 0, 0)

	frei := 0
	for _, g := range gs {
		x := versatzX + (g.X-minX)*faktor
		y := oben + (g.Y-minY)*faktor
		w, h := lpBreite*faktor, lpHoehe*faktor
		pdf.SetLineWidth(0.35)
		if g.Frei {
			frei++
			pdf.SetDrawColor(mr, mg, mb)
			pdf.SetDashPattern([]float64{1.6, 1.2}, 0)
			pdf.SetFillColor(255, 255, 255)
			pdf.RoundedRect(x, y, w, h, 1.5, "1234", "FD")
			pdf.SetDashPattern([]float64{}, 0)
		} else {
			pdf.SetDrawColor(mr, mg, mb)
			pdf.SetFillColor(fr, fg, fb)
			pdf.RoundedRect(x, y, w, h, 1.5, "1234", "FD")
		}
		groesse := math.Max(7, math.Min(16, h*0.42/0.3528*0.6))
		pdf.SetTextColor(mr, mg, mb)
		pdf.SetXY(x, y+h*0.12)
		pdf.CellFormat(w, h*0.42, passendKleinStil(pdf, "B", g.Nr, w-2, groesse, 6), "", 0, "C", false, 0, "")
		pdf.SetTextColor(60, 60, 60)
		unten := g.Name
		if g.Frei {
			unten = "frei"
		}
		if unten != "" {
			pdf.SetXY(x+1, y+h*0.56)
			pdf.CellFormat(w-2, h*0.3, passendKlein(pdf, unten, w-3, math.Max(6, groesse*0.55), 5), "", 0, "C", false, 0, "")
		}
		pdf.SetTextColor(0, 0, 0)
	}
	pdf.SetFont("lib", "", 8)
	pdf.SetTextColor(100, 100, 100)
	hinweis := fmt.Sprintf("%d Gärten", len(gs))
	if frei > 0 {
		hinweis += fmt.Sprintf(", davon %d frei (gestrichelt)", frei)
	}
	if mitNamen {
		hinweis += " · enthält Namen – nur für den Vorstand"
	}
	pdf.SetXY(links, 199)
	pdf.CellFormat(rechts-links, 4, hinweis+" · Stand "+time.Now().Format("02.01.2006"), "", 0, "L", false, 0, "")
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// lageplanPDF: Namen nur auf Wunsch und nur für den Admin (der Aushang ist sonst ohne Personenbezug).
func (a *App) lageplanMitNamen(r *http.Request) bool {
	return r.URL.Query().Get("namen") == "1" && a.isAdmin(r)
}

func (a *App) lageplanPDF(r *http.Request) ([]byte, error) {
	mitNamen := a.lageplanMitNamen(r)
	a.st.mu.Lock()
	gs := a.st.lageplanGaertenLocked(mitNamen)
	s := a.st.d.Settings
	a.st.mu.Unlock()
	return buildLageplan(s, gs, mitNamen)
}

func (a *App) handleLageplanDruck(w http.ResponseWriter, r *http.Request) {
	pdf, err := a.lageplanPDF(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="Lageplan.pdf"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(pdf)
}

func (a *App) handleLageplanDruckOeffnen(w http.ResponseWriter, r *http.Request) {
	pdf, err := a.lageplanPDF(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	full, err := a.st.schreibeDruckdatei("Lageplan.pdf", pdf)
	if err != nil {
		writeErr(w, err)
		return
	}
	openPathFn(full)
	writeJSON(w, 200, map[string]string{"datei": filepath.ToSlash(filepath.Join(druckOrdner, filepath.Base(full)))})
}
