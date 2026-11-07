package main

import (
	"fmt"
	"net/http"
	"sort"
	"time"
)

// Ämter: wer welches Amt seit wann und bis wann innehat. Das Cockpit erinnert rechtzeitig an fällige Wahlen,
// und beim Amtswechsel führt eine Checkliste durch alles, was übergeben werden muss.

type Amt struct {
	ID         string            `json:"id"`
	Amt        string            `json:"amt"`
	Name       string            `json:"name"`
	Seit       string            `json:"seit"`
	Bis        string            `json:"bis,omitempty"` // Ende der Amtszeit (leer = unbefristet)
	Ehemalig   bool              `json:"ehemalig,omitempty"`
	Notiz      string            `json:"notiz,omitempty"`
	Checkliste map[string]string `json:"checkliste,omitempty"` // Punkt -> erledigt am (nur nach einem Amtswechsel)
}

var amtsWechselPunkte = []struct{ Key, Text string }{
	{"protokoll", "Wahl im Protokoll und im Beschlussbuch festgehalten"},
	{"register", "Änderung beim Vereinsregister angemeldet (nur Vorstand nach § 26 BGB, über den Notar)"},
	{"bank", "Bankvollmacht umgestellt (Kasse und Vorstand)"},
	{"schluessel", "Schlüssel übergeben"},
	{"zugaenge", "Passwörter und Zugänge übergeben (auch das Passwort dieses Programms ändern)"},
	{"unterlagen", "Unterlagen und Vereinsakte übergeben"},
	{"verband", "Verband, Verpächter und Versicherungen informiert"},
}

var amtVorschlaege = []string{"Vorsitz", "Stellv. Vorsitz", "Kassenwart/in", "Schriftführer/in", "Fachberater/in", "Wasserwart/in", "Kassenprüfer/in", "Beisitzer/in"}

type aemterAntwort struct {
	Aemter      []Amt        `json:"aemter"`
	Punkte      []pruefPunkt `json:"punkte"`
	Vorschlaege []string     `json:"vorschlaege"`
}

func (a *App) aemterAntwortLocked() aemterAntwort {
	l := append([]Amt{}, a.st.d.Aemter...)
	sort.SliceStable(l, func(i, j int) bool {
		if l[i].Ehemalig != l[j].Ehemalig {
			return !l[i].Ehemalig
		}
		return l[i].Seit > l[j].Seit
	})
	p := []pruefPunkt{}
	for _, x := range amtsWechselPunkte {
		p = append(p, pruefPunkt{Key: x.Key, Text: x.Text})
	}
	return aemterAntwort{Aemter: l, Punkte: p, Vorschlaege: amtVorschlaege}
}

func (a *App) handleAemter(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	writeJSON(w, 200, a.aemterAntwortLocked())
}

func cleanAmt(in Amt) (Amt, error) {
	in.Amt, in.Name, in.Notiz = trim(in.Amt, 60), trim(in.Name, 100), trim(in.Notiz, 500)
	if in.Amt == "" || in.Name == "" {
		return in, bad("Bitte Amt und Namen eintragen")
	}
	if !gueltigesDatum(in.Seit, true) || !gueltigesDatum(in.Bis, true) || (in.Bis != "" && in.Seit != "" && in.Bis < in.Seit) {
		return in, bad("Bitte die Amtszeit prüfen")
	}
	return in, nil
}

func (a *App) handleAmtSpeichern(w http.ResponseWriter, r *http.Request) {
	var in Amt
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in, err := cleanAmt(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if i := a.amtIndex(in.ID); i >= 0 {
		alt := a.st.d.Aemter[i]
		in.Checkliste, in.Ehemalig = alt.Checkliste, alt.Ehemalig
		a.st.d.Aemter[i] = in
		a.st.audit("Amt geändert: %s (%s)", in.Amt, in.Name)
	} else {
		if len(a.st.d.Aemter) >= maxVerwaltung {
			writeErr(w, bad("Zu viele Ämter"))
			return
		}
		in.ID, in.Checkliste, in.Ehemalig = newID(), nil, false
		a.st.d.Aemter = append(a.st.d.Aemter, in)
		a.st.audit("Amt eingetragen: %s (%s)", in.Amt, in.Name)
	}
	a.speichernUndAntworten(w, a.aemterAntwortLocked())
}

func (a *App) amtIndex(id string) int {
	for i := range a.st.d.Aemter {
		if id != "" && a.st.d.Aemter[i].ID == id {
			return i
		}
	}
	return -1
}

// handleAmtWechsel: neue Person übernimmt das Amt. Das bisherige Amt endet am Tag der Wahl und wandert in die
// Historie, das neue bekommt die Übergabe-Checkliste.
func (a *App) handleAmtWechsel(w http.ResponseWriter, r *http.Request) {
	var in Amt
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	i := a.amtIndex(r.PathValue("id"))
	if i < 0 || a.st.d.Aemter[i].Ehemalig {
		writeErr(w, notFound("Amt nicht gefunden"))
		return
	}
	in.Amt = a.st.d.Aemter[i].Amt
	in, err := cleanAmt(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	if in.Seit == "" {
		writeErr(w, bad("Bitte den Tag der Wahl eintragen"))
		return
	}
	if len(a.st.d.Aemter) >= maxVerwaltung {
		writeErr(w, bad("Zu viele Ämter"))
		return
	}
	alt := &a.st.d.Aemter[i]
	if alt.Seit != "" && in.Seit < alt.Seit {
		writeErr(w, bad("Der Tag der Wahl liegt vor dem Amtsantritt von "+alt.Name))
		return
	}
	alt.Ehemalig = true
	if alt.Bis == "" || alt.Bis > in.Seit {
		alt.Bis = in.Seit
	}
	in.ID, in.Ehemalig, in.Checkliste = newID(), false, map[string]string{}
	a.st.audit("Amtswechsel %s: %s → %s", in.Amt, alt.Name, in.Name)
	a.st.d.Aemter = append(a.st.d.Aemter, in)
	a.speichernUndAntworten(w, a.aemterAntwortLocked())
}

func (a *App) handleAmtCheckliste(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Key      string `json:"key"`
		Erledigt bool   `json:"erledigt"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	gueltig := false
	for _, p := range amtsWechselPunkte {
		gueltig = gueltig || p.Key == in.Key
	}
	if !gueltig {
		writeErr(w, bad("Unbekannter Punkt"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	i := a.amtIndex(r.PathValue("id"))
	if i < 0 || a.st.d.Aemter[i].Checkliste == nil {
		writeErr(w, notFound("Amtswechsel nicht gefunden"))
		return
	}
	if in.Erledigt {
		a.st.d.Aemter[i].Checkliste[in.Key] = time.Now().Format("2006-01-02")
	} else {
		delete(a.st.d.Aemter[i].Checkliste, in.Key)
	}
	a.speichernUndAntworten(w, a.aemterAntwortLocked())
}

func (a *App) handleAmtLoeschen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	i := a.amtIndex(r.PathValue("id"))
	if i < 0 {
		writeErr(w, notFound("Amt nicht gefunden"))
		return
	}
	a.st.audit("Amt gelöscht: %s (%s)", a.st.d.Aemter[i].Amt, a.st.d.Aemter[i].Name)
	a.st.d.Aemter = append(a.st.d.Aemter[:i], a.st.d.Aemter[i+1:]...)
	a.speichernUndAntworten(w, a.aemterAntwortLocked())
}

// aemterAufgabenLocked: Amtszeit endet in 120 Tagen oder ist abgelaufen; offene Punkte nach einem Wechsel.
func (s *Store) aemterAufgabenLocked(heute time.Time, add func(stufe, titel, text, ziel, knopf string)) {
	for _, x := range s.d.Aemter {
		if x.Ehemalig {
			continue
		}
		if x.Bis != "" {
			switch n := tageBis(heute, x.Bis); {
			case n < 0:
				add("warn", fmt.Sprintf("Amtszeit %s abgelaufen", x.Amt), fmt.Sprintf("Die Amtszeit von %s endete am %s. Neuwahl ansetzen oder die Amtszeit verlängern.", x.Name, germanDate(x.Bis)), "admin:verwaltung:aemter", "Ämter")
			case n <= 120:
				add("info", fmt.Sprintf("Wahl %s vorbereiten", x.Amt), fmt.Sprintf("Die Amtszeit von %s endet am %s. Die Wahl gehört auf die Tagesordnung der nächsten Versammlung.", x.Name, germanDate(x.Bis)), "admin:verwaltung:aemter", "Ämter")
			}
		}
		if x.Checkliste != nil && len(x.Checkliste) < len(amtsWechselPunkte) && tageBis(heute, x.Seit) >= -180 {
			add("info", fmt.Sprintf("Amtsübergabe %s: %d Punkte offen", x.Amt, len(amtsWechselPunkte)-len(x.Checkliste)), "Bankvollmacht, Vereinsregister, Schlüssel und Zugänge – die Checkliste zeigt, was noch fehlt.", "admin:verwaltung:aemter", "Checkliste")
		}
	}
}
