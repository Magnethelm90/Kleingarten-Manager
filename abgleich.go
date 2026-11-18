package main

import (
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"
)

// Abgleich zwischen zwei Rechnern (z. B. Kassier und Schriftführer): Jeder Rechner kann eine Abgleich-Datei mit seinen
// Einträgen erzeugen (USB-Stick), der andere liest sie ein. Es werden nur NEUE Einträge übernommen – vorhandene werden
// nie überschrieben oder gelöscht. Vorher gibt es immer eine Vorschau. Belege, Fotos und Barkasse sind nicht dabei.

const abgleichKennung = "kleingarten-abgleich"

type abgleichJahr struct {
	Ausgaben   []Ausgabe           `json:"ausgaben,omitempty"`
	Ablesungen map[string]Ablesung `json:"ablesungen,omitempty"`
}

type abgleichDatei struct {
	Kennung     string                  `json:"kennung"`
	Erstellt    string                  `json:"erstellt"`
	Verein      string                  `json:"verein"`
	Jahre       map[string]abgleichJahr `json:"jahre"`
	Termine     []Termin                `json:"termine,omitempty"`
	Beschluesse []Beschluss             `json:"beschluesse,omitempty"`
	Maengel     []Mangel                `json:"maengel,omitempty"`
	Spenden     []Spende                `json:"spenden,omitempty"`
	Stunden     []EinsatzStunden        `json:"stunden,omitempty"`
}

type abgleichZeile struct {
	Bereich string `json:"bereich"`
	Neu     int    `json:"neu"`
	Da      int    `json:"schonDa"`
}

type abgleichErgebnis struct {
	Zeilen    []abgleichZeile `json:"zeilen"`
	Neu       int             `json:"neu"`
	Angewandt bool            `json:"angewandt"`
	Hinweis   string          `json:"hinweis,omitempty"`
}

func (s *Store) abgleichExportLocked() abgleichDatei {
	d := abgleichDatei{Kennung: abgleichKennung, Erstellt: time.Now().Format(time.RFC3339), Verein: s.d.Settings.VereinName, Jahre: map[string]abgleichJahr{}}
	for k, j := range s.d.Jahre {
		aj := abgleichJahr{Ablesungen: map[string]Ablesung{}}
		for _, a := range j.Ausgaben {
			a.Belege, a.Beleg = nil, ""
			aj.Ausgaben = append(aj.Ausgaben, a)
		}
		for id, a := range j.Ablesungen {
			aj.Ablesungen[id] = a
		}
		d.Jahre[k] = aj
	}
	d.Termine = append(d.Termine, s.d.Termine...)
	d.Beschluesse = append(d.Beschluesse, s.d.Beschluesse...)
	for _, m := range s.d.Maengel {
		m.Foto = ""
		d.Maengel = append(d.Maengel, m)
	}
	d.Spenden = append(d.Spenden, s.d.Spenden...)
	d.Stunden = append(d.Stunden, s.d.Einsatzstunden...)
	return d
}

func ablesungLeer(a Ablesung) bool {
	return a.WasserVJ == nil && a.WasserAkt == nil && a.StromVJ == nil && a.StromAkt == nil && a.Stunden == nil &&
		a.Versicherung == 0 && a.Grundsteuer == 0 && a.Auslagen == 0 && a.Abschlag == 0 && strings.TrimSpace(a.Hinweis) == "" &&
		a.WasserWechsel == nil && a.StromWechsel == nil
}

// abgleichLocked zählt (anwenden=false) oder übernimmt (anwenden=true) die neuen Einträge. Der Aufrufer hält s.mu.
func (s *Store) abgleichLocked(in abgleichDatei, anwenden bool) abgleichErgebnis {
	texteBereinigen(reflect.ValueOf(&in))
	var res abgleichErgebnis
	zeile := func(bereich string, neu, da int) {
		if neu+da > 0 {
			res.Zeilen = append(res.Zeilen, abgleichZeile{bereich, neu, da})
			res.Neu += neu
		}
	}
	aktivePaechterIDs := map[string]bool{}
	for _, p := range s.d.Paechter {
		aktivePaechterIDs[p.ID] = true
	}

	// Ausgaben und Zählerstände je Jahr (nur vorhandene, nicht abgeschlossene Jahre)
	var nAus, dAus, nAb, dAb int
	keys := make([]string, 0, len(in.Jahre))
	for k := range in.Jahre {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ij := in.Jahre[k]
		mine := s.d.Jahre[k]
		if mine == nil || mine.Abgeschlossen {
			continue
		}
		have := map[string]bool{}
		for _, a := range mine.Ausgaben {
			have[a.ID] = true
		}
		for _, a := range ij.Ausgaben {
			if a.ID == "" || have[a.ID] {
				dAus++
				continue
			}
			c, err := cleanAusgabe(a)
			if err != nil {
				continue
			}
			c.ID, c.Geprueft, c.GeprueftAm = a.ID, false, ""
			nAus++
			if anwenden {
				mine.Ausgaben = append(mine.Ausgaben, c)
			}
		}
		if mine.Ablesungen == nil && anwenden {
			mine.Ablesungen = map[string]Ablesung{}
		}
		for id, a := range ij.Ablesungen {
			if !aktivePaechterIDs[id] || ablesungLeer(a) {
				continue
			}
			if cur, ok := mine.Ablesungen[id]; ok && !ablesungLeer(cur) {
				dAb++
				continue
			}
			nAb++
			if anwenden {
				mine.Ablesungen[id] = a
			}
		}
	}
	zeile("Ausgaben", nAus, dAus)
	zeile("Zählerstände", nAb, dAb)

	// Termine
	haveT := map[string]bool{}
	for _, t := range s.d.Termine {
		haveT[t.ID] = true
	}
	var nT, dT int
	for _, t := range in.Termine {
		if t.ID == "" || haveT[t.ID] {
			dT++
			continue
		}
		nT++
		if anwenden {
			s.d.Termine = append(s.d.Termine, t)
			haveT[t.ID] = true
		}
	}
	zeile("Termine", nT, dT)

	// Beschlüsse (Nummer wird bei Kollision neu vergeben)
	haveB := map[string]bool{}
	for _, b := range s.d.Beschluesse {
		haveB[b.ID] = true
	}
	var nB, dB int
	for _, b := range in.Beschluesse {
		if b.ID == "" || haveB[b.ID] {
			dB++
			continue
		}
		nB++
		if anwenden {
			if len(b.Datum) >= 4 {
				for _, x := range s.d.Beschluesse {
					if x.Nummer == b.Nummer {
						b.Nummer = s.beschlussNummerLocked(b)
						break
					}
				}
			}
			s.d.Beschluesse = append(s.d.Beschluesse, b)
			haveB[b.ID] = true
		}
	}
	zeile("Beschlüsse", nB, dB)

	// Mängel
	haveM := map[string]bool{}
	for _, m := range s.d.Maengel {
		haveM[m.ID] = true
	}
	var nM, dM int
	for _, m := range in.Maengel {
		if m.ID == "" || haveM[m.ID] {
			dM++
			continue
		}
		nM++
		if anwenden {
			m.Foto = ""
			s.d.Maengel = append(s.d.Maengel, m)
			haveM[m.ID] = true
		}
	}
	zeile("Mängel", nM, dM)

	// Spenden
	haveS := map[string]bool{}
	for _, x := range s.d.Spenden {
		haveS[x.ID] = true
	}
	var nS, dS int
	for _, x := range in.Spenden {
		if x.ID == "" || haveS[x.ID] {
			dS++
			continue
		}
		nS++
		if anwenden {
			s.d.Spenden = append(s.d.Spenden, x)
			haveS[x.ID] = true
		}
	}
	zeile("Spenden", nS, dS)

	// Einsatzstunden (nur zu vorhandenen Terminen und Pächtern, je Termin und Pächter einmal)
	haveE := map[string]bool{}
	for _, e := range s.d.Einsatzstunden {
		haveE[e.TerminID+"|"+e.PaechterID] = true
	}
	var nE, dE int
	for _, e := range in.Stunden {
		k := e.TerminID + "|" + e.PaechterID
		if haveE[k] {
			dE++
			continue
		}
		if !haveT[e.TerminID] || !aktivePaechterIDs[e.PaechterID] {
			continue
		}
		nE++
		if anwenden {
			s.d.Einsatzstunden = append(s.d.Einsatzstunden, e)
			haveE[k] = true
		}
	}
	zeile("Einsatzstunden", nE, dE)

	res.Angewandt = anwenden
	if res.Zeilen == nil {
		res.Zeilen = []abgleichZeile{}
	}
	return res
}

func (a *App) handleAbgleichExport(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	d := a.st.abgleichExportLocked()
	a.st.mu.Unlock()
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Abgleich_%s.json"`, time.Now().Format("2006-01-02")))
	writeJSON(w, 200, d)
}

// handleAbgleichEinlesen: ?anwenden=1 übernimmt, sonst nur Vorschau.
func (a *App) handleAbgleichEinlesen(w http.ResponseWriter, r *http.Request) {
	var in abgleichDatei
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if in.Kennung != abgleichKennung {
		writeErr(w, bad("Das ist keine Abgleich-Datei. Bitte auf dem anderen Rechner »Abgleich-Datei erstellen« benutzen."))
		return
	}
	anwenden := r.URL.Query().Get("anwenden") == "1"
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	res := a.st.abgleichLocked(in, anwenden)
	if anwenden && res.Neu > 0 {
		if err := a.st.saveLocked(); err != nil {
			writeErr(w, err)
			return
		}
		a.st.audit("Abgleich übernommen: %d neue Einträge", res.Neu)
	}
	writeJSON(w, 200, res)
}
