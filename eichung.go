package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// Eichfristen: Wasser- und Stromzähler dürfen nach dem Mess- und Eichgesetz nur eine begrenzte Zeit genutzt
// werden (Kaltwasserzähler 6 Jahre, elektronische Stromzähler 8 Jahre, Ferraris-Zähler 16 Jahre). Die Frist
// endet mit Ablauf des Kalenderjahres: Ein 2020 geeichter Wasserzähler darf bis 31.12.2026 messen.

type Eichfristen struct {
	Wasser int `json:"wasser"`
	Strom  int `json:"strom"`
}

func (e Eichfristen) mitStandard() Eichfristen {
	if e.Wasser <= 0 {
		e.Wasser = 6
	}
	if e.Strom <= 0 {
		e.Strom = 8
	}
	return e
}

type eichZeile struct {
	PaechterID string `json:"paechterId"`
	Gartennr   string `json:"gartennr"`
	Name       string `json:"name"`
	Art        string `json:"art"` // wasser | strom
	Nr         string `json:"nr"`
	Eichjahr   int    `json:"eichjahr,omitempty"`
	GueltigBis int    `json:"gueltigBis,omitempty"`
	// Status: abgelaufen, faellig (Frist endet dieses Jahr), bald (nächstes Jahr), ok, unbekannt (Eichjahr fehlt)
	Status string `json:"status"`
}

type eichAntwort struct {
	Jahr    int         `json:"jahr"`
	Fristen Eichfristen `json:"fristen"`
	Zeilen  []eichZeile `json:"zeilen"`
}

func eichStatus(eichjahr, frist, jahr int) (int, string) {
	if eichjahr <= 0 {
		return 0, "unbekannt"
	}
	bis := eichjahr + frist
	switch {
	case jahr > bis:
		return bis, "abgelaufen"
	case jahr == bis:
		return bis, "faellig"
	case jahr+1 == bis:
		return bis, "bald"
	}
	return bis, "ok"
}

// eichungLocked: alle Zähler der aktiven Pächter, dringendste zuerst. Der Aufrufer hält s.mu.
func (s *Store) eichungLocked(jahr int) []eichZeile {
	f := s.d.Eichfristen.mitStandard()
	out := []eichZeile{}
	for _, p := range s.d.Paechter {
		if p.Geloescht || p.AusgeschiedenAm != "" {
			continue
		}
		for _, z := range []struct {
			art, nr string
			jahr    int
			frist   int
		}{{"wasser", p.WasserzaehlerNr, p.WasserEichjahr, f.Wasser}, {"strom", p.StromzaehlerNr, p.StromEichjahr, f.Strom}} {
			if strings.TrimSpace(z.nr) == "" {
				continue
			}
			bis, st := eichStatus(z.jahr, z.frist, jahr)
			out = append(out, eichZeile{p.ID, p.Gartennr, p.Name, z.art, z.nr, z.jahr, bis, st})
		}
	}
	rang := map[string]int{"abgelaufen": 0, "faellig": 1, "bald": 2, "unbekannt": 3, "ok": 4}
	sort.SliceStable(out, func(i, j int) bool {
		if rang[out[i].Status] != rang[out[j].Status] {
			return rang[out[i].Status] < rang[out[j].Status]
		}
		if out[i].Gartennr != out[j].Gartennr {
			return naturalLess(out[i].Gartennr, out[j].Gartennr)
		}
		return out[i].Art > out[j].Art // Wasser vor Strom
	})
	return out
}

// eichjahrUebernehmenLocked: Zähler gehören zum Garten, nicht zur Person. Bekommt ein Pächter (Wechsel, Vergabe,
// Import) einen Zähler, dessen Nummer schon bei einem anderen – auch gelöschten – Pächter mit Eichjahr steht,
// wird das Eichjahr übernommen. Der Aufrufer hält s.mu.
func (s *Store) eichjahrUebernehmenLocked(p *Paechter) {
	for _, q := range s.d.Paechter {
		if q.ID == p.ID {
			continue
		}
		if p.WasserEichjahr == 0 && q.WasserEichjahr > 0 && p.WasserzaehlerNr != "" && q.WasserzaehlerNr == p.WasserzaehlerNr {
			p.WasserEichjahr = q.WasserEichjahr
		}
		if p.StromEichjahr == 0 && q.StromEichjahr > 0 && p.StromzaehlerNr != "" && q.StromzaehlerNr == p.StromzaehlerNr {
			p.StromEichjahr = q.StromEichjahr
		}
	}
}

func (a *App) handleEichung(w http.ResponseWriter, r *http.Request) {
	jahr := time.Now().Year()
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	writeJSON(w, 200, eichAntwort{Jahr: jahr, Fristen: a.st.d.Eichfristen.mitStandard(), Zeilen: a.st.eichungLocked(jahr)})
}

func (a *App) handleEichfristen(w http.ResponseWriter, r *http.Request) {
	var in Eichfristen
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if in.Wasser < 1 || in.Wasser > 30 || in.Strom < 1 || in.Strom > 30 {
		writeErr(w, bad("Die Eichfrist muss zwischen 1 und 30 Jahren liegen"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	a.st.d.Eichfristen = in
	a.st.audit("Eichfristen geändert: Wasser %d, Strom %d Jahre", in.Wasser, in.Strom)
	a.speichernUndAntworten(w, eichAntwort{Jahr: time.Now().Year(), Fristen: in, Zeilen: a.st.eichungLocked(time.Now().Year())})
}

// handleEichjahr setzt das Eich- bzw. Einbaujahr eines Zählers (0 = unbekannt).
func (a *App) handleEichjahr(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Art      string `json:"art"`
		Eichjahr int    `json:"eichjahr"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	jahr := time.Now().Year()
	if in.Eichjahr != 0 && (in.Eichjahr < 1950 || in.Eichjahr > jahr+1) || (in.Art != "wasser" && in.Art != "strom") {
		writeErr(w, bad("Bitte ein gültiges Eichjahr eintragen"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Paechter {
		p := &a.st.d.Paechter[i]
		if p.ID != r.PathValue("id") || p.Geloescht {
			continue
		}
		if in.Art == "wasser" {
			p.WasserEichjahr = in.Eichjahr
		} else {
			p.StromEichjahr = in.Eichjahr
		}
		a.speichernUndAntworten(w, eichAntwort{Jahr: jahr, Fristen: a.st.d.Eichfristen.mitStandard(), Zeilen: a.st.eichungLocked(jahr)})
		return
	}
	writeErr(w, notFound("Pächter nicht gefunden"))
}

// tauschlistePDF: Liste für den Wasser-/Elektrowart mit allen Zählern, deren Eichfrist dieses oder nächstes
// Jahr endet oder schon abgelaufen ist, und Feldern zum Eintragen der Stände beim Tausch.
func (a *App) tauschlistePDF(r *http.Request) ([]byte, string, error) {
	jahr := time.Now().Year()
	a.st.mu.Lock()
	s := a.st.d.Settings
	var l []eichZeile
	for _, z := range a.st.eichungLocked(jahr) {
		if z.Status == "abgelaufen" || z.Status == "faellig" || z.Status == "bald" {
			l = append(l, z)
		}
	}
	a.st.mu.Unlock()
	if len(l) == 0 {
		return nil, "", bad("Kein Zähler muss in diesem oder im nächsten Jahr getauscht werden")
	}
	titel := fmt.Sprintf("Zählertausch %d/%d", jahr, jahr+1)
	pdf := neuesPDF(s, titel, true)
	kopf := []string{"Garten", "Pächter", "Zähler", "Nummer", "Gültig bis", "Stand alt", "Neue Nummer", "Stand neu", "Datum"}
	breiten := []float64{18, 52, 18, 34, 22, 26, 38, 26, 23}
	zeilen := make([][]string, 0, len(l))
	for _, z := range l {
		art := "Wasser"
		if z.Art == "strom" {
			art = "Strom"
		}
		bis := fmt.Sprintf("%d", z.GueltigBis)
		if z.Status == "abgelaufen" {
			bis += " (!)"
		}
		zeilen = append(zeilen, []string{z.Gartennr, z.Name, art, z.Nr, bis, "", "", "", ""})
	}
	pdfListe(pdf, s, titel, "Eichfrist endet mit Ablauf des genannten Jahres. Nach dem Tausch den Zählerwechsel im Programm eintragen – das Eichjahr wird dann automatisch gesetzt.", kopf, breiten, zeilen, 9)
	out, err := pdfBytes(pdf)
	return out, fmt.Sprintf("Zaehlertausch-%d.pdf", jahr), err
}

// eichungAufgabenLocked: Hinweis im Cockpit, wenn Zähler getauscht werden müssen.
func (s *Store) eichungAufgabenLocked(heute time.Time, add func(stufe, titel, text, ziel, knopf string)) {
	ab, faellig, bald := 0, 0, 0
	for _, z := range s.eichungLocked(heute.Year()) {
		switch z.Status {
		case "abgelaufen":
			ab++
		case "faellig":
			faellig++
		case "bald":
			bald++
		}
	}
	switch {
	case ab > 0:
		add("warn", fmt.Sprintf("%d Zähler mit abgelaufener Eichfrist", ab),
			"Diese Zähler dürfen nicht mehr abgerechnet werden. Bitte tauschen lassen – die Tauschliste hilft dabei.", "eingabe:eichung", "Eichfristen")
	case faellig > 0:
		add("warn", fmt.Sprintf("%d Zähler müssen bis 31.12.%d getauscht werden", faellig, heute.Year()),
			"Ihre Eichfrist endet in diesem Jahr. Am besten gleich zur nächsten Ablesung tauschen.", "eingabe:eichung", "Eichfristen")
	case bald > 0 && heute.Month() >= time.September:
		add("info", fmt.Sprintf("%d Zähler müssen nächstes Jahr getauscht werden", bald),
			"Jetzt schon Zähler bestellen und den Tausch für das Frühjahr einplanen.", "eingabe:eichung", "Eichfristen")
	}
}

// pdfListe setzt eine einfache Tabelle mit Kopfzeile, Seitenumbruch und Fußzeile.
func pdfListe(pdf *fpdf.Fpdf, s Settings, titel, hinweis string, kopf []string, breiten []float64, zeilen [][]string, groesse float64) {
	_, hoehe := pdf.GetPageSize()
	unten := hoehe - 18
	breite := 0.0
	for _, b := range breiten {
		breite += b
	}
	zeileH := groesse * 0.75
	seite := func() float64 {
		pdf.AddPage()
		zeichneKopf(pdf, s)
		y := mappeAbschnitt(pdf, s, 36, titel)
		if hinweis != "" {
			pdf.SetFont("lib", "", 8.5)
			pdf.SetTextColor(90, 90, 90)
			pdf.SetXY(leftX, y-2)
			pdf.MultiCell(breite, 4.2, hinweis, "", "L", false)
			pdf.SetTextColor(0, 0, 0)
			y = pdf.GetY() + 2
		}
		pdf.SetFont("lib", "B", groesse)
		pdf.SetFillColor(238, 241, 236)
		x := leftX
		for i, k := range kopf {
			pdf.SetXY(x, y)
			pdf.CellFormat(breiten[i], zeileH+1, k, "B", 0, "L", true, 0, "")
			x += breiten[i]
		}
		return y + zeileH + 1
	}
	fuss := func() { mappeFuss(pdf, s, s.VereinName+" · "+titel+" · Stand "+time.Now().Format("02.01.2006")) }
	y := seite()
	pdf.SetFont("lib", "", groesse)
	for _, z := range zeilen {
		if y+zeileH+1.5 > unten {
			fuss()
			y = seite()
			pdf.SetFont("lib", "", groesse)
		}
		x := leftX
		for i, c := range z {
			pdf.SetXY(x, y)
			pdf.CellFormat(breiten[i], zeileH+1.5, passendKlein(pdf, c, breiten[i]-1, groesse, groesse-2), "B", 0, "L", false, 0, "")
			pdf.SetFont("lib", "", groesse)
			x += breiten[i]
		}
		y += zeileH + 1.5
	}
	fuss()
}
