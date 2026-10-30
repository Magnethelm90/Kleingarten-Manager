package main

import (
	"net/http"
	"sort"
	"strings"
	"time"
)

// Warteliste: Interessenten für einen Garten, in der Reihenfolge ihrer Anmeldung. Wird ein Garten frei,
// übernimmt der Pächterwechsel die Angaben des gewählten Eintrags und nimmt ihn von der Liste. Wer kein
// Interesse mehr hat, wird gelöscht, auch aus den Sicherungen (wie beim endgültigen Löschen eines Pächters).

// Bewerber ist ein Eintrag auf der Warteliste.
type Bewerber struct {
	ID      string `json:"id"`
	Seit    string `json:"seit"` // Tag der Anmeldung, JJJJ-MM-TT (bestimmt die Reihenfolge)
	Anrede  string `json:"anrede"`
	Name    string `json:"name"`
	Strasse string `json:"strasse"`
	PLZOrt  string `json:"plzOrt"`
	Email   string `json:"email,omitempty"`
	Telefon string `json:"telefon,omitempty"`
	// Wunsch: z. B. »klein, mit Laube, gern nahe am Eingang«
	Wunsch string `json:"wunsch,omitempty"`
	Notiz  string `json:"notiz,omitempty"`
	// BestaetigtAm: wann das Interesse zuletzt bestätigt wurde (leer = seit der Anmeldung nicht nachgefragt)
	BestaetigtAm string `json:"bestaetigtAm,omitempty"`
}

// wartelisteNachfragenJahre: nach so vielen Jahren ohne Bestätigung soll der Vorstand nachfragen, ob noch
// Interesse besteht (Datenschutz: nicht mehr benötigte Angaben nicht unbegrenzt aufheben).
const wartelisteNachfragenJahre = 2

// zuletztBestaetigt ist das jüngere Datum aus Anmeldung und letzter Bestätigung.
func (b Bewerber) zuletztBestaetigt() string {
	if b.BestaetigtAm > b.Seit {
		return b.BestaetigtAm
	}
	return b.Seit
}

func (b Bewerber) nachfragen(heute time.Time) bool {
	t, err := parseDate(b.zuletztBestaetigt())
	return err == nil && !t.AddDate(wartelisteNachfragenJahre, 0, 0).After(heute)
}

func cleanBewerber(b Bewerber) (Bewerber, error) {
	b.Anrede = trim(b.Anrede, 30)
	b.Name = trim(b.Name, 100)
	b.Strasse = trim(b.Strasse, 100)
	b.PLZOrt = trim(b.PLZOrt, 100)
	b.Email = trim(b.Email, 100)
	b.Telefon = trim(b.Telefon, 40)
	b.Wunsch = trim(b.Wunsch, 200)
	b.Notiz = trim(b.Notiz, 300)
	if b.Name == "" {
		return b, bad("Bitte einen Namen eintragen")
	}
	if b.Email != "" && !emailGueltig(b.Email) {
		return b, bad("Die E-Mail-Adresse »" + b.Email + "« sieht nicht richtig aus (Beispiel: name@beispiel.de)")
	}
	if _, err := parseDate(b.Seit); err != nil {
		return b, bad("Bitte das Datum der Anmeldung eintragen")
	}
	if b.BestaetigtAm != "" {
		if _, err := parseDate(b.BestaetigtAm); err != nil {
			return b, bad("Das Datum der Bestätigung ist ungültig")
		}
	}
	return b, nil
}

func sortiereWarteliste(l []Bewerber) {
	sort.SliceStable(l, func(i, j int) bool {
		if l[i].Seit != l[j].Seit {
			return l[i].Seit < l[j].Seit
		}
		return strings.ToLower(l[i].Name) < strings.ToLower(l[j].Name)
	})
}

type bewerberResp struct {
	Bewerber
	Platz      int  `json:"platz"`
	Nachfragen bool `json:"nachfragen"`
}

func (a *App) handleWarteliste(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	l := append([]Bewerber{}, a.st.d.Warteliste...)
	a.st.mu.Unlock()
	sortiereWarteliste(l)
	heute := time.Now()
	out := make([]bewerberResp, 0, len(l))
	for i, b := range l {
		out = append(out, bewerberResp{Bewerber: b, Platz: i + 1, Nachfragen: b.nachfragen(heute)})
	}
	writeJSON(w, 200, out)
}

func (a *App) handleBewerberCreate(w http.ResponseWriter, r *http.Request) {
	var in Bewerber
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if strings.TrimSpace(in.Seit) == "" {
		in.Seit = time.Now().Format("2006-01-02")
	}
	in, err := cleanBewerber(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if len(a.st.d.Warteliste) >= 1000 {
		writeErr(w, bad("Die Warteliste ist voll (höchstens 1000 Einträge)"))
		return
	}
	in.ID = newID()
	a.st.d.Warteliste = append(a.st.d.Warteliste, in)
	sortiereWarteliste(a.st.d.Warteliste)
	a.st.audit("Warteliste: Eintrag hinzugefügt")
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, in)
}

func (a *App) handleBewerberUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in Bewerber
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in, err := cleanBewerber(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Warteliste {
		if a.st.d.Warteliste[i].ID == id {
			in.ID = id
			a.st.d.Warteliste[i] = in
			sortiereWarteliste(a.st.d.Warteliste)
			a.st.audit("Warteliste: Eintrag geändert")
			if err := a.st.saveLocked(); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, in)
			return
		}
	}
	writeErr(w, notFound("Eintrag nicht gefunden"))
}

// entferneBewerber nimmt einen Eintrag von der Warteliste. Wirkt auf d, auch für Sicherungen.
func (d *Data) entferneBewerber(id string) bool {
	for i := range d.Warteliste {
		if d.Warteliste[i].ID == id {
			d.Warteliste = append(d.Warteliste[:i], d.Warteliste[i+1:]...)
			return true
		}
	}
	return false
}

// handleBewerberDelete löscht einen Eintrag samt seinen Spuren in den Sicherungen. Im Protokoll steht
// bewusst kein Name.
func (a *App) handleBewerberDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if !a.st.d.entferneBewerber(id) {
		writeErr(w, notFound("Eintrag nicht gefunden"))
		return
	}
	a.st.audit("Warteliste: Eintrag gelöscht")
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	sicherungen := a.st.bereinigeSicherungen(func(d *Data) bool { return d.entferneBewerber(id) })
	writeJSON(w, 200, map[string]int{"sicherungenBereinigt": sicherungen})
}
