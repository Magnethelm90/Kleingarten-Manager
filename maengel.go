package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// Gartenbegehung und Mängel: Beim Rundgang festgestellte Mängel je Garten mit Frist erfassen, als erledigt
// markieren und dem Pächter eine schriftliche Aufforderung schicken. Die Begehungsliste ist ein Vordruck
// zum Mitnehmen. Nur für den Admin; Mängel gehören zum Pächter des Gartens und werden mit ihm gelöscht.

var mangelArten = []string{"Pflege und Unkraut", "Hecke und Bäume", "Laube und Bauten", "Abfall und Lagerung", "Wege und Zäune", "Wasser und Strom", "Sonstiges"}

// Mangel ist ein bei einer Begehung festgestellter Mangel.
type Mangel struct {
	ID           string `json:"id"`
	Gartennr     string `json:"gartennr"`
	PaechterID   string `json:"paechterId,omitempty"` // Pächter zur Zeit der Feststellung (für Datenschutz und Auskunft)
	Datum        string `json:"datum"`                // Tag der Begehung
	Art          string `json:"art"`
	Beschreibung string `json:"beschreibung"`
	Frist        string `json:"frist,omitempty"`
	ErledigtAm   string `json:"erledigtAm,omitempty"`
}

type mangelAnsicht struct {
	Mangel
	Name string `json:"name"`
}

const maxMaengel = 3000

func cleanMangel(m Mangel) (Mangel, error) {
	m.Gartennr = trim(m.Gartennr, 20)
	m.Beschreibung = trim(strings.Join(strings.Fields(m.Beschreibung), " "), 300)
	if m.Gartennr == "" {
		return m, bad("Bitte die Gartennummer angeben")
	}
	if m.Beschreibung == "" {
		return m, bad("Bitte beschreiben, was zu beseitigen ist, z. B. »Hecke über 1,25 m, bitte auf 1,25 m zurückschneiden«")
	}
	d, err := parseDate(m.Datum)
	if err != nil || d.Year() < 2000 || d.Year() > 2100 {
		return m, bad("Bitte das Datum der Begehung eintragen")
	}
	for _, x := range []string{m.Frist, m.ErledigtAm} {
		if x != "" {
			if f, err := parseDate(x); err != nil || f.Year() < 2000 || f.Year() > 2100 {
				return m, bad("Frist und Erledigt-Datum müssen gültige Daten sein")
			}
		}
	}
	gueltig := false
	for _, a := range mangelArten {
		if a == m.Art {
			gueltig = true
		}
	}
	if !gueltig {
		m.Art = "Sonstiges"
	}
	return m, nil
}

func (a *App) handleMaengelListe(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	namen := map[string]string{}
	for _, p := range a.st.d.Paechter {
		namen[p.ID] = p.Name
	}
	out := make([]mangelAnsicht, 0, len(a.st.d.Maengel))
	for _, m := range a.st.d.Maengel {
		n := ""
		if p, ok := a.st.aktiverPaechterLocked(m.Gartennr); ok {
			n = p.Name
		} else if m.PaechterID != "" {
			n = namen[m.PaechterID]
		}
		out = append(out, mangelAnsicht{m, n})
	}
	writeJSON(w, 200, out)
}

func (a *App) handleMangelSpeichern(neu bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in Mangel
		if err := readJSON(w, r, &in); err != nil {
			writeErr(w, err)
			return
		}
		in, err := cleanMangel(in)
		if err != nil {
			writeErr(w, err)
			return
		}
		a.st.mu.Lock()
		defer a.st.mu.Unlock()
		p, ok := a.st.aktiverPaechterLocked(in.Gartennr)
		if neu {
			if !ok {
				writeErr(w, bad(fmt.Sprintf("Für Garten %s ist kein Pächter eingetragen", in.Gartennr)))
				return
			}
			if len(a.st.d.Maengel) >= maxMaengel {
				writeErr(w, bad("Es sind sehr viele Mängel gespeichert. Bitte zuerst erledigte löschen"))
				return
			}
			in.ID, in.PaechterID = newID(), p.ID
			a.st.d.Maengel = append(a.st.d.Maengel, in)
			a.st.audit("Mangel festgestellt: Garten %s (%s)", in.Gartennr, in.Art)
		} else {
			id := r.PathValue("id")
			idx := -1
			for i, m := range a.st.d.Maengel {
				if m.ID == id {
					idx = i
				}
			}
			if idx < 0 {
				writeErr(w, notFound("Mangel nicht gefunden"))
				return
			}
			old := a.st.d.Maengel[idx]
			in.ID, in.PaechterID = id, old.PaechterID
			if in.Gartennr != old.Gartennr { // anderer Garten: Pächter neu zuordnen
				if !ok {
					writeErr(w, bad(fmt.Sprintf("Für Garten %s ist kein Pächter eingetragen", in.Gartennr)))
					return
				}
				in.PaechterID = p.ID
			}
			a.st.d.Maengel[idx] = in
			if old.ErledigtAm == "" && in.ErledigtAm != "" {
				a.st.audit("Mangel erledigt: Garten %s (%s)", in.Gartennr, in.Art)
			} else {
				a.st.audit("Mangel geändert: Garten %s (%s)", in.Gartennr, in.Art)
			}
		}
		if err := a.st.saveLocked(); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 200, in)
	}
}

func (a *App) handleMangelLoeschen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i, m := range a.st.d.Maengel {
		if m.ID == id {
			a.st.d.Maengel = append(a.st.d.Maengel[:i], a.st.d.Maengel[i+1:]...)
			a.st.audit("Mangel gelöscht: Garten %s (%s)", m.Gartennr, m.Art)
			if err := a.st.saveLocked(); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, map[string]bool{"ok": true})
			return
		}
	}
	writeErr(w, notFound("Mangel nicht gefunden"))
}

// ------------------------------------------------------------------ Begehungsliste (Vordruck)

type begehungZeile struct {
	Garten, Name string
	Offen        []string
	Anbau        string // letzte Schätzung als Hinweis, z. B. »zuletzt 25«
}

func (s *Store) begehungLocked() []begehungZeile {
	letzteNutzung := map[string]*Nutzung{}
	for i := range s.d.Nutzungen {
		n := &s.d.Nutzungen[i]
		if v := letzteNutzung[n.Gartennr]; v == nil || n.Datum >= v.Datum {
			letzteNutzung[n.Gartennr] = n
		}
	}
	var l []begehungZeile
	for _, p := range s.d.Paechter {
		if p.Geloescht || p.AusgeschiedenAm != "" {
			continue
		}
		z := begehungZeile{Garten: p.Gartennr, Name: p.Name}
		if l := letzteNutzung[strings.TrimSpace(p.Gartennr)]; l != nil {
			z.Anbau = "zuletzt " + fmtFlex(l.Anbau)
		}
		for _, m := range s.d.Maengel {
			if m.Gartennr == strings.TrimSpace(p.Gartennr) && m.ErledigtAm == "" {
				z.Offen = append(z.Offen, m.Beschreibung)
			}
		}
		l = append(l, z)
	}
	sort.SliceStable(l, func(i, j int) bool { return naturalLess(l[i].Garten, l[j].Garten) })
	return l
}

func buildBegehungsliste(s Settings, l []begehungZeile) ([]byte, error) {
	if len(l) == 0 {
		return nil, bad("Es sind keine Pächter angelegt")
	}
	pdf := neuesPDF(s, "Begehungsliste", true)
	pdf.AliasNbPages("{nb}")
	mr, mg, mb := markenfarbe(s.Farbe)
	fr, fg, fb := aufgehellt(mr, mg, mb, 0.9)
	const (
		left, right = 10.0, 287.0
		rowH        = 11.0
		bottom      = 190.0
	)
	cols := []struct {
		titel string
		w     float64
	}{{"Garten", 15}, {"Pächter", 46}, {"noch offen aus früheren Begehungen", 78}, {"in Ordnung", 20}, {"Anbau ca. %", 20}, {"Mangel festgestellt (Art, Beschreibung)", 98}}
	var y float64
	seite := func() {
		pdf.AddPage()
		pdf.SetFont("lib", "B", 15)
		pdf.SetTextColor(mr, mg, mb)
		pdf.SetXY(left, 11)
		pdf.CellFormat(right-left, 7, "Begehungsliste – "+s.VereinName, "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("lib", "", 10)
		pdf.SetXY(left, 19)
		pdf.CellFormat(right-left, 6, "Begehung am: ____________________      Begangen von: ______________________________      Eintragen im Programm: Bereich »Mängel«", "", 0, "L", false, 0, "")
		y = 28
		pdf.SetFillColor(fr, fg, fb)
		pdf.SetDrawColor(120, 120, 120)
		pdf.SetLineWidth(0.25)
		pdf.SetFont("lib", "B", 9)
		x := left
		for _, c := range cols {
			pdf.SetXY(x, y)
			pdf.CellFormat(c.w, 7.5, c.titel, "1", 0, "L", true, 0, "")
			x += c.w
		}
		y += 7.5
		pdf.SetFont("lib", "", 7.5)
		pdf.SetTextColor(90, 90, 90)
		pdf.SetXY(left, 199)
		pdf.CellFormat(right-left, 4, fmt.Sprintf("%s · Begehungsliste · Seite %d von {nb}", s.VereinName, pdf.PageNo()), "", 0, "R", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}
	seite()
	for _, z := range l {
		if y+rowH > bottom {
			seite()
		}
		offen := trim(strings.Join(z.Offen, "; "), 140)
		x := left
		for i, txt := range []string{z.Garten, z.Name, "", "", "", ""} {
			pdf.SetFont("lib", "", 9.5)
			pdf.SetXY(x, y)
			pdf.CellFormat(cols[i].w, rowH, passendKlein(pdf, txt, cols[i].w-2, 9.5, 6.5), "1", 0, "L", false, 0, "")
			x += cols[i].w
		}
		if offen != "" { // mehrere Zeilen in kleiner Schrift, damit auch längere Texte lesbar bleiben
			pdf.SetFont("lib", "", 8)
			pdf.SetXY(left+cols[0].w+cols[1].w+1, y+1)
			pdf.MultiCell(cols[2].w-2, 3.5, offen, "", "L", false)
		}
		if z.Anbau != "" { // letzte Schätzung klein als Hinweis, darunter ist Platz für die neue
			pdf.SetFont("lib", "", 6.5)
			pdf.SetTextColor(110, 110, 110)
			pdf.SetXY(left+cols[0].w+cols[1].w+cols[2].w+cols[3].w+1, y+0.8)
			pdf.CellFormat(cols[4].w-2, 3, z.Anbau+" %", "", 0, "L", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
		}
		// Kästchen »in Ordnung«
		pdf.SetDrawColor(120, 120, 120)
		pdf.Rect(left+cols[0].w+cols[1].w+cols[2].w+cols[3].w/2-2.5, y+rowH/2-2.5, 5, 5, "D")
		y += rowH
	}
	return pdfBytes(pdf)
}

func (a *App) begehungslistePDF(r *http.Request) ([]byte, string, error) {
	a.st.mu.Lock()
	l := a.st.begehungLocked()
	s := a.st.d.Settings
	a.st.mu.Unlock()
	pdf, err := buildBegehungsliste(s, l)
	return pdf, "Begehungsliste.pdf", err
}

// ------------------------------------------------------------------ Aufforderung an den Pächter

func buildMaengelschreiben(s Settings, p Paechter, ms []Mangel, frist string) ([]byte, error) {
	if len(ms) == 0 {
		return nil, bad("Für diesen Garten sind keine offenen Mängel eingetragen")
	}
	pdf := neuesPDF(s, "Aufforderung zur Beseitigung von Mängeln – Garten "+p.Gartennr, false)
	mr, mg, mb := markenfarbe(s.Farbe)
	y := briefAnfang(pdf, s, p, time.Now().Format("2006-01-02"))
	pdf.SetFont("lib", "B", 13)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 7, "Aufforderung zur Beseitigung von Mängeln", "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	y += 7
	pdf.SetFont("lib", "", 10)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 6, "Garten "+p.Gartennr, "", 0, "L", false, 0, "")
	y += 10
	pdf.SetXY(leftX, y)
	pdf.MultiCell(rightX-leftX, 5.2, "Liebe Gartenfreundin, lieber Gartenfreund,\n\nbei der Begehung der Gartenanlage wurden in Ihrem Garten die folgenden Mängel festgestellt:", "", "L", false)
	y = pdf.GetY() + 3
	for i, m := range ms {
		if y > 262 {
			pdf.AddPage()
			y = 20
		}
		pdf.SetFont("lib", "B", 10)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(8, 5.2, fmt.Sprintf("%d.", i+1), "", 0, "L", false, 0, "")
		pdf.SetXY(leftX+8, y)
		pdf.CellFormat(rightX-leftX-8, 5.2, fmt.Sprintf("%s – festgestellt am %s", m.Art, germanDate(m.Datum)), "", 0, "L", false, 0, "")
		y += 5.4
		pdf.SetFont("lib", "", 10)
		pdf.SetXY(leftX+8, y)
		pdf.MultiCell(rightX-leftX-8, 5.2, m.Beschreibung, "", "L", false)
		y = pdf.GetY() + 2.5
	}
	if y > 240 {
		pdf.AddPage()
		y = 20
	}
	y += 2
	pdf.SetFont("lib", "", 10)
	pdf.SetXY(leftX, y)
	text := fmt.Sprintf("Wir bitten Sie, diese Mängel bis zum %s zu beseitigen. Sind sie bereits behoben oder gibt es Gründe, warum das nicht möglich ist, melden Sie sich bitte beim Vorstand. "+
		"Wir bitten um Verständnis, dass wir nach Fristablauf weitere Schritte nach Pachtvertrag und Satzung prüfen müssen.", germanDate(frist))
	pdf.MultiCell(rightX-leftX, 5.2, text, "", "L", false)
	y = pdf.GetY() + 8
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 6, "Mit freundlichen Gartengrüßen", "", 0, "L", false, 0, "")
	y += 6
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 6, "Der Vorstand", "", 0, "L", false, 0, "")
	return pdfBytes(pdf)
}

// briefAnfang beginnt eine neue Seite mit Briefkopf, Absenderzeile, Anschrift des Pächters und Datum
// (Fensterumschlag DIN lang). Liefert die Höhe, ab der der Brieftext beginnt.
func briefAnfang(pdf *fpdf.Fpdf, s Settings, p Paechter, datum string) float64 {
	pdf.AddPage()
	zeichneKopf(pdf, s)
	y := 31.0
	pdf.SetFont("lib", "", 7.5)
	pdf.SetTextColor(60, 60, 60)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, rowH, s.Absenderzeile, "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	y += 6
	pdf.SetFont("lib", "B", 10)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, rowH, p.Versand, "", 0, "R", false, 0, "")
	y += 5
	y += adressZeile(pdf, leftX, y, p.Anrede, "")
	y += adressZeile(pdf, leftX, y, p.Name, "B")
	y += adressZeile(pdf, leftX, y, p.Strasse, "")
	y += adressZeile(pdf, leftX, y, p.PLZOrt, "")
	y += 10
	pdf.SetFont("lib", "", 10)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, rowH, "Datum: "+germanDate(datum), "", 0, "R", false, 0, "")
	y += 8
	return y
}

func (a *App) maengelschreibenPDF(r *http.Request) ([]byte, string, error) {
	nr := strings.TrimSpace(r.URL.Query().Get("gartennr"))
	if nr == "" {
		return nil, "", bad("Bitte die Gartennummer angeben")
	}
	a.st.mu.Lock()
	p, ok := a.st.aktiverPaechterLocked(nr)
	var ms []Mangel
	s := a.st.d.Settings
	if ok {
		for _, m := range a.st.d.Maengel {
			if m.ErledigtAm == "" && m.Gartennr == nr {
				ms = append(ms, m)
			}
		}
	}
	a.st.mu.Unlock()
	if !ok {
		return nil, "", bad(fmt.Sprintf("Für Garten %s ist kein Pächter eingetragen", nr))
	}
	// Frist: gewünschtes Datum, sonst die früheste eingetragene Frist, sonst 14 Tage ab heute
	frist := strings.TrimSpace(r.URL.Query().Get("frist"))
	if _, err := parseDate(frist); err != nil {
		frist = ""
	}
	if frist == "" {
		for _, m := range ms {
			if m.Frist != "" && (frist == "" || m.Frist < frist) {
				frist = m.Frist
			}
		}
	}
	if frist == "" {
		frist = time.Now().AddDate(0, 0, 14).Format("2006-01-02")
	}
	sort.SliceStable(ms, func(i, j int) bool { return ms[i].Datum < ms[j].Datum })
	pdf, err := buildMaengelschreiben(s, p, ms, frist)
	return pdf, safeName("Maengel_Garten_"+nr, 80) + ".pdf", err
}
