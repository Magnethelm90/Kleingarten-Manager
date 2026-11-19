package main

import (
	"fmt"
	"net/http"
	"sort"
)

// Jahresabrechnung als eine PDF-Datei: vorn eine Übersicht (Garten, Name, Rechnungsnummer, Betrag, bezahlt/offen,
// Summen), danach alle gültigen Rechnungen des Jahres, je Pächter eine Seite. Zum Abheften oder Ausdrucken in einem Rutsch.
// Das Programm liest nur vorhandene Rechnungen, es wird nichts ausgestellt oder verändert.

func (a *App) jahresabrechnungPDF(r *http.Request) ([]byte, string, error) {
	jahr := a.yearParam(r)
	a.st.mu.Lock()
	var recs []Rechnung
	for _, x := range a.st.d.Rechnungen {
		if x.Jahr == jahr && x.Status == statusGueltig && !x.Bereinigt {
			recs = append(recs, *x)
		}
	}
	s := a.st.d.Settings
	a.st.mu.Unlock()
	if len(recs) == 0 {
		return nil, "", notFound(fmt.Sprintf("Für %d gibt es noch keine ausgestellten Rechnungen.", jahr))
	}
	sort.SliceStable(recs, func(i, j int) bool { return naturalLess(recs[i].Paechter.Mitgliedsnr, recs[j].Paechter.Mitgliedsnr) })
	if recs[0].Settings.VereinName != "" {
		s = recs[0].Settings
	}

	var summe, bezahlt, offen float64
	zeilen := make([][]string, 0, len(recs))
	for i := range recs {
		rec := &recs[i]
		betrag := zuZahlen(rec)
		summe += betrag
		status := "offen"
		if rec.BezahltAm != "" && openAmount(rec) <= 0.004 {
			status = "bezahlt"
			bezahlt += betrag
		} else {
			offen += openAmount(rec)
		}
		zeilen = append(zeilen, []string{rec.Paechter.Gartennr, rec.Paechter.Name, rec.Nummer, fmtEUR(betrag), status})
	}
	titel := fmt.Sprintf("Jahresabrechnung %d", jahr)
	pdf := neuesPDF(s, titel, false)
	hinweis := fmt.Sprintf("%s. Zusammen %s berechnet, davon %s bezahlt und %s noch offen. Danach folgen alle Rechnungen, je Pächter eine Seite.",
		mehrzahl(len(recs), "Rechnung", "Rechnungen"), fmtEUR(summe), fmtEUR(bezahlt), fmtEUR(offen))
	pdfListe(pdf, s, titel, hinweis, []string{"Garten", "Name", "Rechnung", "Betrag", "Stand"}, []float64{20, 62, 38, 28, 22}, zeilen, 10)

	for _, rec := range recs {
		sq, _, err := layoutFit(rec.Settings, rec.Paechter, rec.Ablesung, rec.Result, false)
		if err != nil {
			return nil, "", bad(fmt.Sprintf("%s: %v", rec.Nummer, err))
		}
		drawInvoicePage(pdf, rec.Settings, rec.Paechter, rec.Ablesung, rec.Result, sq)
	}
	b, err := pdfBytes(pdf)
	return b, fmt.Sprintf("Jahresabrechnung_%d.pdf", jahr), err
}
