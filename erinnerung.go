package main

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Zählerstand-Erinnerung: Wer hat Wasser- oder Stromzähler im laufenden Abrechnungsjahr noch nicht gemeldet?
// Die Liste ist für den Admin (mit Namen und E-Mail); der Aushang nennt nur Gartennummern, keine Namen.

type fehlt struct {
	ID          string   `json:"id"`
	Gartennr    string   `json:"gartennr"`
	Name        string   `json:"name"`
	Email       string   `json:"email,omitempty"`
	Fehlende    []string `json:"fehlt"`
	Mitgliedsnr string   `json:"mitgliedsnr"`
}

type erinnerungDaten struct {
	Jahr    int     `json:"jahr"`
	Fehlend []fehlt `json:"fehlend"`
	Gesamt  int     `json:"gesamt"` // Pächter mit mindestens einem Zähler
}

// zaehlerFehlenLocked sammelt alle Pächter des Jahres, bei denen ein vorhandener Zähler noch keinen aktuellen Stand hat.
// Ein Zähler gilt als vorhanden, wenn eine Zählernummer oder ein Vorjahresstand eingetragen ist.
func (s *Store) zaehlerFehlenLocked(year int) (erinnerungDaten, bool) {
	v, ok := s.viewLocked(year)
	if !ok {
		return erinnerungDaten{}, false
	}
	out := erinnerungDaten{Jahr: year, Fehlend: []fehlt{}}
	for _, p := range v.Paechter {
		if p.AusgeschiedenAm != "" || (p.Geloescht && !v.ReadOnly) {
			continue
		}
		a := v.Ablesungen[p.ID]
		wasser := strings.TrimSpace(p.WasserzaehlerNr) != "" || a.WasserVJ != nil
		strom := strings.TrimSpace(p.StromzaehlerNr) != "" || a.StromVJ != nil
		if !wasser && !strom {
			continue
		}
		out.Gesamt++
		var f []string
		if wasser && a.WasserAkt == nil {
			f = append(f, "Wasser")
		}
		if strom && a.StromAkt == nil {
			f = append(f, "Strom")
		}
		if len(f) > 0 {
			out.Fehlend = append(out.Fehlend, fehlt{ID: p.ID, Gartennr: p.Gartennr, Name: p.Name, Email: p.Email, Fehlende: f, Mitgliedsnr: p.Mitgliedsnr})
		}
	}
	sort.SliceStable(out.Fehlend, func(i, j int) bool { return naturalLess(out.Fehlend[i].Gartennr, out.Fehlend[j].Gartennr) })
	return out, true
}

func (a *App) handleZaehlerFehlen(w http.ResponseWriter, r *http.Request) {
	year := a.yearParam(r)
	a.st.mu.Lock()
	d, ok := a.st.zaehlerFehlenLocked(year)
	a.st.mu.Unlock()
	if !ok {
		writeErr(w, notFound("Dieses Jahr gibt es nicht"))
		return
	}
	writeJSON(w, 200, d)
}

// fristParam liest ein Datum aus dem Parameter frist; leer oder ungültig = ersatzweise heute + Tage.
func fristParam(r *http.Request, tage int) string {
	f := strings.TrimSpace(r.URL.Query().Get("frist"))
	if _, err := parseDate(f); err != nil {
		f = time.Now().AddDate(0, 0, tage).Format("2006-01-02")
	}
	return f
}

func buildZaehlerAushang(s Settings, d erinnerungDaten, frist string) ([]byte, error) {
	if len(d.Fehlend) == 0 {
		return nil, bad("Es fehlen keine Zählerstände – ein Aushang ist nicht nötig")
	}
	pdf := neuesPDF(s, fmt.Sprintf("Zählerstände %d", d.Jahr), false)
	mr, mg, mb := markenfarbe(s.Farbe)
	fr, fg, fb := aufgehellt(mr, mg, mb, 0.9)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	y := 38.0
	pdf.SetFont("lib", "B", 22)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 10, "Zählerstände fehlen noch", "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	y += 15
	pdf.SetFont("lib", "", 11.5)
	pdf.SetXY(leftX, y)
	pdf.MultiCell(rightX-leftX, 6, fmt.Sprintf("Für die Abrechnung %d fehlen uns noch die Zählerstände der folgenden Gärten. "+
		"Bitte teilt sie dem Vorstand bis spätestens %s mit (Wasser in m³, Strom in kWh).", d.Jahr, germanDate(frist)), "", "L", false)
	y = pdf.GetY() + 8

	const cols, boxW, boxH, gap = 4, 38.0, 17.0, 6.0
	for i, f := range d.Fehlend {
		col := i % cols
		if col == 0 && i > 0 {
			y += boxH + gap
		}
		if y+boxH > 272 {
			pdf.AddPage()
			y = 20
		}
		x := leftX + float64(col)*(boxW+gap)
		pdf.SetFillColor(fr, fg, fb)
		pdf.SetDrawColor(mr, mg, mb)
		pdf.SetLineWidth(0.3)
		pdf.RoundedRect(x, y, boxW, boxH, 2, "1234", "FD")
		pdf.SetFont("lib", "B", 13)
		pdf.SetTextColor(mr, mg, mb)
		pdf.SetXY(x, y+2)
		pdf.CellFormat(boxW, 7, passendKleinStil(pdf, "B", "Garten "+f.Gartennr, boxW-3, 13, 8), "", 0, "C", false, 0, "")
		pdf.SetFont("lib", "", 9.5)
		pdf.SetTextColor(60, 60, 60)
		pdf.SetXY(x, y+9.5)
		pdf.CellFormat(boxW, 5, strings.Join(f.Fehlende, " + "), "", 0, "C", false, 0, "")
	}
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("lib", "", 7.5)
	pdf.SetTextColor(110, 110, 110)
	pdf.SetXY(leftX, 287)
	pdf.CellFormat(rightX-leftX, 4, fmt.Sprintf("%s · Stand %s", s.VereinName, time.Now().Format("02.01.2006")), "", 0, "C", false, 0, "")
	return pdfBytes(pdf)
}

func (a *App) zaehlerAushangPDF(r *http.Request) ([]byte, string, error) {
	year := a.yearParam(r)
	a.st.mu.Lock()
	d, ok := a.st.zaehlerFehlenLocked(year)
	s := a.st.d.Settings
	a.st.mu.Unlock()
	if !ok {
		return nil, "", notFound("Dieses Jahr gibt es nicht")
	}
	pdf, err := buildZaehlerAushang(s, d, fristParam(r, 14))
	return pdf, fmt.Sprintf("Zaehlerstaende_fehlen_%d.pdf", year), err
}

const maxMailtoLaenge = 1800

// handleZaehlerMail öffnet das E-Mail-Programm mit einer Erinnerung an alle, die noch keinen Stand gemeldet
// haben und eine E-Mail-Adresse hinterlegt haben. Die Adressen stehen im Feld »Bcc«, damit niemand die
// Adressen der anderen sieht. Passt nicht alles in einen mailto-Link, wird der Rest gemeldet.
func (a *App) handleZaehlerMail(w http.ResponseWriter, r *http.Request) {
	year := a.yearParam(r)
	a.st.mu.Lock()
	d, ok := a.st.zaehlerFehlenLocked(year)
	verein := a.st.d.Settings.VereinName
	a.st.mu.Unlock()
	if !ok {
		writeErr(w, notFound("Dieses Jahr gibt es nicht"))
		return
	}
	frist := fristParam(r, 14)
	betreff := fmt.Sprintf("Erinnerung: Zählerstände %d", year)
	text := fmt.Sprintf("Liebe Gartenfreundin, lieber Gartenfreund,\n\nfür die Abrechnung %d fehlen uns noch die Zählerstände für Wasser (m³) und Strom (kWh) deines Gartens. "+
		"Bitte teile sie uns bis spätestens %s mit.\n\nVielen Dank!\nDer Vorstand\n%s", year, germanDate(frist), verein)
	basis := "mailto:?subject=" + mailtoEscape(betreff) + "&body=" + mailtoEscape(text) + "&bcc="
	var adressen []string
	laenge := len(basis)
	ohne := 0
	for _, f := range d.Fehlend {
		if !emailGueltig(f.Email) {
			ohne++
			continue
		}
		e := url.QueryEscape(f.Email)
		if laenge+len(e)+3 > maxMailtoLaenge {
			ohne++
			continue
		}
		adressen = append(adressen, e)
		laenge += len(e) + 3
	}
	if len(adressen) == 0 {
		writeErr(w, bad("Bei den Gärten ohne Zählerstand ist keine gültige E-Mail-Adresse hinterlegt. Der Aushang ist die Alternative"))
		return
	}
	openMailFn(basis + strings.Join(adressen, ","))
	writeJSON(w, 200, map[string]int{"empfaenger": len(adressen), "ohneMail": ohne})
}
