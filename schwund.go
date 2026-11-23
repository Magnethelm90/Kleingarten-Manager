package main

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Wasserschwund: Der Hauptzähler (bzw. die Versorgerrechnung) wird Jahr für Jahr mit der Summe der Gartenzähler
// verglichen. Die Differenz ist Wasser, das niemand bezahlt – undichte Leitungen, stehengebliebene oder falsch
// abgelesene Zähler. Dazu die Gärten, deren Verbrauch auffällt, als Ansatzpunkt für die Suche.

const (
	schwundWarnProzent  = 10.0 // ab diesem Anteil meldet sich das Cockpit
	schwundAnstiegPunkt = 5.0  // oder wenn der Anteil gegenüber dem Vorjahr so stark steigt
)

type schwundWerte struct {
	Haupt   *float64 `json:"haupt,omitempty"`   // Hauptzähler bzw. Versorger
	Gaerten float64  `json:"gaerten"`           // Summe der Gartenzähler
	Menge   *float64 `json:"menge,omitempty"`   // Haupt - Gärten
	Prozent *float64 `json:"prozent,omitempty"` // Menge / Haupt
	Kosten  *float64 `json:"kosten,omitempty"`  // Menge × Preis je Einheit
}

type schwundJahr struct {
	Jahr   int          `json:"jahr"`
	Wasser schwundWerte `json:"wasser"`
	Strom  schwundWerte `json:"strom"`
	// Unvollstaendig: Gärten mit Wasserzähler, aber ohne Stand – sie fehlen in der Summe (Schwund dann zu hoch)
	Unvollstaendig int `json:"unvollstaendig"`
}

type schwundAuffaellig struct {
	PaechterID string   `json:"paechterId"`
	Gartennr   string   `json:"gartennr"`
	Name       string   `json:"name"`
	Hinweise   []string `json:"hinweise"`
}

type schwundAntwort struct {
	Jahre      []schwundJahr       `json:"jahre"`
	Auffaellig []schwundAuffaellig `json:"auffaellig"`
	Jahr       int                 `json:"jahr"` // Jahr der auffälligen Gärten
	WarnAb     float64             `json:"warnAb"`
}

func schwundRechnen(haupt *float64, gaerten, preisVersorger *float64, preisVerein float64) schwundWerte {
	w := schwundWerte{Haupt: haupt, Gaerten: round2(*gaerten)}
	if haupt == nil || *haupt <= 0 {
		return w
	}
	m := round2(*haupt - *gaerten)
	p := math.Round(m / *haupt * 1000) / 10
	w.Menge, w.Prozent = &m, &p
	preis := preisVerein
	if preisVersorger != nil && *preisVersorger > 0 {
		preis = *preisVersorger / *haupt
	}
	if m > 0 && preis > 0 {
		k := round2(m * preis)
		w.Kosten = &k
	}
	return w
}

// schwundLocked: alle Jahre, ältestes zuerst. Der Aufrufer hält s.mu.
func (s *Store) schwundLocked() []schwundJahr {
	var out []schwundJahr
	jahre := s.years()
	sort.Ints(jahre)
	for _, y := range jahre {
		v, ok := s.viewLocked(y)
		if !ok {
			continue
		}
		var vs Versorger
		if j := s.d.Jahre[yearKey(y)]; j != nil && j.Versorger != nil {
			vs = *j.Versorger
		}
		// alle abgelesenen Zähler zählen, auch wenn die Rechnung noch unvollständig ist (z. B. Stunden fehlen)
		var wasser, strom float64
		fehlt := 0
		for _, p := range v.Paechter {
			r := calculate(v.Settings, p, v.Ablesungen[p.ID])
			if inv := s.validInvoiceLocked(y, p.ID); inv != nil {
				r = inv.Result
			}
			if r.WasserVerbrauch != nil && *r.WasserVerbrauch > 0 {
				wasser += *r.WasserVerbrauch
			} else if r.WasserVerbrauch == nil && strings.TrimSpace(p.WasserzaehlerNr) != "" {
				fehlt++
			}
			if r.EnergieVerbrauch != nil && *r.EnergieVerbrauch > 0 {
				strom += *r.EnergieVerbrauch
			}
		}
		out = append(out, schwundJahr{
			Jahr:           y,
			Wasser:         schwundRechnen(vs.WasserM3, &wasser, vs.WasserEUR, v.Settings.WasserPreis),
			Strom:          schwundRechnen(vs.StromKWh, &strom, vs.StromEUR, v.Settings.EnergiePreis),
			Unvollstaendig: fehlt,
		})
	}
	return out
}

// schwundAuffaelligLocked: Gärten mit Hinweisen der Plausibilitätsprüfung oder ganz ohne Wasserverbrauch.
func (s *Store) schwundAuffaelligLocked(jahr int) []schwundAuffaellig {
	out := []schwundAuffaellig{}
	v, ok := s.viewLocked(jahr)
	if !ok {
		return out
	}
	results := map[string]Result{}
	for _, p := range v.Paechter {
		results[p.ID] = calculate(v.Settings, p, v.Ablesungen[p.ID])
	}
	hinweise := plausiLocked(v, results, s.historyLocked())
	for _, p := range v.Paechter {
		if p.Geloescht {
			continue
		}
		hs := append([]string(nil), hinweise[p.ID]...)
		if r := results[p.ID]; r.WasserVerbrauch != nil && *r.WasserVerbrauch == 0 && strings.TrimSpace(p.WasserzaehlerNr) != "" {
			hs = append(hs, "Wasser: kein Verbrauch – steht der Zähler still?")
		}
		if len(hs) > 0 {
			out = append(out, schwundAuffaellig{p.ID, p.Gartennr, p.Name, hs})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return naturalLess(out[i].Gartennr, out[j].Gartennr) })
	return out
}

func (a *App) handleSchwund(w http.ResponseWriter, r *http.Request) {
	jahr := a.yearParam(r)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	writeJSON(w, 200, schwundAntwort{Jahre: a.st.schwundLocked(), Auffaellig: a.st.schwundAuffaelligLocked(jahr), Jahr: jahr, WarnAb: schwundWarnProzent})
}

// schwundAufgabenLocked: Warnung im Cockpit für das jüngste Jahr mit Hauptzählerwert.
func (s *Store) schwundAufgabenLocked(heute time.Time, add func(stufe, titel, text, ziel, knopf string)) {
	jahre := s.schwundLocked()
	for i := len(jahre) - 1; i >= 0; i-- {
		w := jahre[i].Wasser
		if w.Prozent == nil {
			continue
		}
		if jahre[i].Unvollstaendig > 0 {
			return // noch nicht alle Stände da: der Schwund wäre zu hoch
		}
		anstieg := false
		for j := i - 1; j >= 0; j-- {
			if p := jahre[j].Wasser.Prozent; p != nil {
				anstieg = *w.Prozent-*p >= schwundAnstiegPunkt
				break
			}
		}
		if *w.Prozent < schwundWarnProzent && !anstieg {
			return
		}
		text := fmt.Sprintf("Der Hauptzähler zeigt %s m³ mehr als alle Gärten zusammen (%s %%)", fmtFlex(*w.Menge), fmtFlex(*w.Prozent))
		if w.Kosten != nil {
			text += ", das sind etwa " + fmtEUR(*w.Kosten)
		}
		text += ". Leitungen und auffällige Zähler prüfen."
		add("warn", fmt.Sprintf("Wasserschwund %d: %s %%", jahre[i].Jahr, fmtFlex(*w.Prozent)), text, "admin:kassenbericht", "Schwund ansehen")
		return
	}
}
