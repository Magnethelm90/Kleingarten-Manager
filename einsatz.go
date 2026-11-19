package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
)

// Stunden aus Arbeitseinsätzen: Zu einem Termin im Jahresplaner wird festgehalten, wer wie viele Stunden geleistet
// hat. Die Stunden werden auf die Gemeinschaftsstunden des Pächters im gewählten Abrechnungsjahr gebucht. Ändert
// man die Eintragung später, wird nur der Unterschied gebucht, damit nichts doppelt zählt; von Hand eingetragene
// Stunden bleiben unberührt. Die Eintragungen stehen nicht im öffentlichen Jahresplaner, sondern nur hier.

// EinsatzStunden ist die Eintragung eines Pächters zu einem Arbeitseinsatz.
type EinsatzStunden struct {
	TerminID   string  `json:"terminId"`
	Jahr       int     `json:"jahr"` // Abrechnungsjahr, auf das gebucht wurde
	PaechterID string  `json:"paechterId"`
	Stunden    float64 `json:"stunden"`
}

type einsatzZeile struct {
	PaechterID  string   `json:"paechterId"`
	Gartennr    string   `json:"gartennr"`
	Mitgliedsnr string   `json:"mitgliedsnr"`
	Name        string   `json:"name"`
	Stunden     *float64 `json:"stunden"` // bei diesem Einsatz (leer = nicht dabei)
	Gesamt      float64  `json:"gesamt"`  // Stunden im Abrechnungsjahr insgesamt
}

type einsatzAnsicht struct {
	Termin  Termin         `json:"termin"`
	Jahr    int            `json:"jahr"`
	Jahre   []int          `json:"jahre"` // offene Abrechnungsjahre zur Auswahl
	Gebucht bool           `json:"gebucht"`
	Dauer   float64        `json:"dauer"` // Vorschlag aus Beginn und Ende des Termins
	Pflicht float64        `json:"pflicht"`
	Zeilen  []einsatzZeile `json:"zeilen"`
}

const maxEinsatzStunden = 24

// terminDauer schlägt die Stunden aus Beginn und Ende vor (z. B. 09:00–13:00 → 4), sonst 0.
func terminDauer(t Termin) float64 {
	if t.Von == "" || t.Bis == "" {
		return 0
	}
	min := func(s string) int {
		h, _ := strconv.Atoi(s[:2])
		m, _ := strconv.Atoi(s[3:])
		return h*60 + m
	}
	return round2(float64(min(t.Bis)-min(t.Von)) / 60)
}

// offeneJahreLocked liefert die nicht abgeschlossenen Abrechnungsjahre, aufsteigend.
func (s *Store) offeneJahreLocked() []int {
	var out []int
	for k, j := range s.d.Jahre {
		if y, err := strconv.Atoi(k); err == nil && !j.Abgeschlossen {
			out = append(out, y)
		}
	}
	sort.Ints(out)
	return out
}

// einsatzJahrLocked bestimmt das Abrechnungsjahr: bereits gebucht → dieses; sonst das Jahr des Termins, wenn es
// offen ist; sonst das aktuelle Abrechnungsjahr.
func (s *Store) einsatzJahrLocked(t Termin) (int, bool) {
	for _, e := range s.d.Einsatzstunden {
		if e.TerminID == t.ID {
			return e.Jahr, true
		}
	}
	y, _ := strconv.Atoi(t.Datum[:4])
	if j := s.d.Jahre[yearKey(y)]; j != nil && !j.Abgeschlossen {
		return y, false
	}
	return s.d.Settings.Jahr, false
}

func (s *Store) einsatzAnsichtLocked(t Termin, jahr int) (einsatzAnsicht, error) {
	gebuchtJahr, gebucht := s.einsatzJahrLocked(t)
	if jahr == 0 {
		jahr = gebuchtJahr
	}
	v, ok := s.viewLocked(jahr)
	if !ok {
		return einsatzAnsicht{}, notFound(fmt.Sprintf("Das Abrechnungsjahr %d gibt es nicht", jahr))
	}
	out := einsatzAnsicht{Termin: t, Jahr: jahr, Jahre: s.offeneJahreLocked(), Gebucht: gebucht, Dauer: terminDauer(t), Pflicht: v.Settings.Pflichtstunden, Zeilen: []einsatzZeile{}}
	hier := map[string]float64{}
	for _, e := range s.d.Einsatzstunden {
		if e.TerminID == t.ID {
			hier[e.PaechterID] = e.Stunden
		}
	}
	for _, p := range v.Paechter {
		if p.Geloescht || p.AusgeschiedenAm != "" {
			continue
		}
		z := einsatzZeile{PaechterID: p.ID, Gartennr: p.Gartennr, Mitgliedsnr: p.Mitgliedsnr, Name: p.Name}
		if st, ok := hier[p.ID]; ok {
			z.Stunden = &st
		}
		if a := v.Ablesungen[p.ID]; a.Stunden != nil {
			z.Gesamt = *a.Stunden
		}
		out.Zeilen = append(out.Zeilen, z)
	}
	sort.SliceStable(out.Zeilen, func(i, j int) bool {
		return naturalLess(out.Zeilen[i].Gartennr+" "+out.Zeilen[i].Mitgliedsnr, out.Zeilen[j].Gartennr+" "+out.Zeilen[j].Mitgliedsnr)
	})
	return out, nil
}

func (a *App) einsatzTermin(id string) (Termin, error) {
	for _, t := range a.st.d.Termine {
		if t.ID == id {
			if t.Art != "Arbeitseinsatz" {
				return t, bad("Stunden lassen sich nur bei Terminen der Art »Arbeitseinsatz« eintragen")
			}
			return t, nil
		}
	}
	return Termin{}, notFound("Termin nicht gefunden")
}

func (a *App) handleEinsatzAnsicht(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	t, err := a.einsatzTermin(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	jahr, _ := strconv.Atoi(r.URL.Query().Get("jahr"))
	v, err := a.st.einsatzAnsichtLocked(t, jahr)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, v)
}

// handleEinsatzSpeichern bucht die Stunden eines Arbeitseinsatzes. Auf die Gemeinschaftsstunden der Pächter wird
// nur der Unterschied zur bisherigen Eintragung gebucht (auch beim Wechsel des Abrechnungsjahres).
func (a *App) handleEinsatzSpeichern(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Jahr      int `json:"jahr"`
		Eintraege []struct {
			PaechterID string  `json:"paechterId"`
			Stunden    float64 `json:"stunden"`
		} `json:"eintraege"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	t, err := a.einsatzTermin(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	neuJahr := a.st.d.Jahre[yearKey(in.Jahr)]
	if neuJahr == nil || neuJahr.Abgeschlossen {
		writeErr(w, bad("Bitte ein offenes Abrechnungsjahr wählen"))
		return
	}
	aktiv := map[string]bool{}
	if v, ok := a.st.viewLocked(in.Jahr); ok {
		for _, p := range v.Paechter {
			if !p.Geloescht && p.AusgeschiedenAm == "" {
				aktiv[p.ID] = true
			}
		}
	}
	neu := map[string]float64{}
	for _, e := range in.Eintraege {
		if !aktiv[e.PaechterID] {
			writeErr(w, bad("Unbekannter Pächter in der Liste – bitte die Seite neu laden"))
			return
		}
		if !validNum(e.Stunden) || e.Stunden < 0 || e.Stunden > maxEinsatzStunden {
			writeErr(w, bad(fmt.Sprintf("Stunden bitte zwischen 0 und %d eintragen", maxEinsatzStunden)))
			return
		}
		if e.Stunden > 0 {
			neu[e.PaechterID] = round2(e.Stunden)
		}
	}
	// erst prüfen, dann ändern: nichts darf halb gebucht im Speicher stehen bleiben
	for _, e := range a.st.d.Einsatzstunden {
		if j := a.st.d.Jahre[yearKey(e.Jahr)]; e.TerminID == t.ID && j != nil && j.Abgeschlossen {
			writeErr(w, bad(fmt.Sprintf("Die Stunden dieses Einsatzes stehen im abgeschlossenen Jahr %d und lassen sich nicht mehr ändern", e.Jahr)))
			return
		}
	}
	// alte Buchungen dieses Einsatzes zurücknehmen, dann neu buchen
	var rest []EinsatzStunden
	for _, e := range a.st.d.Einsatzstunden {
		if e.TerminID != t.ID {
			rest = append(rest, e)
			continue
		}
		bucheStunden(a.st.d.Jahre[yearKey(e.Jahr)], e.PaechterID, -e.Stunden)
	}
	ids := make([]string, 0, len(neu))
	for id := range neu {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	summe := 0.0
	for _, id := range ids {
		bucheStunden(neuJahr, id, neu[id])
		rest = append(rest, EinsatzStunden{TerminID: t.ID, Jahr: in.Jahr, PaechterID: id, Stunden: neu[id]})
		summe += neu[id]
	}
	a.st.d.Einsatzstunden = rest
	a.st.audit("Arbeitseinsatz %s am %s: %d Teilnehmer, %s Stunden gebucht auf %d", t.Titel, germanDate(t.Datum), len(ids), fmtFlex(summe), in.Jahr)
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"teilnehmer": len(ids), "stunden": round2(summe)})
}

// bucheStunden ändert die Gemeinschaftsstunden eines Pächters um delta (nie unter 0).
func bucheStunden(j *Jahr, pid string, delta float64) {
	if j == nil || delta == 0 {
		return
	}
	if j.Ablesungen == nil {
		j.Ablesungen = map[string]Ablesung{}
	}
	a := j.Ablesungen[pid]
	v := delta
	if a.Stunden != nil {
		v += *a.Stunden
	}
	v = round2(v)
	if v <= 0 && delta < 0 {
		a.Stunden = nil // zurückgenommen: wieder »nicht eingetragen« statt einer ausdrücklichen 0
	} else {
		a.Stunden = &v
	}
	j.Ablesungen[pid] = a
}

func (a *App) stundenlistePDF(r *http.Request) ([]byte, string, error) {
	a.st.mu.Lock()
	t, err := a.einsatzTermin(r.PathValue("id"))
	l := a.st.anwesenheitLocked()
	s := a.st.d.Settings
	a.st.mu.Unlock()
	if err != nil {
		return nil, "", err
	}
	pdf, err := buildAnwesenheit(s, t, l, true)
	return pdf, safeName("Stundenliste_"+t.Titel+"_"+t.Datum, 80) + ".pdf", err
}

// einsatzTerminGeloescht entfernt die Eintragungen eines gelöschten Termins; die gebuchten Stunden bleiben
// bei den Pächtern, denn der Einsatz hat ja stattgefunden.
func (d *Data) einsatzTerminGeloescht(id string) {
	rest := d.Einsatzstunden[:0]
	for _, e := range d.Einsatzstunden {
		if e.TerminID != id {
			rest = append(rest, e)
		}
	}
	d.Einsatzstunden = rest
}
