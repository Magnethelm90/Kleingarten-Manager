package main

import (
	"fmt"
	"math"
	"net/http"
	"sort"
)

// Beitragsübersicht je Pächter: alle Rechnungen mit Zahlungsstand und Mahnungen über die Jahre sowie die
// Vorauszahlungen. Es wird nur gelesen; die Zahlen stammen aus den vorhandenen Rechnungen und Zahlungen.

type kontoZeile struct {
	Jahr      int     `json:"jahr"`
	Nummer    string  `json:"nummer"`
	Datum     string  `json:"datum"`
	Faellig   string  `json:"faellig"`
	Betrag    float64 `json:"betrag"` // positiv = Forderung, negativ = Guthaben des Pächters
	Bezahlt   float64 `json:"bezahlt"`
	BezahltAm string  `json:"bezahltAm"`
	Offen     float64 `json:"offen"` // mit Vorzeichen wie Betrag
	Mahnstufe int     `json:"mahnstufe"`
}

type kontoVZ struct {
	Jahr    int     `json:"jahr"`
	Summe   float64 `json:"summe"`
	Anzahl  int     `json:"anzahl"`
	Gebucht bool    `json:"gebucht"`
}

type beitragskonto struct {
	Name          string       `json:"name"`
	Gartennr      string       `json:"gartennr"`
	Mitgliedsnr   string       `json:"mitgliedsnr"`
	Zeilen        []kontoZeile `json:"zeilen"`
	Vorauszahlung []kontoVZ    `json:"vorauszahlungen"`
	Forderung     float64      `json:"forderungOffen"`
	Guthaben      float64      `json:"guthabenOffen"`
	Saldo         float64      `json:"saldo"` // positiv: der Pächter schuldet dem Verein noch etwas
}

func runde2(x float64) float64 { return math.Round(x*100) / 100 }

// beitragskontoLocked stellt das Konto zusammen. Der Aufrufer hält s.mu.
func (s *Store) beitragskontoLocked(id string) (beitragskonto, bool) {
	var k beitragskonto
	gefunden := false
	for _, p := range s.d.Paechter {
		if p.ID == id {
			k.Name, k.Gartennr, k.Mitgliedsnr, gefunden = p.Name, p.Gartennr, p.Mitgliedsnr, true
		}
	}
	if !gefunden {
		return k, false
	}
	for _, r := range s.d.Rechnungen {
		if r.PaechterID != id || r.Status != "gueltig" {
			continue
		}
		betrag := r.Result.Gesamt
		gesamt := math.Abs(betrag)
		bezahlt := 0.0
		if r.BezahltAm != "" {
			bezahlt = gesamt
			if r.BezahltBetrag != nil {
				bezahlt = *r.BezahltBetrag
			}
		}
		offen := runde2(gesamt - bezahlt)
		if offen < 0 {
			offen = 0
		}
		z := kontoZeile{Jahr: r.Jahr, Nummer: r.Nummer, Datum: r.Ausgestellt, Faellig: r.Settings.Zahlungsziel,
			Betrag: runde2(betrag), Bezahlt: runde2(bezahlt), BezahltAm: r.BezahltAm, Mahnstufe: len(r.Mahnungen)}
		if betrag < 0 {
			z.Offen = -offen
			k.Guthaben += offen
		} else {
			z.Offen = offen
			k.Forderung += offen
		}
		k.Zeilen = append(k.Zeilen, z)
	}
	sort.SliceStable(k.Zeilen, func(i, j int) bool {
		if k.Zeilen[i].Jahr != k.Zeilen[j].Jahr {
			return k.Zeilen[i].Jahr > k.Zeilen[j].Jahr
		}
		return k.Zeilen[i].Nummer > k.Zeilen[j].Nummer
	})
	je := map[int]*kontoVZ{}
	for _, v := range s.d.Vorauszahlungen {
		if v.PaechterID != id {
			continue
		}
		e := je[v.Jahr]
		if e == nil {
			e = &kontoVZ{Jahr: v.Jahr, Gebucht: true}
			je[v.Jahr] = e
		}
		e.Summe += v.Betrag
		e.Anzahl++
		e.Gebucht = e.Gebucht && v.Gebucht
	}
	for _, e := range je {
		e.Summe = runde2(e.Summe)
		k.Vorauszahlung = append(k.Vorauszahlung, *e)
	}
	sort.Slice(k.Vorauszahlung, func(i, j int) bool { return k.Vorauszahlung[i].Jahr > k.Vorauszahlung[j].Jahr })
	k.Forderung, k.Guthaben = runde2(k.Forderung), runde2(k.Guthaben)
	k.Saldo = runde2(k.Forderung - k.Guthaben)
	if k.Zeilen == nil {
		k.Zeilen = []kontoZeile{}
	}
	if k.Vorauszahlung == nil {
		k.Vorauszahlung = []kontoVZ{}
	}
	return k, true
}

func (a *App) handleBeitragskonto(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	k, ok := a.st.beitragskontoLocked(r.PathValue("id"))
	a.st.mu.Unlock()
	if !ok {
		writeErr(w, notFound("Pächter nicht gefunden"))
		return
	}
	writeJSON(w, 200, k)
}

func (a *App) beitragskontoPDF(r *http.Request) ([]byte, string, error) {
	a.st.mu.Lock()
	k, ok := a.st.beitragskontoLocked(r.PathValue("id"))
	s := a.st.d.Settings
	a.st.mu.Unlock()
	if !ok {
		return nil, "", notFound("Pächter nicht gefunden")
	}
	titel := "Beitragsübersicht " + k.Name
	pdf := neuesPDF(s, titel, true)
	kopf := []string{"Jahr", "Rechnung", "Datum", "Fällig", "Betrag", "Bezahlt", "Bezahlt am", "Offen", "Mahnungen"}
	breiten := []float64{16, 36, 26, 26, 28, 28, 28, 28, 24}
	zeilen := make([][]string, 0, len(k.Zeilen)+1)
	for _, z := range k.Zeilen {
		zeilen = append(zeilen, []string{fmt.Sprint(z.Jahr), z.Nummer, germanDate(z.Datum), germanDate(z.Faellig), fmtEUR(z.Betrag), fmtEUR(z.Bezahlt), germanDate(z.BezahltAm), fmtEUR(z.Offen), fmt.Sprint(z.Mahnstufe)})
	}
	zeilen = append(zeilen, []string{"", "Saldo", "", "", "", "", "", fmtEUR(k.Saldo), ""})
	hinweis := fmt.Sprintf("Garten %s · Mitgliedsnr. %s. Positiver Saldo: noch zu zahlen; negativer Saldo: Guthaben des Pächters.", k.Gartennr, k.Mitgliedsnr)
	pdfListe(pdf, s, titel, hinweis, kopf, breiten, zeilen, 9.5)
	out, err := pdfBytes(pdf)
	return out, "Beitragsuebersicht.pdf", err
}
