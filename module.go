package main

import (
	"net/http"
	"slices"
)

// Bereiche (Module): Zusatzfunktionen, die nicht jeder Verein braucht, lassen sich ausblenden. Ausgeblendete
// Bereiche verschwinden aus Menü und Cockpit; ihre Daten bleiben erhalten und sind nach dem Einschalten wieder da.
// Gespeichert wird die Liste der ausgeschalteten Bereiche, damit bestehende Datenbestände (ohne Liste)
// weiterhin alles zeigen. Neue Installationen starten mit dem Grundpaket (standardAusModule).

var module = []string{"warteliste", "rundschreiben", "inventar", "bauten", "eichung", "barkasse", "haushalt", "spenden", "vorauszahlung", "pachtgrenze", "jubilaeen"}

var standardAusModule = []string{"inventar", "bauten", "eichung", "barkasse", "haushalt", "spenden", "vorauszahlung", "pachtgrenze", "jubilaeen"}

// modulAn: ist der Bereich eingeschaltet? Der Aufrufer hält s.mu.
func (s *Store) modulAn(name string) bool { return !slices.Contains(s.d.AusModule, name) }

func (s *Store) moduleLocked() map[string]bool {
	m := make(map[string]bool, len(module))
	for _, x := range module {
		m[x] = s.modulAn(x)
	}
	return m
}

func (a *App) handleModule(w http.ResponseWriter, r *http.Request) {
	var in map[string]bool
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	var aus []string
	for _, x := range module {
		an, gesetzt := in[x]
		if (gesetzt && !an) || (!gesetzt && !a.st.modulAn(x)) {
			aus = append(aus, x)
		}
	}
	a.st.d.AusModule = aus
	a.st.audit("Bereiche geändert")
	a.speichernUndAntworten(w, a.st.moduleLocked())
}
