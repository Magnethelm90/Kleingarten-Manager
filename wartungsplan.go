package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Wartungsplan für Gemeinschaftsflächen: Hecke schneiden, Wege kehren, Spielplatz prüfen …
// Jede Aufgabe kehrt in festen Abständen wieder; das Cockpit erinnert rechtzeitig.

const maxWartungsaufgaben = 200

type Wartungsaufgabe struct {
	ID      string `json:"id"`
	Titel   string `json:"titel"`
	Ort     string `json:"ort,omitempty"`
	Monate  int    `json:"monate"`           // Abstand in Monaten (1–60)
	Faellig string `json:"faellig"`          // nächstes Fälligkeitsdatum
	Letzte  string `json:"letzte,omitempty"` // zuletzt erledigt
}

func (s *Store) wartungsplanAufgabenLocked(heute time.Time, add func(stufe, titel, text, ziel, knopf string)) {
	var faellig, bald []string
	for _, w := range s.d.Wartungsplan {
		t := tageBis(heute, w.Faellig)
		txt := w.Titel
		if w.Ort != "" {
			txt += " (" + w.Ort + ")"
		}
		switch {
		case t < 0:
			faellig = append(faellig, txt)
		case t <= 14:
			bald = append(bald, fmt.Sprintf("%s am %s", txt, germanDate(w.Faellig)))
		}
	}
	if len(faellig) > 0 {
		add("warn", mehrzahl(len(faellig), "Pflegearbeit überfällig", "Pflegearbeiten überfällig"), strings.Join(faellig, ", ")+".", "admin:inventar:wartung", "Wartungsplan")
	}
	if len(bald) > 0 {
		add("info", mehrzahl(len(bald), "Pflegearbeit bald fällig", "Pflegearbeiten bald fällig"), strings.Join(bald, ", ")+".", "admin:inventar:wartung", "Wartungsplan")
	}
}

func (a *App) handleWartungsplan(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	l := append([]Wartungsaufgabe{}, a.st.d.Wartungsplan...)
	sort.SliceStable(l, func(i, j int) bool { return l[i].Faellig < l[j].Faellig })
	writeJSON(w, 200, l)
}

func (a *App) handleWartungsaufgabeSpeichern(w http.ResponseWriter, r *http.Request) {
	var in Wartungsaufgabe
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in.Titel, in.Ort = trim(in.Titel, 120), trim(in.Ort, 120)
	if in.Titel == "" || len(in.Titel) > 120 || len(in.Ort) > 120 {
		writeErr(w, bad("Bitte einen Namen für die Arbeit eingeben (höchstens 120 Zeichen)."))
		return
	}
	if in.Monate < 1 || in.Monate > 60 {
		writeErr(w, bad("Der Abstand muss zwischen 1 und 60 Monaten liegen."))
		return
	}
	if in.Faellig == "" {
		in.Faellig = time.Now().Format("2006-01-02")
	}
	if !gueltigesDatum(in.Faellig, false) || !gueltigesDatum(in.Letzte, true) {
		writeErr(w, bad("Das Datum ist ungültig."))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if in.ID != "" {
		for i := range a.st.d.Wartungsplan {
			if a.st.d.Wartungsplan[i].ID == in.ID {
				a.st.d.Wartungsplan[i] = in
				a.st.audit("Pflegearbeit geändert: %s", in.Titel)
				a.speichernUndAntworten(w, in)
				return
			}
		}
		writeErr(w, notFound("Pflegearbeit nicht gefunden"))
		return
	}
	if len(a.st.d.Wartungsplan) >= maxWartungsaufgaben {
		writeErr(w, bad("Es sind höchstens 200 Pflegearbeiten möglich."))
		return
	}
	in.ID = newID()
	a.st.d.Wartungsplan = append(a.st.d.Wartungsplan, in)
	a.st.audit("Pflegearbeit angelegt: %s", in.Titel)
	a.speichernUndAntworten(w, in)
}

// handleWartungsaufgabeErledigt: Datum im Body (leer = heute); nächste Fälligkeit = Datum + Abstand.
func (a *App) handleWartungsaufgabeErledigt(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Datum string `json:"datum"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if in.Datum == "" {
		in.Datum = time.Now().Format("2006-01-02")
	}
	if !gueltigesDatum(in.Datum, false) {
		writeErr(w, bad("Das Datum ist ungültig."))
		return
	}
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Wartungsplan {
		x := &a.st.d.Wartungsplan[i]
		if x.ID == id {
			d, _ := parseDate(in.Datum)
			x.Letzte = in.Datum
			x.Faellig = monateDazu(d, x.Monate).Format("2006-01-02")
			a.st.audit("Pflegearbeit erledigt: %s", x.Titel)
			a.speichernUndAntworten(w, *x)
			return
		}
	}
	writeErr(w, notFound("Pflegearbeit nicht gefunden"))
}

func (a *App) handleWartungsaufgabeLoeschen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i, x := range a.st.d.Wartungsplan {
		if x.ID == id {
			a.st.d.Wartungsplan = append(a.st.d.Wartungsplan[:i], a.st.d.Wartungsplan[i+1:]...)
			a.st.audit("Pflegearbeit gelöscht: %s", x.Titel)
			a.speichernUndAntworten(w, map[string]bool{"ok": true})
			return
		}
	}
	writeErr(w, notFound("Pflegearbeit nicht gefunden"))
}
