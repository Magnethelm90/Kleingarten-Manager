package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Start eines neuen Vereins: Erkennen von Musterwerten aus der Erstausstattung (damit keine Rechnung mit
// »Mustername« oder »DE00 …« hinausgeht) und Festlegen des ersten Abrechnungsjahres, solange noch nichts
// gebucht ist.

// musterwerteLocked: Angaben, die noch aus der Erstausstattung stammen oder offensichtlich nicht passen.
// Leer bei Beispieldaten (die sind absichtlich erfunden).
func (s *Store) musterwerteLocked(heute time.Time) []string {
	out := []string{}
	if s.d.Demo {
		return out
	}
	st := s.d.Settings
	std := defaultSettings()
	if strings.Contains(st.VereinName, "Mustername") || strings.TrimSpace(st.VereinName) == "" {
		out = append(out, "Vereinsname")
	}
	if strings.ReplaceAll(st.IBAN, " ", "") == strings.ReplaceAll(std.IBAN, " ", "") || strings.TrimSpace(st.IBAN) == "" {
		out = append(out, "IBAN des Vereinskontos")
	}
	if strings.Contains(st.Absenderzeile, "Vorname Nachname") || strings.Contains(st.Absenderzeile, "example.org") || strings.Contains(st.Absenderzeile, "Mustername") {
		out = append(out, "Absenderzeile über der Anschrift")
	}
	ausgestellt := false
	for _, r := range s.d.Rechnungen {
		if r.Jahr == st.Jahr {
			ausgestellt = true
			break
		}
	}
	if !ausgestellt && tageBis(heute, st.Rechnungsdatum) < -30 {
		out = append(out, fmt.Sprintf("Rechnungsdatum (%s liegt in der Vergangenheit)", germanDate(st.Rechnungsdatum)))
	}
	return out
}

// startjahrAenderbarLocked: Das erste Abrechnungsjahr darf gewechselt werden, solange es nur ein Jahr gibt und darin
// nichts gebucht ist. Pächter dürfen schon angelegt sein (sie hängen an keinem Jahr).
func (s *Store) startjahrAenderbarLocked() bool {
	if len(s.d.Rechnungen) > 0 || len(s.d.Jahre) != 1 || len(s.d.Vorauszahlungen) > 0 || len(s.d.Spenden) > 0 || len(s.d.Haushalt) > 0 || len(s.d.Kassenpruefungen) > 0 {
		return false
	}
	for _, j := range s.d.Jahre {
		if j.Abgeschlossen || len(j.Ablesungen) > 0 || len(j.Ausgaben) > 0 || len(j.Barkasse) > 0 || j.Versorger != nil || j.Anfangsbestand != nil || j.BarAnfang != nil {
			return false
		}
	}
	return true
}

func (a *App) handleStartjahr(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Jahr int `json:"jahr"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	jetzt := time.Now().Year()
	if in.Jahr < jetzt-2 || in.Jahr > jetzt+1 {
		writeErr(w, bad(fmt.Sprintf("Bitte ein Jahr zwischen %d und %d wählen", jetzt-2, jetzt+1)))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if !a.st.startjahrAenderbarLocked() {
		writeErr(w, bad("Das Abrechnungsjahr lässt sich nur wechseln, solange noch nichts gebucht ist. Später geht es über den Jahreswechsel."))
		return
	}
	alt := a.st.d.Settings.Jahr
	if in.Jahr != alt {
		j := a.st.d.Jahre[yearKey(alt)]
		if j == nil {
			j = &Jahr{Ablesungen: map[string]Ablesung{}}
		}
		a.st.d.Jahre = map[string]*Jahr{yearKey(in.Jahr): j}
		a.st.d.Settings.Jahr = in.Jahr
		a.st.audit("Erstes Abrechnungsjahr: %d (vorher %d)", in.Jahr, alt)
	}
	// Rechnungen gehen Anfang des Folgejahres hinaus; ein Datum in der Vergangenheit wäre auf der Rechnung falsch
	s := &a.st.d.Settings
	if tageBis(time.Now(), s.Rechnungsdatum) < 0 || in.Jahr != alt {
		rd := time.Date(in.Jahr+1, 1, 10, 0, 0, 0, 0, time.UTC)
		if heute := time.Now(); rd.Before(heute) {
			rd = time.Date(heute.Year(), heute.Month(), heute.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 7)
		}
		s.Rechnungsdatum = rd.Format("2006-01-02")
		s.Zahlungsziel = rd.AddDate(0, 0, 36).Format("2006-01-02")
	}
	a.speichernUndAntworten(w, map[string]any{"jahr": s.Jahr, "rechnungsdatum": s.Rechnungsdatum, "zahlungsziel": s.Zahlungsziel})
}
