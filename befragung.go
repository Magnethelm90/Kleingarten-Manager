package main

import (
	"fmt"
	"net/http"
	"strings"
)

// Mitgliederbefragung: Fragebogen zum Ausdrucken, Erfassungsbogen zum Auszählen und anonyme Auswertung.
// Gespeichert werden nur Fragen und Strichlisten-Summen – keine Namen und keine Einzelbögen. Freitext-Fragen
// erscheinen nur auf dem Fragebogen; der Vorstand fasst die Antworten in einer Anmerkung zusammen.

const (
	maxBefragungen = 40
	maxFragen      = 30
	maxOptionen    = 8
)

var befragungsArten = map[string]string{"janein": "Ja / Nein", "skala": "Skala 1–5", "wahl": "Auswahl", "text": "Freitext"}

type BefragungFrage struct {
	Text      string   `json:"text"`
	Art       string   `json:"art"` // janein, skala, wahl, text
	Optionen  []string `json:"optionen,omitempty"`
	Zaehlung  []int    `json:"zaehlung,omitempty"`
	Anmerkung string   `json:"anmerkung,omitempty"`
}

type Befragung struct {
	ID         string           `json:"id"`
	Titel      string           `json:"titel"`
	Datum      string           `json:"datum"` // Stichtag/Abgabetermin, JJJJ-MM-TT
	Einleitung string           `json:"einleitung,omitempty"`
	Fragen     []BefragungFrage `json:"fragen"`
	Rueckgabe  int              `json:"rueckgabe"` // Zahl der ausgefüllten Bögen
}

// optionenVon liefert die Antwortmöglichkeiten einer Frage (bei Ja/Nein und Skala fest).
func optionenVon(f BefragungFrage) []string {
	switch f.Art {
	case "janein":
		return []string{"Ja", "Nein", "Enthaltung"}
	case "skala":
		return []string{"1 – trifft nicht zu", "2", "3", "4", "5 – trifft voll zu"}
	case "wahl":
		return f.Optionen
	}
	return nil
}

func (f BefragungFrage) summe() int {
	n := 0
	for _, z := range f.Zaehlung {
		n += z
	}
	return n
}

func (a *App) handleBefragungen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	out := append([]Befragung{}, a.st.d.Befragungen...)
	writeJSON(w, 200, out)
}

func (a *App) handleBefragungSpeichern(w http.ResponseWriter, r *http.Request) {
	var in Befragung
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in.Titel, in.Einleitung = trim(in.Titel, 120), trim(in.Einleitung, 1500)
	if in.Titel == "" {
		writeErr(w, bad("Bitte einen Titel für die Befragung eintragen"))
		return
	}
	if !gueltigesDatum(in.Datum, true) || in.Rueckgabe < 0 || in.Rueckgabe > 100000 {
		writeErr(w, bad("Bitte Datum und Zahl der Bögen prüfen"))
		return
	}
	if len(in.Fragen) == 0 || len(in.Fragen) > maxFragen {
		writeErr(w, bad(fmt.Sprintf("Bitte 1 bis %d Fragen eintragen", maxFragen)))
		return
	}
	fragen := make([]BefragungFrage, 0, len(in.Fragen))
	for i, f := range in.Fragen {
		f.Text, f.Anmerkung = trim(f.Text, 300), trim(f.Anmerkung, 1500)
		if f.Text == "" {
			writeErr(w, bad(fmt.Sprintf("Frage %d hat keinen Text", i+1)))
			return
		}
		if _, ok := befragungsArten[f.Art]; !ok {
			writeErr(w, bad(fmt.Sprintf("Frage %d: unbekannte Art", i+1)))
			return
		}
		var opt []string
		for _, o := range f.Optionen {
			if o = trim(o, 80); o != "" {
				opt = append(opt, o)
			}
		}
		f.Optionen = nil
		if f.Art == "wahl" {
			if len(opt) < 2 || len(opt) > maxOptionen {
				writeErr(w, bad(fmt.Sprintf("Frage %d: bitte 2 bis %d Antworten angeben", i+1, maxOptionen)))
				return
			}
			f.Optionen = opt
		}
		n := len(optionenVon(f))
		z := make([]int, n)
		sum := 0
		for k := 0; k < n && k < len(f.Zaehlung); k++ {
			if f.Zaehlung[k] < 0 || f.Zaehlung[k] > 100000 {
				writeErr(w, bad(fmt.Sprintf("Frage %d: ungültige Zahl", i+1)))
				return
			}
			z[k] = f.Zaehlung[k]
			sum += z[k]
		}
		if n == 0 {
			z = nil
		}
		if sum > in.Rueckgabe {
			writeErr(w, bad(fmt.Sprintf("Frage %d: mehr Antworten (%d) als ausgefüllte Bögen (%d)", i+1, sum, in.Rueckgabe)))
			return
		}
		f.Zaehlung = z
		fragen = append(fragen, f)
	}
	in.Fragen = fragen
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Befragungen {
		if in.ID != "" && a.st.d.Befragungen[i].ID == in.ID {
			a.st.d.Befragungen[i] = in
			a.st.audit("Befragung geändert: %s", in.Titel)
			a.speichernUndAntworten(w, in)
			return
		}
	}
	if len(a.st.d.Befragungen) >= maxBefragungen {
		writeErr(w, bad("Zu viele Befragungen – bitte alte löschen"))
		return
	}
	in.ID = newID()
	a.st.d.Befragungen = append(a.st.d.Befragungen, in)
	a.st.audit("Befragung angelegt: %s", in.Titel)
	a.speichernUndAntworten(w, in)
}

func (a *App) handleBefragungLoeschen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i, b := range a.st.d.Befragungen {
		if b.ID == r.PathValue("id") {
			a.st.d.Befragungen = append(a.st.d.Befragungen[:i], a.st.d.Befragungen[i+1:]...)
			a.st.audit("Befragung gelöscht: %s", b.Titel)
			a.speichernUndAntworten(w, map[string]any{"ok": true})
			return
		}
	}
	writeErr(w, notFound("Befragung nicht gefunden"))
}

func (s *Store) befragungLocked(id string) (Befragung, bool) {
	for _, b := range s.d.Befragungen {
		if b.ID == id {
			return b, true
		}
	}
	return Befragung{}, false
}

// befragungPDF: ?art=fragebogen (zum Ausfüllen), erfassung (Strichliste) oder auswertung (Ergebnis ohne Namen).
func (a *App) befragungPDF(r *http.Request) ([]byte, string, error) {
	art := r.URL.Query().Get("art")
	if art != "fragebogen" && art != "erfassung" && art != "auswertung" {
		return nil, "", bad("Unbekannte Art")
	}
	a.st.mu.Lock()
	b, ok := a.st.befragungLocked(r.PathValue("id"))
	s := a.st.d.Settings
	a.st.mu.Unlock()
	if !ok {
		return nil, "", notFound("Befragung nicht gefunden")
	}
	pdf, err := buildBefragung(s, b, art)
	return pdf, safeName("Befragung_"+art+"_"+b.Titel, 80) + ".pdf", err
}

func buildBefragung(s Settings, b Befragung, art string) ([]byte, error) {
	pdf := neuesPDF(s, "Befragung – "+b.Titel, false)
	mr, mg, mb := markenfarbe(s.Farbe)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	y := 34.0
	neueSeite := func(bedarf float64) {
		if y+bedarf > 272 {
			pdf.AddPage()
			y = 20
		}
	}
	text := func(font string, size float64, hoehe float64, t string) {
		pdf.SetFont("lib", font, size)
		pdf.SetXY(leftX, y)
		pdf.MultiCell(rightX-leftX, hoehe, t, "", "L", false)
		y = pdf.GetY()
	}
	titel := map[string]string{"fragebogen": b.Titel, "erfassung": "Erfassungsbogen: " + b.Titel, "auswertung": "Auswertung: " + b.Titel}[art]
	pdf.SetFont("lib", "B", 15)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(leftX, y)
	pdf.MultiCell(rightX-leftX, 7.5, titel, "", "L", false)
	y = pdf.GetY() + 2
	pdf.SetTextColor(0, 0, 0)
	switch art {
	case "fragebogen":
		if b.Einleitung != "" {
			text("", 10.5, 5.4, b.Einleitung)
			y += 2
		}
		hinweis := "Die Befragung ist anonym. Bitte keinen Namen eintragen."
		if b.Datum != "" {
			hinweis += " Abgabe bis " + germanDate(b.Datum) + "."
		}
		pdf.SetTextColor(90, 90, 90)
		text("", 9, 4.6, hinweis)
		pdf.SetTextColor(0, 0, 0)
		y += 5
		for i, f := range b.Fragen {
			opt := optionenVon(f)
			bedarf := 12.0 + 6.5*float64(len(opt))
			if f.Art == "text" {
				bedarf = 40
			}
			neueSeite(bedarf)
			text("B", 10.5, 5.4, fmt.Sprintf("%d. %s", i+1, f.Text))
			y += 2
			if f.Art == "text" {
				pdf.SetDrawColor(170, 170, 170)
				for k := 0; k < 3; k++ {
					y += 7
					pdf.Line(leftX, y, rightX, y)
				}
				y += 6
				continue
			}
			if f.Art == "skala" || f.Art == "janein" {
				// Antworten in einer Reihe.
				bw := (rightX - leftX) / float64(len(opt))
				for k, o := range opt {
					x := leftX + bw*float64(k)
					pdf.SetDrawColor(60, 60, 60)
					pdf.Rect(x, y+0.8, 4, 4, "D")
					pdf.SetFont("lib", "", 9)
					pdf.SetXY(x+5.5, y)
					pdf.MultiCell(bw-6, 4.4, o, "", "L", false)
				}
				if f.Art == "skala" {
					y += 11
				} else {
					y += 8
				}
				continue
			}
			for _, o := range opt {
				pdf.SetDrawColor(60, 60, 60)
				pdf.Rect(leftX+2, y+0.8, 4, 4, "D")
				pdf.SetFont("lib", "", 10)
				pdf.SetXY(leftX+9, y)
				pdf.MultiCell(rightX-leftX-9, 5.2, o, "", "L", false)
				y = pdf.GetY() + 1.3
			}
			y += 4
		}
		neueSeite(14)
		pdf.SetTextColor(90, 90, 90)
		y += 4
		text("", 9, 4.6, "Vielen Dank für die Teilnahme! Den ausgefüllten Bogen bitte in den Briefkasten des Vereins werfen oder beim Vorstand abgeben.")
	case "erfassung":
		text("", 9.5, 4.8, "Pro ausgefülltem Bogen je Frage einen Strich in die Zeile der gewählten Antwort. Danach die Striche zählen und die Summe eintragen. Die Zahlen werden im Programm unter Berichte → Mitgliederbefragung eingegeben.")
		y += 3
		pdf.SetFont("lib", "B", 10.5)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(60, 8, "Anzahl ausgefüllter Bögen:", "", 0, "L", false, 0, "")
		pdf.SetDrawColor(120, 120, 120)
		pdf.Rect(leftX+58, y, 24, 8, "D")
		y += 14
		for i, f := range b.Fragen {
			opt := optionenVon(f)
			if len(opt) == 0 {
				continue
			}
			neueSeite(10 + 9*float64(len(opt)))
			text("B", 10, 5, fmt.Sprintf("%d. %s", i+1, f.Text))
			y += 1.5
			for _, o := range opt {
				pdf.SetDrawColor(150, 150, 150)
				pdf.SetFont("lib", "", 9.5)
				pdf.SetXY(leftX, y)
				pdf.CellFormat(46, 8, trim(o, 28), "1", 0, "L", false, 0, "")
				pdf.CellFormat(100, 8, "", "1", 0, "L", false, 0, "")
				pdf.CellFormat(24, 8, "", "1", 0, "L", false, 0, "")
				y += 8
			}
			pdf.SetFont("lib", "", 7.5)
			pdf.SetTextColor(100, 100, 100)
			pdf.SetXY(leftX+146, y)
			pdf.CellFormat(24, 4, "Summe", "", 0, "C", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
			y += 8
		}
	default:
		pdf.SetFont("lib", "", 9.5)
		pdf.SetTextColor(90, 90, 90)
		ueber := fmt.Sprintf("%d ausgefüllte Bögen", b.Rueckgabe)
		if b.Datum != "" {
			ueber += " · Stand " + germanDate(b.Datum)
		}
		text("", 9.5, 4.8, ueber+" · anonym, ohne Namen")
		pdf.SetTextColor(0, 0, 0)
		y += 4
		for i, f := range b.Fragen {
			opt := optionenVon(f)
			neueSeite(14 + 7*float64(len(opt)))
			text("B", 10.5, 5.4, fmt.Sprintf("%d. %s", i+1, f.Text))
			y += 2
			sum := f.summe()
			for k, o := range opt {
				z := 0
				if k < len(f.Zaehlung) {
					z = f.Zaehlung[k]
				}
				anteil := 0.0
				if sum > 0 {
					anteil = float64(z) / float64(sum)
				}
				pdf.SetFont("lib", "", 9.5)
				pdf.SetXY(leftX, y)
				pdf.CellFormat(50, 6, trim(o, 30), "", 0, "L", false, 0, "")
				pdf.SetFillColor(mr, mg, mb)
				pdf.Rect(leftX+52, y+1.2, 80*anteil, 3.6, "F")
				pdf.SetDrawColor(200, 200, 200)
				pdf.Rect(leftX+52, y+1.2, 80, 3.6, "D")
				pdf.SetXY(leftX+135, y)
				pdf.CellFormat(35, 6, fmt.Sprintf("%d (%.0f %%)", z, anteil*100), "", 0, "R", false, 0, "")
				y += 6.5
			}
			if f.Art == "text" {
				pdf.SetTextColor(90, 90, 90)
				text("", 9.5, 4.8, "Freitext – siehe Zusammenfassung")
				pdf.SetTextColor(0, 0, 0)
			} else if sum < b.Rueckgabe {
				pdf.SetTextColor(100, 100, 100)
				text("", 8.5, 4.2, fmt.Sprintf("%d Bögen ohne Antwort auf diese Frage", b.Rueckgabe-sum))
				pdf.SetTextColor(0, 0, 0)
			}
			if f.Anmerkung != "" {
				y += 1
				text("", 9.5, 4.8, "Zusammenfassung: "+f.Anmerkung)
			}
			y += 5
		}
	}
	return pdfBytes(pdf)
}

// befragungenImJahr liefert für das Versammlungspaket die Auswertungen des Jahres (nur Befragungen mit Bögen).
func (a *App) befragungenImJahr(jahr int) []Befragung {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	var out []Befragung
	for _, b := range a.st.d.Befragungen {
		if b.Rueckgabe > 0 && (b.Datum == "" || strings.HasPrefix(b.Datum, yearKey(jahr))) {
			out = append(out, b)
		}
	}
	return out
}
