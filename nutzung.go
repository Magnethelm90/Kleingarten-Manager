package main

import (
	"math"
	"net/http"
	"sort"
	"strings"
)

// Kleingärtnerische Nutzung (»Drittelregelung«): Nach der Rechtsprechung zum Bundeskleingartengesetz soll in
// der Regel mindestens ein Drittel der Gartenfläche dem Anbau von Obst, Gemüse und anderen Früchten dienen.
// Bei der Begehung werden die Anteile geschätzt; das Programm zeigt, welche Gärten darunter liegen. Es ist eine
// Hilfe für das Gespräch mit dem Pächter, keine Vermessung – maßgeblich sind Satzung, Gartenordnung und Verband.

// Nutzung ist eine geschätzte Flächenaufteilung eines Gartens bei einer Begehung (Anteile in Prozent).
type Nutzung struct {
	ID         string  `json:"id"`
	Gartennr   string  `json:"gartennr"`
	PaechterID string  `json:"paechterId,omitempty"`
	Datum      string  `json:"datum"`
	Anbau      float64 `json:"anbau"`    // Obst, Gemüse, Kräuter, Beerensträucher
	Erholung   float64 `json:"erholung"` // Rasen, Zierpflanzen, Sitzplatz
	Bebauung   float64 `json:"bebauung"` // Laube, Wege, befestigte Flächen
	Notiz      string  `json:"notiz,omitempty"`
}

const (
	anbauMindestens = 100.0 / 3
	maxNutzungen    = 5000
)

func (n Nutzung) unterDrittel() bool { return n.Anbau < anbauMindestens-0.5 }

func cleanNutzung(n Nutzung) (Nutzung, error) {
	n.Gartennr = trim(n.Gartennr, 20)
	n.Notiz = trim(strings.Join(strings.Fields(n.Notiz), " "), 200)
	if n.Gartennr == "" {
		return n, bad("Bitte die Gartennummer angeben")
	}
	if d, err := parseDate(n.Datum); err != nil || d.Year() < 2000 || d.Year() > 2100 {
		return n, bad("Bitte das Datum der Begehung eintragen")
	}
	for _, v := range []float64{n.Anbau, n.Erholung, n.Bebauung} {
		if !validNum(v) || v < 0 || v > 100 {
			return n, bad("Die Anteile bitte in Prozent zwischen 0 und 100 eintragen")
		}
	}
	n.Anbau, n.Erholung, n.Bebauung = math.Round(n.Anbau), math.Round(n.Erholung), math.Round(n.Bebauung)
	if sum := n.Anbau + n.Erholung + n.Bebauung; sum < 90 || sum > 110 {
		return n, bad("Die drei Anteile sollten zusammen etwa 100 % ergeben")
	}
	return n, nil
}

type nutzungZeile struct {
	Gartennr string   `json:"gartennr"`
	Name     string   `json:"name"`
	Letzte   *Nutzung `json:"letzte,omitempty"`
	Unter    bool     `json:"unterDrittel"`
}

// nutzungUebersichtLocked: je belegtem Garten die jüngste Schätzung.
func (s *Store) nutzungUebersichtLocked() []nutzungZeile {
	letzte := map[string]*Nutzung{}
	for i := range s.d.Nutzungen {
		n := &s.d.Nutzungen[i]
		if l := letzte[n.Gartennr]; l == nil || n.Datum >= l.Datum {
			letzte[n.Gartennr] = n
		}
	}
	var out []nutzungZeile
	for nr, p := range s.belegteGaertenLocked() {
		z := nutzungZeile{Gartennr: nr, Name: p.Name}
		if l := letzte[nr]; l != nil {
			c := *l
			z.Letzte, z.Unter = &c, c.unterDrittel()
		}
		out = append(out, z)
	}
	sort.SliceStable(out, func(i, j int) bool { return naturalLess(out[i].Gartennr, out[j].Gartennr) })
	return out
}

func (a *App) handleNutzungListe(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	writeJSON(w, 200, a.st.nutzungUebersichtLocked())
}

func (a *App) handleNutzungSpeichern(w http.ResponseWriter, r *http.Request) {
	var in Nutzung
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in, err := cleanNutzung(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	p, ok := a.st.aktiverPaechterLocked(in.Gartennr)
	if !ok {
		writeErr(w, bad("Für Garten "+in.Gartennr+" ist kein Pächter eingetragen"))
		return
	}
	if len(a.st.d.Nutzungen) >= maxNutzungen {
		writeErr(w, bad("Es sind sehr viele Schätzungen gespeichert"))
		return
	}
	in.ID, in.PaechterID = newID(), p.ID
	a.st.d.Nutzungen = append(a.st.d.Nutzungen, in)
	a.st.audit("Nutzung geschätzt: Garten %s, Anbau %s %%", in.Gartennr, fmtFlex(in.Anbau))
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, in)
}

func (a *App) handleNutzungLoeschen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i, n := range a.st.d.Nutzungen {
		if n.ID == id {
			a.st.d.Nutzungen = append(a.st.d.Nutzungen[:i], a.st.d.Nutzungen[i+1:]...)
			a.st.audit("Nutzungsschätzung gelöscht: Garten %s", n.Gartennr)
			if err := a.st.saveLocked(); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, map[string]bool{"ok": true})
			return
		}
	}
	writeErr(w, notFound("Schätzung nicht gefunden"))
}

// nutzungPersonEntfernen: Schätzungen gehören zur Person, die den Garten damals hatte.
func (d *Data) nutzungPersonEntfernen(id string) bool {
	rest := d.Nutzungen[:0]
	changed := false
	for _, n := range d.Nutzungen {
		if n.PaechterID == id {
			changed = true
			continue
		}
		rest = append(rest, n)
	}
	d.Nutzungen = rest
	return changed
}
