package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Fristen aus der Satzung: Einladungsfrist zur Mitgliederversammlung (hier) und Kündigungsfrist (KuendigungsRegel in
// kuendigung.go). Das Cockpit rechnet daraus die Tage aus, bis zu denen etwas erledigt sein muss.

const einladungTageStandard = 14

type Satzungsfristen struct {
	// EinladungTage: so viele Tage vor der Versammlung muss die Einladung verschickt sein (0 = Standard 14)
	EinladungTage int `json:"einladungTage,omitempty"`
}

func (f Satzungsfristen) einladungTage() int {
	if f.EinladungTage < 1 || f.EinladungTage > 90 {
		return einladungTageStandard
	}
	return f.EinladungTage
}

// einladungsschluss: letzter Tag, an dem die Einladung zu einer Versammlung am datum verschickt werden muss.
func einladungsschluss(datum string, tage int) (string, bool) {
	t, err := parseDate(datum)
	if err != nil {
		return "", false
	}
	return t.AddDate(0, 0, -tage).Format("2006-01-02"), true
}

type satzungsfristenAnsicht struct {
	EinladungTage int              `json:"einladungTage"`
	Kuendigung    KuendigungsRegel `json:"kuendigung"`
}

func (a *App) handleSatzungsfristen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	writeJSON(w, 200, satzungsfristenAnsicht{a.st.d.Satzungsfristen.einladungTage(), a.st.d.KuendigungsRegel.mitStandard()})
}

func (a *App) handleSatzungsfristenSpeichern(w http.ResponseWriter, r *http.Request) {
	var in Satzungsfristen
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if in.EinladungTage < 1 || in.EinladungTage > 90 {
		writeErr(w, bad("Die Einladungsfrist muss zwischen 1 und 90 Tagen liegen"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	a.st.d.Satzungsfristen = in
	a.st.audit("Einladungsfrist zur Mitgliederversammlung: %d Tage", in.EinladungTage)
	a.speichernUndAntworten(w, satzungsfristenAnsicht{in.einladungTage(), a.st.d.KuendigungsRegel.mitStandard()})
}

// versammlungAufgabe: Einladung bis zum Einladungsschluss verschicken (14 Tage Vorlauf), in der letzten Woche vor der
// Versammlung an die Anwesenheitsliste erinnern. Titel ohne »heute/morgen«, damit Zurückstellen hält.
func (s *Store) versammlungAufgabe(heute time.Time, t Termin, add func(stufe, titel, text, ziel, knopf string)) {
	n := tageBis(heute, t.Datum)
	if n < 0 || n > 400 {
		return
	}
	tage := s.d.Satzungsfristen.einladungTage()
	schluss, ok := einladungsschluss(t.Datum, tage)
	if !ok {
		return
	}
	m := tageBis(heute, schluss)
	switch {
	case m >= 0 && m <= 14:
		add("warn", fmt.Sprintf("Einladung zur %s bis %s verschicken", t.Titel, germanDate(schluss)),
			fmt.Sprintf("Die Versammlung ist am %s. Laut Satzung muss die Einladung %d Tage vorher verschickt sein – also spätestens am %s.",
				germanDate(t.Datum), tage, germanDate(schluss)), "tab:termine", "Einladung")
	case m < 0 && m >= -3:
		add("warn", fmt.Sprintf("Einladungsfrist zur %s ist abgelaufen", t.Titel),
			fmt.Sprintf("Die Einladung musste bis %s verschickt sein. Ist sie raus? Sonst wäre nicht fristgerecht eingeladen – dann die Versammlung lieber verschieben.", germanDate(schluss)),
			"tab:termine", "Termine")
	case m < 0 && n <= 7:
		add("info", fmt.Sprintf("%s am %s vorbereiten", t.Titel, germanDate(t.Datum)), "Anwesenheitsliste drucken und die Unterlagen für die Versammlung bereitlegen.", "tab:termine", "Termine")
	}
}

// kuendigungsfristAufgabeLocked: Steht bei einem Garten schon das Zweitschreiben mit letzter Frist aus der Mängelliste,
// erinnert das Cockpit 30 Tage vor dem Stichtag daran, bis wann eine Kündigung durch den Verein beim Pächter sein muss.
func (s *Store) kuendigungsfristAufgabeLocked(heute time.Time, add func(stufe, titel, text, ziel, knopf string)) {
	regel := s.d.KuendigungsRegel.mitStandard()
	bis := fmt.Sprintf("%d-%s", heute.Year(), regel.Bis)
	m := tageBis(heute, bis)
	if m < 0 || m > 30 {
		return
	}
	gaerten := map[string]bool{}
	for _, x := range s.d.Maengel {
		if x.ErledigtAm == "" && x.Stufen["zweit"] != "" {
			gaerten[x.Gartennr] = true
		}
	}
	if len(gaerten) == 0 {
		return
	}
	liste := make([]string, 0, len(gaerten))
	for g := range gaerten {
		liste = append(liste, g)
	}
	sort.Strings(liste)
	wer := "Garten " + liste[0]
	if len(liste) > 1 {
		wer = "Gärten " + strings.Join(liste, ", ")
	}
	add("warn", fmt.Sprintf("Kündigungsfrist des Vereins: bis %s", germanDate(bis)),
		fmt.Sprintf("%s: Das Zweitschreiben ist verschickt, die Mängel sind nicht beseitigt. Soll der Verein zum %s kündigen, muss die Kündigung bis zum %s beim Pächter sein. Vorher im Vorstand beschließen.",
			wer, germanDate(regel.fristgerecht(bis)), germanDate(bis)),
		"admin:maengel", "Mängel")
}
