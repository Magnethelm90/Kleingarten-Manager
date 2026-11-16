package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Arbeitsstunden-Konto: Wer hat im Abrechnungsjahr wie viele Stunden Gemeinschaftsarbeit geleistet, was fehlt noch und
// was würde die Ersatzzahlung kosten? Die Stunden stammen aus den Arbeitseinsätzen (einsatz.go) und den Eintragungen
// beim Pächter; hier lassen sich einzelne Stunden außerhalb von Einsätzen nachtragen. Die Ersatzzahlung kommt
// unverändert in die Jahresrechnung (calc.go) – das Konto zeigt sie nur vorab.

type stundenZeile struct {
	PaechterID  string  `json:"paechterId"`
	Gartennr    string  `json:"gartennr"`
	Mitgliedsnr string  `json:"mitgliedsnr"`
	Name        string  `json:"name"`
	Ist         float64 `json:"ist"`
	Fehlt       float64 `json:"fehlt"`
	Ersatz      float64 `json:"ersatz"`    // voraussichtliche Zahlung für fehlende Stunden
	Einsaetze   int     `json:"einsaetze"` // Arbeitseinsätze mit eingetragenen Stunden
	Erfuellt    bool    `json:"erfuellt"`
}

type stundenKonto struct {
	Jahr        int            `json:"jahr"`
	Pflicht     float64        `json:"pflicht"`
	JeStunde    float64        `json:"jeStunde"`
	Zeilen      []stundenZeile `json:"zeilen"`
	Erfuellt    int            `json:"erfuellt"`
	Summe       float64        `json:"summe"`       // alle geleisteten Stunden
	ErsatzSumme float64        `json:"ersatzSumme"` // zu erwartende Ersatzzahlungen insgesamt
}

func (s *Store) stundenkontoLocked(jahr int) (stundenKonto, bool) {
	v, ok := s.viewLocked(jahr)
	if !ok {
		return stundenKonto{}, false
	}
	k := stundenKonto{Jahr: jahr, Pflicht: v.Settings.Pflichtstunden, JeStunde: v.Settings.NachzahlungJeStd, Zeilen: []stundenZeile{}}
	einsaetze := map[string]int{}
	for _, e := range s.d.Einsatzstunden {
		if e.Jahr == jahr && e.Stunden > 0 {
			einsaetze[e.PaechterID]++
		}
	}
	for _, p := range v.Paechter {
		if p.Geloescht || p.AusgeschiedenAm != "" {
			continue
		}
		z := stundenZeile{PaechterID: p.ID, Gartennr: p.Gartennr, Mitgliedsnr: p.Mitgliedsnr, Name: p.Name, Einsaetze: einsaetze[p.ID]}
		if a := v.Ablesungen[p.ID]; a.Stunden != nil {
			z.Ist = *a.Stunden
		}
		if fehl := k.Pflicht - z.Ist; fehl > 0 {
			z.Fehlt = round2(fehl)
			z.Ersatz = round2(fehl * k.JeStunde)
		}
		z.Erfuellt = z.Fehlt == 0
		if z.Erfuellt {
			k.Erfuellt++
		}
		k.Summe += z.Ist
		k.ErsatzSumme += z.Ersatz
		k.Zeilen = append(k.Zeilen, z)
	}
	k.Summe, k.ErsatzSumme = round2(k.Summe), round2(k.ErsatzSumme)
	sort.SliceStable(k.Zeilen, func(i, j int) bool { return naturalLess(k.Zeilen[i].Gartennr, k.Zeilen[j].Gartennr) })
	return k, true
}

func (a *App) handleStundenkonto(w http.ResponseWriter, r *http.Request) {
	jahr := a.yearParam(r)
	a.st.mu.Lock()
	k, ok := a.st.stundenkontoLocked(jahr)
	a.st.mu.Unlock()
	if !ok {
		writeErr(w, notFound("Dieses Jahr gibt es nicht"))
		return
	}
	writeJSON(w, 200, k)
}

// handleStundenBuchen trägt Stunden außerhalb eines Arbeitseinsatzes nach (oder nimmt sie mit negativem Wert zurück).
func (a *App) handleStundenBuchen(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PaechterID string  `json:"paechterId"`
		Jahr       int     `json:"jahr"`
		Stunden    float64 `json:"stunden"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if in.Stunden == 0 || in.Stunden < -maxEinsatzStunden || in.Stunden > maxEinsatzStunden {
		writeErr(w, bad(fmt.Sprintf("Bitte Stunden zwischen -%d und %d eintragen (nicht 0)", maxEinsatzStunden, maxEinsatzStunden)))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	j := a.st.d.Jahre[yearKey(in.Jahr)]
	if j == nil || j.Abgeschlossen {
		writeErr(w, bad("Dieses Abrechnungsjahr ist abgeschlossen oder gibt es nicht"))
		return
	}
	var p *Paechter
	for i := range a.st.d.Paechter {
		if a.st.d.Paechter[i].ID == in.PaechterID && !a.st.d.Paechter[i].Geloescht {
			p = &a.st.d.Paechter[i]
		}
	}
	if p == nil {
		writeErr(w, notFound("Pächter nicht gefunden"))
		return
	}
	bucheStunden(j, p.ID, round2(in.Stunden))
	a.st.audit("Arbeitsstunden nachgetragen: Garten %s, %s h (%d)", p.Gartennr, strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", in.Stunden), "0"), "."), in.Jahr)
	a.speichernUndAntworten(w, map[string]bool{"ok": true})
}

// stundenAufgabeLocked: im letzten Quartal des Jahres erinnern, wenn Pflichtstunden fehlen.
func (s *Store) stundenAufgabeLocked(heute time.Time, add func(stufe, titel, text, ziel, knopf string)) {
	if heute.Month() < time.October || s.d.Settings.Pflichtstunden <= 0 || heute.Year() != s.d.Settings.Jahr {
		return
	}
	k, ok := s.stundenkontoLocked(s.d.Settings.Jahr)
	if !ok {
		return
	}
	offen := len(k.Zeilen) - k.Erfuellt
	if offen > 0 {
		add("info", mehrzahl(offen, "Garten mit fehlenden Pflichtstunden", "Gärten mit fehlenden Pflichtstunden"),
			fmt.Sprintf("Bis Jahresende noch Gelegenheit zum Nachholen. Sonst fallen zusammen %s Ersatzzahlung an.", fmtEUR(k.ErsatzSumme)), "admin:berichte", "Stundenkonto")
	}
}
