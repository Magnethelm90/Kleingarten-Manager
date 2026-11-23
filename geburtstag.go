package main

import (
	"fmt"
	"net/http"
	"time"
)

// Geburtstage: nur mit ausdrücklicher Einwilligung der Person gespeichert. Ohne Haken wird nichts angezeigt
// und kein Datum behalten. Das Cockpit erinnert eine Woche vorher an die Glückwünsche.

type Geburtstag struct {
	Datum        string `json:"datum"` // JJJJ-MM-TT (das Jahr zählt nur fürs »wird 70«)
	Einwilligung bool   `json:"einwilligung"`
}

func (a *App) handleGeburtstagLesen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	writeJSON(w, 200, a.st.d.Geburtstage[id])
}

func (a *App) handleGeburtstagSpeichern(w http.ResponseWriter, r *http.Request) {
	var in Geburtstag
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	found := false
	for _, p := range a.st.d.Paechter {
		if p.ID == id {
			found = true
		}
	}
	if !found {
		writeErr(w, notFound("Pächter nicht gefunden"))
		return
	}
	// Ohne Einwilligung oder ohne Datum wird nichts gespeichert (Datensparsamkeit).
	if !in.Einwilligung || in.Datum == "" {
		delete(a.st.d.Geburtstage, id)
		a.st.audit("Geburtstag entfernt (Pächter %s)", id)
		a.speichernUndAntworten(w, Geburtstag{})
		return
	}
	d, err := parseDate(in.Datum)
	if err != nil || d.After(time.Now()) || d.Year() < 1900 {
		writeErr(w, bad("Das Geburtsdatum ist ungültig."))
		return
	}
	if a.st.d.Geburtstage == nil {
		a.st.d.Geburtstage = map[string]Geburtstag{}
	}
	a.st.d.Geburtstage[id] = Geburtstag{Datum: in.Datum, Einwilligung: true}
	a.st.audit("Geburtstag gespeichert (Pächter %s, mit Einwilligung)", id)
	a.speichernUndAntworten(w, a.st.d.Geburtstage[id])
}

func (d *Data) geburtstagPersonEntfernen(id string) bool {
	if _, ok := d.Geburtstage[id]; !ok {
		return false
	}
	delete(d.Geburtstage, id)
	return true
}

// naechsterGeburtstag liefert Tage bis zum nächsten Geburtstag und das dann erreichte Alter.
func naechsterGeburtstag(heute time.Time, datum string) (int, int, bool) {
	d, err := parseDate(datum)
	if err != nil {
		return 0, 0, false
	}
	h := time.Date(heute.Year(), heute.Month(), heute.Day(), 0, 0, 0, 0, time.UTC)
	for jahr := h.Year(); jahr <= h.Year()+1; jahr++ {
		m, t := d.Month(), d.Day()
		if m == time.February && t == 29 && !(jahr%4 == 0 && (jahr%100 != 0 || jahr%400 == 0)) {
			t = 28
		}
		n := time.Date(jahr, m, t, 0, 0, 0, 0, time.UTC)
		if !n.Before(h) {
			return int(n.Sub(h).Hours() / 24), jahr - d.Year(), true
		}
	}
	return 0, 0, false
}

func (s *Store) geburtstagAufgabenLocked(heute time.Time, add func(stufe, titel, text, ziel, knopf string)) {
	for _, p := range s.d.Paechter {
		g, ok := s.d.Geburtstage[p.ID]
		if !ok || !g.Einwilligung || p.Geloescht || p.AusgeschiedenAm != "" {
			continue
		}
		tage, alter, ok := naechsterGeburtstag(heute, g.Datum)
		if !ok || tage > 7 {
			continue
		}
		wann := "heute"
		if tage == 1 {
			wann = "morgen"
		} else if tage > 1 {
			wann = fmt.Sprintf("in %d Tagen", tage)
		}
		add("info", "Geburtstag: "+p.Name, fmt.Sprintf("%s wird %s %d. Glückwunsch nicht vergessen.", p.Name, wann, alter), "admin:paechter", "Pächter")
	}
}
