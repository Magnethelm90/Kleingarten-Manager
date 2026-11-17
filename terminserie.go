package main

import (
	"net/http"
	"time"
)

// Terminserien: Wiederkehrende Termine (z. B. Vorstandssitzung jeden ersten Dienstag) werden einmal beschrieben und als
// einzelne Termine angelegt. Danach lässt sich jeder einzeln ändern oder löschen; es wird keine Regel gespeichert.

const (
	maxSerie        = 60
	serieMaxJahre   = 3
	rhythmusWoche   = "woechentlich"
	rhythmus14Tage  = "14tage"
	rhythmusMonat   = "monatlich"           // gleicher Kalendertag
	rhythmusMonatWT = "monatlich_wochentag" // z. B. erster Dienstag
)

// nteWochentag liefert den n-ten Wochentag wd im Monat (n 1–4) oder den letzten (n = 5).
func nteWochentag(jahr int, monat time.Month, wd time.Weekday, n int) time.Time {
	erster := time.Date(jahr, monat, 1, 0, 0, 0, 0, time.UTC)
	d := erster.AddDate(0, 0, (int(wd)-int(erster.Weekday())+7)%7)
	if n >= 5 {
		for d.AddDate(0, 0, 7).Month() == monat {
			d = d.AddDate(0, 0, 7)
		}
		return d
	}
	return d.AddDate(0, 0, 7*(n-1))
}

// serienDaten berechnet die Termindaten ab start (einschließlich) bis bis (einschließlich).
func serienDaten(start, bis time.Time, rhythmus string) []time.Time {
	var out []time.Time
	grenze := start.AddDate(serieMaxJahre, 0, 0)
	if bis.After(grenze) {
		bis = grenze
	}
	switch rhythmus {
	case rhythmusWoche, rhythmus14Tage:
		schritt := 7
		if rhythmus == rhythmus14Tage {
			schritt = 14
		}
		for d := start; !d.After(bis) && len(out) < maxSerie; d = d.AddDate(0, 0, schritt) {
			out = append(out, d)
		}
	case rhythmusMonat:
		for i := 0; len(out) < maxSerie; i++ {
			// gleicher Kalendertag; gibt es ihn im Monat nicht (31.), entfällt der Monat
			m := time.Date(start.Year(), start.Month()+time.Month(i), 1, 0, 0, 0, 0, time.UTC)
			if m.After(bis) {
				break
			}
			d := time.Date(m.Year(), m.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
			if d.Month() != m.Month() || d.After(bis) {
				continue
			}
			out = append(out, d)
		}
	case rhythmusMonatWT:
		n := (start.Day()-1)/7 + 1
		if start.AddDate(0, 0, 7).Month() != start.Month() {
			n = 5 // letzter Wochentag des Monats
		}
		for i := 0; len(out) < maxSerie; i++ {
			m := time.Date(start.Year(), start.Month()+time.Month(i), 1, 0, 0, 0, 0, time.UTC)
			if m.After(bis) {
				break
			}
			d := nteWochentag(m.Year(), m.Month(), start.Weekday(), n)
			if d.After(bis) {
				break
			}
			out = append(out, d)
		}
	}
	return out
}

func (a *App) handleTerminSerie(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Termin
		Rhythmus string `json:"rhythmus"`
		Bis      string `json:"bis"` // letzter möglicher Tag
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	vorlage, err := cleanTermin(in.Termin)
	if err != nil {
		writeErr(w, err)
		return
	}
	start, _ := parseDate(vorlage.Datum)
	bis, err := parseDate(in.Bis)
	if err != nil || bis.Before(start) {
		writeErr(w, bad("Bitte ein Enddatum der Serie eintragen, das nach dem ersten Termin liegt"))
		return
	}
	if in.Rhythmus != rhythmusWoche && in.Rhythmus != rhythmus14Tage && in.Rhythmus != rhythmusMonat && in.Rhythmus != rhythmusMonatWT {
		writeErr(w, bad("Unbekannter Rhythmus"))
		return
	}
	daten := serienDaten(start, bis, in.Rhythmus)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if len(a.st.d.Termine)+len(daten) > maxTermine {
		writeErr(w, bad("Das ergäbe zu viele Termine. Bitte zuerst alte löschen"))
		return
	}
	for _, d := range daten {
		t := vorlage
		t.ID, t.Datum = newID(), d.Format("2006-01-02")
		a.st.d.Termine = append(a.st.d.Termine, t)
	}
	sortiereTermine(a.st.d.Termine)
	a.st.audit("Terminserie angelegt: %s, %d Termine ab %s", vorlage.Titel, len(daten), germanDate(vorlage.Datum))
	a.speichernUndAntworten(w, map[string]int{"angelegt": len(daten)})
}
