package main

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"
)

// Barkasse: Kassenbuch für Bargeld (Sommerfest, Getränke, Pfand, kleine Einkäufe). Jede Buchung bekommt eine
// fortlaufende Belegnummer je Jahr, unter der der Papierbeleg abgeheftet wird. Der Bestand erscheint im
// Kassenbericht; die Kassenprüfung vergleicht ihn mit dem gezählten Bargeld.

type Barbuchung struct {
	ID        string  `json:"id"`
	Datum     string  `json:"datum"`
	Text      string  `json:"text"`
	Kategorie string  `json:"kategorie,omitempty"`
	Betrag    float64 `json:"betrag"` // positiv = Einnahme, negativ = Ausgabe
	BelegNr   int     `json:"belegNr"`
}

type barkasseAnsicht struct {
	Jahr      int          `json:"jahr"`
	Anfang    float64      `json:"anfang"`
	AnfangVJ  bool         `json:"anfangVorjahr"` // Anfangsbestand automatisch aus dem Vorjahr
	Einnahmen float64      `json:"einnahmen"`
	Ausgaben  float64      `json:"ausgaben"`
	Bestand   float64      `json:"bestand"`
	Buchungen []Barbuchung `json:"buchungen"`
	// Laufend: Bestand nach jeder Buchung (gleiche Reihenfolge wie Buchungen)
	Laufend []float64 `json:"laufend"`
}

const maxBarbuchungen = 3000

// barkasseLocked rechnet ein Jahr aus. Ohne eigenen Anfangsbestand gilt der Endbestand des Vorjahres.
// tiefe begrenzt den Rückgriff auf Vorjahre. Der Aufrufer hält s.mu.
func (s *Store) barkasseLocked(jahr int, tiefe int) barkasseAnsicht {
	a := barkasseAnsicht{Jahr: jahr, Buchungen: []Barbuchung{}, Laufend: []float64{}}
	j := s.d.Jahre[yearKey(jahr)]
	switch {
	case j != nil && j.BarAnfang != nil:
		a.Anfang = *j.BarAnfang
	case tiefe > 0:
		if vj := s.d.Jahre[yearKey(jahr-1)]; vj != nil {
			a.Anfang, a.AnfangVJ = s.barkasseLocked(jahr-1, tiefe-1).Bestand, true
		}
	}
	if j != nil {
		a.Buchungen = append(a.Buchungen, j.Barkasse...)
	}
	sort.SliceStable(a.Buchungen, func(x, y int) bool {
		if a.Buchungen[x].Datum != a.Buchungen[y].Datum {
			return a.Buchungen[x].Datum < a.Buchungen[y].Datum
		}
		return a.Buchungen[x].BelegNr < a.Buchungen[y].BelegNr
	})
	bestand := a.Anfang
	for _, b := range a.Buchungen {
		if b.Betrag >= 0 {
			a.Einnahmen += b.Betrag
		} else {
			a.Ausgaben -= b.Betrag
		}
		bestand = round2(bestand + b.Betrag)
		a.Laufend = append(a.Laufend, bestand)
	}
	a.Einnahmen, a.Ausgaben, a.Bestand = round2(a.Einnahmen), round2(a.Ausgaben), bestand
	return a
}

// barkasseAktivLocked: wird die Barkasse in diesem Jahr (oder davor) geführt?
func (s *Store) barkasseAktivLocked(jahr int) bool {
	for y, j := range s.d.Jahre {
		if j != nil && (len(j.Barkasse) > 0 || j.BarAnfang != nil) && y <= yearKey(jahr) {
			return true
		}
	}
	return false
}

func (a *App) handleBarkasse(w http.ResponseWriter, r *http.Request) {
	jahr := a.yearParam(r)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if _, ok := a.st.viewLocked(jahr); !ok {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	writeJSON(w, 200, a.st.barkasseLocked(jahr, 20))
}

func (a *App) handleBarbuchung(w http.ResponseWriter, r *http.Request) {
	jahr := a.yearParam(r)
	var in Barbuchung
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in.Text, in.Kategorie = trim(in.Text, 200), trim(in.Kategorie, 60)
	if in.Text == "" || !gueltigesDatum(in.Datum, false) || in.Betrag == 0 || !validNum(math.Abs(in.Betrag)) || math.Abs(in.Betrag) > 1e6 {
		writeErr(w, bad("Bitte Datum, Text und einen Betrag eintragen (Ausgaben mit Minus)"))
		return
	}
	in.Betrag = round2(in.Betrag)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if _, ok := a.st.viewLocked(jahr); !ok {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	j := a.st.ensureYear(jahr)
	for i := range j.Barkasse {
		if in.ID != "" && j.Barkasse[i].ID == in.ID {
			in.BelegNr = j.Barkasse[i].BelegNr // die Belegnummer bleibt, sie steht auf dem Papierbeleg
			j.Barkasse[i] = in
			a.st.audit("Barkasse %d: Beleg %d geändert", jahr, in.BelegNr)
			a.speichernUndAntworten(w, a.st.barkasseLocked(jahr, 20))
			return
		}
	}
	if len(j.Barkasse) >= maxBarbuchungen {
		writeErr(w, bad("Zu viele Buchungen in der Barkasse"))
		return
	}
	in.BelegNr = j.BarLetzteNr
	for _, b := range j.Barkasse {
		in.BelegNr = max(in.BelegNr, b.BelegNr)
	}
	in.ID, in.BelegNr = newID(), in.BelegNr+1
	j.BarLetzteNr = in.BelegNr
	j.Barkasse = append(j.Barkasse, in)
	a.st.audit("Barkasse %d: Beleg %d %s %s", jahr, in.BelegNr, in.Text, fmtEUR(in.Betrag))
	a.speichernUndAntworten(w, a.st.barkasseLocked(jahr, 20))
}

func (a *App) handleBarbuchungLoeschen(w http.ResponseWriter, r *http.Request) {
	jahr := a.yearParam(r)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	j := a.st.d.Jahre[yearKey(jahr)]
	if j != nil {
		for i, b := range j.Barkasse {
			if b.ID == r.PathValue("id") {
				j.Barkasse = append(j.Barkasse[:i], j.Barkasse[i+1:]...)
				a.st.audit("Barkasse %d: Beleg %d gelöscht", jahr, b.BelegNr)
				a.speichernUndAntworten(w, a.st.barkasseLocked(jahr, 20))
				return
			}
		}
	}
	writeErr(w, notFound("Buchung nicht gefunden"))
}

// handleBarAnfang setzt den Anfangsbestand; null = automatisch aus dem Vorjahr.
func (a *App) handleBarAnfang(w http.ResponseWriter, r *http.Request) {
	jahr := a.yearParam(r)
	var in struct {
		Betrag *float64 `json:"betrag"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if in.Betrag != nil && (!validNum(*in.Betrag) || *in.Betrag > 1e6) {
		writeErr(w, bad("Der Anfangsbestand muss eine Zahl ab 0 sein"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if _, ok := a.st.viewLocked(jahr); !ok {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	j := a.st.ensureYear(jahr)
	if in.Betrag != nil {
		v := round2(*in.Betrag)
		j.BarAnfang = &v
	} else {
		j.BarAnfang = nil
	}
	a.st.audit("Barkasse %d: Anfangsbestand geändert", jahr)
	a.speichernUndAntworten(w, a.st.barkasseLocked(jahr, 20))
}

func (a *App) kassenbuchPDF(r *http.Request) ([]byte, string, error) {
	jahr := a.yearParam(r)
	a.st.mu.Lock()
	s := a.st.d.Settings
	b := a.st.barkasseLocked(jahr, 20)
	a.st.mu.Unlock()
	if len(b.Buchungen) == 0 {
		return nil, "", bad("In diesem Jahr gibt es keine Buchungen in der Barkasse")
	}
	titel := fmt.Sprintf("Kassenbuch Barkasse %d", jahr)
	zeilen := [][]string{{"", fmt.Sprintf("01.01.%d", jahr), "Anfangsbestand", "", "", "", fmtEUR(b.Anfang)}}
	for i, x := range b.Buchungen {
		ein, aus := "", ""
		if x.Betrag >= 0 {
			ein = fmtEUR(x.Betrag)
		} else {
			aus = fmtEUR(-x.Betrag)
		}
		zeilen = append(zeilen, []string{fmt.Sprint(x.BelegNr), germanDate(x.Datum), x.Text, x.Kategorie, ein, aus, fmtEUR(b.Laufend[i])})
	}
	zeilen = append(zeilen, []string{"", "", "Summe / Endbestand", "", fmtEUR(b.Einnahmen), fmtEUR(b.Ausgaben), fmtEUR(b.Bestand)})
	pdf := neuesPDF(s, titel, true)
	pdfListe(pdf, s, titel, "Belege sind unter der Belegnummer abgeheftet. Ausgaben mindern, Einnahmen erhöhen den Bestand.",
		[]string{"Beleg", "Datum", "Buchungstext", "Kategorie", "Einnahme", "Ausgabe", "Bestand"}, []float64{16, 24, 92, 40, 28, 28, 29}, zeilen, 9)
	out, err := pdfBytes(pdf)
	return out, fmt.Sprintf("Kassenbuch-Barkasse-%d.pdf", jahr), err
}

// barkasseAufgabenLocked: ein negativer Bestand ist bei Bargeld unmöglich und deutet auf einen Buchungsfehler hin.
func (s *Store) barkasseAufgabenLocked(_ time.Time, add func(stufe, titel, text, ziel, knopf string)) {
	jahr := s.d.Settings.Jahr
	if !s.barkasseAktivLocked(jahr) {
		return
	}
	if b := s.barkasseLocked(jahr, 20); b.Bestand < 0 {
		add("warn", "Barkasse im Minus", fmt.Sprintf("Laut Kassenbuch wären %s in der Barkasse – das geht bei Bargeld nicht. Fehlt eine Einnahme oder ist eine Ausgabe doppelt?", fmtEUR(b.Bestand)),
			"admin:kassenbericht", "Barkasse")
	}
}
