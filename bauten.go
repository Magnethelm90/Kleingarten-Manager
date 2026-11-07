package main

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Laubenkataster: Lauben, Gewächshäuser, Schuppen und überdachte Freisitze je Garten. Nach § 3 Abs. 2 BKleingG
// darf die Laube einschließlich überdachtem Freisitz höchstens 24 m² Grundfläche haben; ältere Lauben genießen
// oft Bestandsschutz. Die Bauten gehören zum Garten, nicht zur Person, und bleiben beim Pächterwechsel erhalten.

type Bauwerk struct {
	ID             string  `json:"id"`
	Gartennr       string  `json:"gartennr"`
	Art            string  `json:"art"`
	Flaeche        float64 `json:"flaeche"` // Grundfläche in m²
	Baujahr        int     `json:"baujahr,omitempty"`
	Genehmigt      string  `json:"genehmigt"` // ja | nein | unbekannt
	Bestandsschutz bool    `json:"bestandsschutz,omitempty"`
	Notiz          string  `json:"notiz,omitempty"`
}

var bauwerkArten = []string{"Laube", "Überdachter Freisitz", "Gewächshaus", "Schuppen", "Sonstiges"}

const (
	laubeHoechstQm = 24.0
	maxBauwerke    = 5000
)

// zaehltZurLaube: Laube und überdachter Freisitz bilden zusammen die nach BKleingG begrenzte Grundfläche.
func (b Bauwerk) zaehltZurLaube() bool { return b.Art == "Laube" || b.Art == "Überdachter Freisitz" }

type bautenGarten struct {
	Gartennr       string    `json:"gartennr"`
	Name           string    `json:"name"`
	Laube          float64   `json:"laube"` // Laube + überdachter Freisitz
	Gesamt         float64   `json:"gesamt"`
	Ueber          bool      `json:"ueber"`          // über 24 m² ohne Bestandsschutz
	Ungenehmigt    int       `json:"ungenehmigt"`    // Bauten mit »nein«
	Bestandsschutz bool      `json:"bestandsschutz"` // mind. ein Teil der Laube hat Bestandsschutz
	Bauten         []Bauwerk `json:"bauten"`
}

type bautenAntwort struct {
	Gaerten []bautenGarten `json:"gaerten"`
	Arten   []string       `json:"arten"`
	Grenze  float64        `json:"grenze"`
}

// bautenLocked: alle belegten Gärten und Gärten mit eingetragenen Bauten. Der Aufrufer hält s.mu.
func (s *Store) bautenLocked() []bautenGarten {
	m := map[string]*bautenGarten{}
	hole := func(nr string) *bautenGarten {
		if m[nr] == nil {
			m[nr] = &bautenGarten{Gartennr: nr, Bauten: []Bauwerk{}}
		}
		return m[nr]
	}
	for _, p := range s.d.Paechter {
		if !p.Geloescht && p.AusgeschiedenAm == "" && p.Gartennr != "" {
			hole(p.Gartennr).Name = p.Name
		}
	}
	for _, b := range s.d.Bauwerke {
		g := hole(b.Gartennr)
		g.Bauten = append(g.Bauten, b)
		g.Gesamt += b.Flaeche
		if b.zaehltZurLaube() {
			g.Laube += b.Flaeche
			g.Bestandsschutz = g.Bestandsschutz || b.Bestandsschutz
		}
		if b.Genehmigt == "nein" {
			g.Ungenehmigt++
		}
	}
	out := make([]bautenGarten, 0, len(m))
	for _, g := range m {
		g.Laube, g.Gesamt = round2(g.Laube), round2(g.Gesamt)
		g.Ueber = g.Laube > laubeHoechstQm+0.005 && !g.Bestandsschutz
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return naturalLess(out[i].Gartennr, out[j].Gartennr) })
	return out
}

func (a *App) bautenAntwortLocked() bautenAntwort {
	return bautenAntwort{Gaerten: a.st.bautenLocked(), Arten: bauwerkArten, Grenze: laubeHoechstQm}
}

func (a *App) handleBauten(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	writeJSON(w, 200, a.bautenAntwortLocked())
}

func (a *App) handleBauwerkSpeichern(w http.ResponseWriter, r *http.Request) {
	var in Bauwerk
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in.Gartennr, in.Notiz = trim(in.Gartennr, 20), trim(in.Notiz, 300)
	gueltig := false
	for _, x := range bauwerkArten {
		gueltig = gueltig || x == in.Art
	}
	if !gueltig {
		in.Art = "Sonstiges"
	}
	if in.Genehmigt != "ja" && in.Genehmigt != "nein" {
		in.Genehmigt = "unbekannt"
	}
	if in.Gartennr == "" || !validNum(in.Flaeche) || in.Flaeche <= 0 || in.Flaeche > 500 ||
		(in.Baujahr != 0 && (in.Baujahr < 1900 || in.Baujahr > time.Now().Year())) {
		writeErr(w, bad("Bitte Garten, Grundfläche in m² und ggf. ein gültiges Baujahr eintragen"))
		return
	}
	in.Flaeche = math.Round(in.Flaeche*100) / 100
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Bauwerke {
		if in.ID != "" && a.st.d.Bauwerke[i].ID == in.ID {
			a.st.d.Bauwerke[i] = in
			a.st.audit("Bauwerk geändert: Garten %s, %s %s m²", in.Gartennr, in.Art, fmtFlex(in.Flaeche))
			a.speichernUndAntworten(w, a.bautenAntwortLocked())
			return
		}
	}
	if len(a.st.d.Bauwerke) >= maxBauwerke {
		writeErr(w, bad("Zu viele Bauwerke"))
		return
	}
	in.ID = newID()
	a.st.d.Bauwerke = append(a.st.d.Bauwerke, in)
	a.st.audit("Bauwerk eingetragen: Garten %s, %s %s m²", in.Gartennr, in.Art, fmtFlex(in.Flaeche))
	a.speichernUndAntworten(w, a.bautenAntwortLocked())
}

func (a *App) handleBauwerkLoeschen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i, b := range a.st.d.Bauwerke {
		if b.ID == r.PathValue("id") {
			a.st.d.Bauwerke = append(a.st.d.Bauwerke[:i], a.st.d.Bauwerke[i+1:]...)
			a.st.audit("Bauwerk gelöscht: Garten %s, %s", b.Gartennr, b.Art)
			a.speichernUndAntworten(w, a.bautenAntwortLocked())
			return
		}
	}
	writeErr(w, notFound("Bauwerk nicht gefunden"))
}

func (a *App) laubenlistePDF(r *http.Request) ([]byte, string, error) {
	a.st.mu.Lock()
	s := a.st.d.Settings
	l := a.st.bautenLocked()
	a.st.mu.Unlock()
	titel := "Laubenkataster"
	zeilen := [][]string{}
	for _, g := range l {
		var teile []string
		for _, b := range g.Bauten {
			t := fmt.Sprintf("%s %s m²", b.Art, fmtFlex(b.Flaeche))
			if b.Baujahr > 0 {
				t += fmt.Sprintf(" (%d)", b.Baujahr)
			}
			if b.Genehmigt == "nein" {
				t += " – nicht genehmigt"
			}
			teile = append(teile, t)
		}
		hinweis := ""
		switch {
		case g.Ueber:
			hinweis = "über 24 m²"
		case g.Bestandsschutz && g.Laube > laubeHoechstQm:
			hinweis = "Bestandsschutz"
		}
		laube := ""
		if g.Laube > 0 {
			laube = fmtFlex(g.Laube) + " m²"
		}
		zeilen = append(zeilen, []string{g.Gartennr, g.Name, laube, strings.Join(teile, "; "), hinweis, ""})
	}
	if len(zeilen) == 0 {
		return nil, "", bad("Es sind noch keine Gärten vorhanden")
	}
	pdf := neuesPDF(s, titel, true)
	pdfListe(pdf, s, titel, "Laube einschließlich überdachtem Freisitz höchstens 24 m² Grundfläche (§ 3 Abs. 2 BKleingG). Spalte »Geprüft« für die Begehung.",
		[]string{"Garten", "Pächter", "Laube", "Bauten", "Hinweis", "Geprüft"}, []float64{16, 46, 20, 125, 30, 20}, zeilen, 9)
	out, err := pdfBytes(pdf)
	return out, "Laubenkataster.pdf", err
}

func (s *Store) bautenAufgabenLocked(_ time.Time, add func(stufe, titel, text, ziel, knopf string)) {
	ueber, ohne := 0, 0
	for _, g := range s.bautenLocked() {
		if g.Ueber {
			ueber++
		}
		ohne += g.Ungenehmigt
	}
	if ueber > 0 {
		add("warn", fmt.Sprintf("%d Laube(n) größer als 24 m²", ueber),
			"Laube mit überdachtem Freisitz überschreitet die Grenze des Bundeskleingartengesetzes. Bestandsschutz prüfen oder Rückbau besprechen.", "admin:maengel:bauten", "Laubenkataster")
	}
	if ohne > 0 {
		add("info", fmt.Sprintf("%d Bauwerk(e) ohne Genehmigung", ohne), "Bei der nächsten Begehung klären, ob eine Genehmigung nachgeholt werden kann.", "admin:maengel:bauten", "Laubenkataster")
	}
}
