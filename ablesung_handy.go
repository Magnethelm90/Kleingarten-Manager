package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// Ablesung mit dem Handy: Das Programm läuft nur auf dem Vereinsrechner, nicht im Netz. Für den Rundgang gibt es
// deshalb eine einzelne HTML-Datei, die sich ohne Programm und ohne Internet auf dem Handy öffnen lässt (per
// Mail, USB oder Messenger). Dort tippt man die Zählerstände ein; Eingaben bleiben im Browser des Handys
// erhalten. Am Ende speichert man eine kleine Ergebnisdatei und liest sie im Programm wieder ein. Die Datei
// enthält Namen der Pächter und ist vertraulich zu behandeln; ins Programm übernommen wird nur, was gültig ist.

type handyZeile struct {
	ID        string   `json:"id"`
	Garten    string   `json:"garten"`
	Name      string   `json:"name"`
	WasserNr  string   `json:"wasserNr,omitempty"`
	StromNr   string   `json:"stromNr,omitempty"`
	WasserVJ  *float64 `json:"wasserVJ"`
	StromVJ   *float64 `json:"stromVJ"`
	WasserAkt *float64 `json:"wasserAkt"`
	StromAkt  *float64 `json:"stromAkt"`
}

type handyDatei struct {
	Version int          `json:"version"`
	Jahr    int          `json:"jahr"`
	Verein  string       `json:"verein"`
	Zeilen  []handyZeile `json:"zeilen"`
}

func (s *Store) handyDateiLocked() (handyDatei, bool) {
	v, ok := s.viewLocked(s.d.Settings.Jahr)
	if !ok {
		return handyDatei{}, false
	}
	d := handyDatei{Version: 1, Jahr: s.d.Settings.Jahr, Verein: s.d.Settings.VereinName, Zeilen: []handyZeile{}}
	for _, p := range v.Paechter {
		if p.Geloescht || p.AusgeschiedenAm != "" {
			continue
		}
		a := v.Ablesungen[p.ID]
		d.Zeilen = append(d.Zeilen, handyZeile{ID: p.ID, Garten: p.Gartennr, Name: p.Name, WasserNr: p.WasserzaehlerNr, StromNr: p.StromzaehlerNr,
			WasserVJ: a.WasserVJ, StromVJ: a.StromVJ, WasserAkt: a.WasserAkt, StromAkt: a.StromAkt})
	}
	sortiereNachGarten := func(i, j int) bool { return naturalLess(d.Zeilen[i].Garten, d.Zeilen[j].Garten) }
	sort.SliceStable(d.Zeilen, sortiereNachGarten)
	return d, true
}

func (a *App) handleAblesungHandy(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	d, ok := a.st.handyDateiLocked()
	a.st.mu.Unlock()
	if !ok || len(d.Zeilen) == 0 {
		writeErr(w, bad("Es sind keine Pächter angelegt"))
		return
	}
	daten, _ := json.Marshal(d) // maskiert <, > und & für die Einbettung in das Skript
	seite := strings.Replace(handySeite, "/*DATEN*/null", string(daten), 1)
	a.st.mu.Lock()
	a.st.audit("Ablesedatei fürs Handy erstellt (%d Gärten)", len(d.Zeilen))
	_ = a.st.saveLocked()
	a.st.mu.Unlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Ablesung_%d.html"`, d.Jahr))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(seite))
}

type handyErgebnis struct {
	Version int `json:"version"`
	Jahr    int `json:"jahr"`
	Staende []struct {
		ID     string   `json:"id"`
		Wasser *float64 `json:"wasser"`
		Strom  *float64 `json:"strom"`
	} `json:"staende"`
}

type handyBericht struct {
	Uebernommen  int      `json:"uebernommen"`
	Unveraendert int      `json:"unveraendert"`
	Hinweise     []string `json:"hinweise"`
}

func (a *App) handleAblesungHandyImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		writeErr(w, bad("Datei konnte nicht gelesen werden"))
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, bad("Bitte die Ergebnisdatei vom Handy auswählen"))
		return
	}
	defer file.Close()
	roh, err := io.ReadAll(file)
	var in handyErgebnis
	if err != nil || json.Unmarshal(roh, &in) != nil || in.Version != 1 {
		writeErr(w, bad("Das ist keine Ergebnisdatei der Handy-Ablesung"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if in.Jahr != a.st.d.Settings.Jahr {
		writeErr(w, bad(fmt.Sprintf("Die Datei gehört zum Jahr %d, im Programm ist das Jahr %d offen", in.Jahr, a.st.d.Settings.Jahr)))
		return
	}
	aktiv := map[string]*Paechter{}
	for i := range a.st.d.Paechter {
		p := &a.st.d.Paechter[i]
		if !p.Geloescht && p.AusgeschiedenAm == "" {
			aktiv[p.ID] = p
		}
	}
	// erst alles prüfen, dann übernehmen
	for _, x := range in.Staende {
		if aktiv[x.ID] == nil {
			writeErr(w, bad("Die Datei enthält einen unbekannten Garten – die Pächterliste hat sich seit der Erstellung geändert. Bitte die Datei neu erstellen."))
			return
		}
		if !validNumPtr(x.Wasser) || !validNumPtr(x.Strom) {
			writeErr(w, bad("Die Datei enthält ungültige Zählerstände"))
			return
		}
	}
	j := a.st.ensureYear(in.Jahr)
	var b handyBericht
	b.Hinweise = []string{}
	gleich := func(p, q *float64) bool { return p != nil && q != nil && *p == *q }
	for _, x := range in.Staende {
		p := aktiv[x.ID]
		ab := j.Ablesungen[x.ID]
		geaendert := false
		pruefe := func(art string, neu *float64, vj *float64, ziel **float64, aktuell *float64) {
			if neu == nil || gleich(neu, aktuell) {
				return
			}
			if vj != nil && *neu < *vj && (art == "Wasser" && ab.WasserWechsel == nil || art == "Strom" && ab.StromWechsel == nil) {
				b.Hinweise = append(b.Hinweise, fmt.Sprintf("Garten %s: %s-Stand %s ist kleiner als der Vorjahresstand %s – Zählerwechsel?", p.Gartennr, art, fmtFlex(*neu), fmtFlex(*vj)))
			}
			v := *neu
			*ziel = &v
			geaendert = true
		}
		pruefe("Wasser", x.Wasser, ab.WasserVJ, &ab.WasserAkt, ab.WasserAkt)
		pruefe("Strom", x.Strom, ab.StromVJ, &ab.StromAkt, ab.StromAkt)
		if geaendert {
			j.Ablesungen[x.ID] = ab
			b.Uebernommen++
		} else {
			b.Unveraendert++
		}
	}
	a.st.audit("Zählerstände aus der Handy-Ablesung übernommen (%d Gärten)", b.Uebernommen)
	a.speichernUndAntworten(w, b)
}
