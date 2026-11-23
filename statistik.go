package main

import (
	"net/http"
	"sort"
)

// Statistik über die Jahre: Verbrauch, Ausgaben und Rechnungssummen (aus dem Kostenvergleich) sowie die Zahlungsmoral
// (Anteil bezahlter Rechnungen und durchschnittliche Tage bis zur Zahlung). Es wird nur gelesen, was schon da ist.

type zahlungsmoral struct {
	Jahr    int     `json:"jahr"`
	Anzahl  int     `json:"anzahl"`  // gültige Rechnungen
	Bezahlt int     `json:"bezahlt"` // davon vollständig bezahlt
	Quote   float64 `json:"quote"`   // Prozent bezahlt
	Tage    float64 `json:"tage"`    // durchschnittliche Tage von Ausstellung bis Zahlung (nur bezahlte), 0 = keine Angabe
}

type statistikErgebnis struct {
	Kosten  kostenvergleich `json:"kosten"`
	Zahlung []zahlungsmoral `json:"zahlung"`
}

func (s *Store) statistikLocked() statistikErgebnis {
	res := statistikErgebnis{Kosten: s.kostenvergleichLocked(), Zahlung: []zahlungsmoral{}}
	nach := map[int]*zahlungsmoral{}
	summeTage := map[int]float64{}
	nTage := map[int]int{}
	for _, r := range s.d.Rechnungen {
		if r.Status != statusGueltig {
			continue
		}
		z := nach[r.Jahr]
		if z == nil {
			z = &zahlungsmoral{Jahr: r.Jahr}
			nach[r.Jahr] = z
		}
		z.Anzahl++
		if r.BezahltAm != "" && openAmount(r) <= 0.004 {
			z.Bezahlt++
			if len(r.Ausgestellt) >= 10 {
				a, e1 := parseDate(r.Ausgestellt[:10])
				b, e2 := parseDate(r.BezahltAm)
				if e1 == nil && e2 == nil && !b.Before(a) {
					summeTage[r.Jahr] += b.Sub(a).Hours() / 24
					nTage[r.Jahr]++
				}
			}
		}
	}
	for y, z := range nach {
		if z.Anzahl > 0 {
			z.Quote = round2(float64(z.Bezahlt) * 100 / float64(z.Anzahl))
		}
		if nTage[y] > 0 {
			z.Tage = round2(summeTage[y] / float64(nTage[y]))
		}
		res.Zahlung = append(res.Zahlung, *z)
	}
	sort.Slice(res.Zahlung, func(i, j int) bool { return res.Zahlung[i].Jahr < res.Zahlung[j].Jahr })
	return res
}

func (a *App) handleStatistik(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	res := a.st.statistikLocked()
	a.st.mu.Unlock()
	writeJSON(w, 200, res)
}
