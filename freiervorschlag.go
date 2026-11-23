package main

import (
	"net/http"
	"strings"
	"unicode"
)

// Vorschlag für einen frei werdenden Garten: die Warteliste in der Reihenfolge der Anmeldung, dazu je Bewerber,
// welche Wünsche (klein, groß, Laube …) zum Garten passen. Entschieden wird weiter vom Vorstand; wer abgesagt hat,
// erscheint nicht. Es wird nur gelesen.

type vorschlagZeile struct {
	Bewerber
	Platz      int      `json:"platz"`
	Passt      []string `json:"passt"`
	PasstNicht []string `json:"passtNicht"`
	Empfohlen  bool     `json:"empfohlen"`
}

type freierVorschlag struct {
	Gartennr string           `json:"gartennr"`
	Groesse  float64          `json:"groesse"`
	Laube    bool             `json:"laube"`
	Zeilen   []vorschlagZeile `json:"zeilen"`
}

const kleinBis, grossAb = 250.0, 350.0

// wunschPrueft vergleicht den Wunschtext mit den Eigenschaften des Gartens.
func wunschPrueft(wunsch string, groesse float64, laube bool) (passt, passtNicht []string) {
	w := strings.ToLower(wunsch)
	worte := strings.FieldsFunc(w, func(r rune) bool { return !unicode.IsLetter(r) })
	hat := func(stamm string) bool {
		for _, x := range worte {
			if strings.HasPrefix(x, stamm) {
				return true
			}
		}
		return false
	}
	entscheide := func(ok bool, text string) {
		if ok {
			passt = append(passt, text)
		} else {
			passtNicht = append(passtNicht, text)
		}
	}
	if groesse > 0 && hat("klein") {
		entscheide(groesse <= kleinBis, "klein")
	}
	if groesse > 0 && (hat("groß") || hat("gross") || hat("groess")) {
		entscheide(groesse >= grossAb, "groß")
	}
	switch {
	case hat("ohne") && strings.Contains(w, "laube"):
		entscheide(!laube, "ohne Laube")
	case hat("laube") || hat("hütte") || hat("huette"):
		entscheide(laube, "mit Laube")
	}
	return
}

func (s *Store) freierVorschlagLocked(nr string) freierVorschlag {
	v := freierVorschlag{Gartennr: nr, Zeilen: []vorschlagZeile{}}
	for i := len(s.d.Paechter) - 1; i >= 0; i-- { // zuletzt angelegter Pächter dieses Gartens
		if p := s.d.Paechter[i]; strings.TrimSpace(p.Gartennr) == nr && p.Gartengroesse > 0 {
			v.Groesse = p.Gartengroesse
			break
		}
	}
	for _, b := range s.d.Bauwerke {
		if strings.TrimSpace(b.Gartennr) == nr && b.Art == "Laube" {
			v.Laube = true
		}
	}
	l := append([]Bewerber{}, s.d.Warteliste...)
	sortiereWarteliste(l)
	platz := 0
	for _, b := range l {
		platz++
		if b.BesichtigungErgebnis == "kein Interesse" || b.BesichtigungErgebnis == "nicht erschienen" {
			continue
		}
		z := vorschlagZeile{Bewerber: b, Platz: platz}
		z.Passt, z.PasstNicht = wunschPrueft(b.Wunsch, v.Groesse, v.Laube)
		if z.Passt == nil {
			z.Passt = []string{}
		}
		if z.PasstNicht == nil {
			z.PasstNicht = []string{}
		}
		v.Zeilen = append(v.Zeilen, z)
	}
	// Empfohlen: der erste Bewerber ohne Widerspruch zwischen Wunsch und Garten
	for i := range v.Zeilen {
		if len(v.Zeilen[i].PasstNicht) == 0 {
			v.Zeilen[i].Empfohlen = true
			break
		}
	}
	return v
}

func (a *App) handleFreierVorschlag(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	writeJSON(w, 200, a.st.freierVorschlagLocked(strings.TrimSpace(r.PathValue("nr"))))
}
