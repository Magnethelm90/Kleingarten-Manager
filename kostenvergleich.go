package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// Kostenvergleich über die Jahre: Verbrauch und Ausgaben des Vereins im Verlauf, Ausgaben nach Kategorie und der
// Verbrauch je Garten. Auffällig ist, wer gegenüber dem Vorjahr deutlich mehr verbraucht (Wasserrohrbruch, defekter
// Zähler, Dauerlauf der Pumpe) oder wo die Ausgaben stark gestiegen sind. Es werden nur vorhandene Daten gelesen.

// vergleichHistorie: höchstens so viele Jahre zeigen.
const vergleichHistorie = 8

type kostenJahr struct {
	Jahr      int                `json:"jahr"`
	Wasser    float64            `json:"wasser"` // m³ aller Pächter
	Strom     float64            `json:"strom"`  // kWh aller Pächter
	Ausgaben  float64            `json:"ausgaben"`
	Rechnung  float64            `json:"rechnungen"` // Summe der Pächterrechnungen
	Kategorie map[string]float64 `json:"kategorien"`
}

type kostenWert struct {
	Wasser *float64 `json:"wasser"`
	Strom  *float64 `json:"strom"`
}

type kostenGarten struct {
	Gartennr string       `json:"gartennr"`
	Name     string       `json:"name"`  // aktueller Pächter
	Werte    []kostenWert `json:"werte"` // je Jahr in der Reihenfolge von Jahre
	Hinweise []string     `json:"hinweise"`
}

type kostenvergleich struct {
	Jahre      []int          `json:"jahre"` // älteste zuerst
	Verein     []kostenJahr   `json:"verein"`
	Kategorien []string       `json:"kategorien"`
	Gaerten    []kostenGarten `json:"gaerten"`
	Hinweise   []string       `json:"hinweise"` // zum Verein insgesamt
}

func addPtr(sum **float64, v *float64) {
	if v == nil {
		return
	}
	if *sum == nil {
		z := 0.0
		*sum = &z
	}
	**sum += *v
}

// auffaellig: deutlich mehr als im Vorjahr (mindestens +50 % und mindestens minAbs mehr).
func auffaellig(neu, alt *float64, minAbs float64) bool {
	return neu != nil && alt != nil && *alt > 0 && *neu >= *alt*1.5 && *neu-*alt >= minAbs
}

func (s *Store) kostenvergleichLocked() kostenvergleich {
	hist := s.historyLocked()
	jahre := s.years() // neueste zuerst
	if len(jahre) > vergleichHistorie {
		jahre = jahre[:vergleichHistorie]
	}
	var k kostenvergleich
	for i := len(jahre) - 1; i >= 0; i-- {
		k.Jahre = append(k.Jahre, jahre[i])
	}
	idx := map[int]int{}
	for i, y := range k.Jahre {
		idx[y] = i
	}
	gaerten := map[string]*kostenGarten{}
	katSet := map[string]bool{}
	for _, y := range k.Jahre {
		v, ok := s.viewLocked(y)
		if !ok {
			continue
		}
		kb := s.kassenberichtLocked(v)
		kj := kostenJahr{Jahr: y, Wasser: round2(kb.WasserVerbrauch), Strom: round2(kb.StromVerbrauch), Ausgaben: kb.AusgabenSumme, Rechnung: kb.Summen.Gesamt, Kategorie: map[string]float64{}}
		for _, c := range kb.AusgabenKategorie {
			kj.Kategorie[c.Kategorie] = c.Summe
			katSet[c.Kategorie] = true
		}
		k.Verein = append(k.Verein, kj)
		for _, p := range v.Paechter {
			nr := strings.TrimSpace(p.Gartennr)
			if nr == "" {
				continue
			}
			g := gaerten[nr]
			if g == nil {
				g = &kostenGarten{Gartennr: nr, Werte: make([]kostenWert, len(k.Jahre)), Hinweise: []string{}}
				gaerten[nr] = g
			}
			if p.AusgeschiedenAm == "" && !p.Geloescht {
				g.Name = p.Name
			} else if g.Name == "" {
				g.Name = p.Name
			}
			for _, e := range hist[p.ID] {
				if e.Jahr == y {
					w := &g.Werte[idx[y]]
					addPtr(&w.Wasser, e.Wasser)
					addPtr(&w.Strom, e.Strom)
				}
			}
		}
	}
	for c := range katSet {
		k.Kategorien = append(k.Kategorien, c)
	}
	sort.Strings(k.Kategorien)
	for _, g := range gaerten {
		// letztes und vorletztes Jahr mit Werten vergleichen
		n := len(g.Werte)
		if n >= 2 {
			a, b := g.Werte[n-1], g.Werte[n-2]
			if auffaellig(a.Wasser, b.Wasser, minAbweichungWasser) {
				g.Hinweise = append(g.Hinweise, fmt.Sprintf("Wasser %s m³ statt %s m³ im Vorjahr", fmtFlex(round2(*a.Wasser)), fmtFlex(round2(*b.Wasser))))
			}
			if auffaellig(a.Strom, b.Strom, minAbweichungStrom) {
				g.Hinweise = append(g.Hinweise, fmt.Sprintf("Strom %s kWh statt %s kWh im Vorjahr", fmtFlex(round2(*a.Strom)), fmtFlex(round2(*b.Strom))))
			}
		}
		k.Gaerten = append(k.Gaerten, *g)
	}
	sort.SliceStable(k.Gaerten, func(i, j int) bool { return naturalLess(k.Gaerten[i].Gartennr, k.Gaerten[j].Gartennr) })
	if k.Gaerten == nil {
		k.Gaerten = []kostenGarten{}
	}
	if k.Verein == nil {
		k.Verein = []kostenJahr{}
	}
	if k.Kategorien == nil {
		k.Kategorien = []string{}
	}
	// Vereinsweit: Ausgaben je Kategorie gegenüber dem Vorjahr
	k.Hinweise = []string{}
	if n := len(k.Verein); n >= 2 {
		a, b := k.Verein[n-1], k.Verein[n-2]
		for _, c := range k.Kategorien {
			if b.Kategorie[c] > 0 && a.Kategorie[c] >= b.Kategorie[c]*1.25 && a.Kategorie[c]-b.Kategorie[c] >= 100 {
				k.Hinweise = append(k.Hinweise, fmt.Sprintf("%s: %s statt %s im Vorjahr (%d)", c, fmtEUR(a.Kategorie[c]), fmtEUR(b.Kategorie[c]), b.Jahr))
			}
		}
		if auffaellig(&a.Wasser, &b.Wasser, 50) {
			k.Hinweise = append(k.Hinweise, fmt.Sprintf("Wasserverbrauch der Anlage %s m³ statt %s m³ im Vorjahr", fmtFlex(a.Wasser), fmtFlex(b.Wasser)))
		}
	}
	return k
}

func (a *App) handleKostenvergleich(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	k := a.st.kostenvergleichLocked()
	a.st.mu.Unlock()
	writeJSON(w, 200, k)
}
