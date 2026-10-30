package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Beispieldaten: Wer das Programm zum ersten Mal öffnet, kann es mit einem erfundenen Verein
// ausprobieren, ohne eigene Daten einzugeben. Beispieldaten lassen sich nur in einen leeren
// Datenbestand laden und mit einem Klick wieder vollständig entfernen.

// datenbestandLeer meldet, ob noch nichts eingegeben wurde (keine Pächter, Rechnungen, Ausgaben,
// kein abgeschlossenes Jahr). Der Aufrufer hält s.mu.
func (s *Store) datenbestandLeer() bool {
	if len(s.d.Paechter) > 0 || len(s.d.Rechnungen) > 0 || len(s.d.Jahre) > 1 {
		return false
	}
	for _, j := range s.d.Jahre {
		if j.Abgeschlossen || len(j.Ablesungen) > 0 || len(j.Ausgaben) > 0 || j.Versorger != nil || j.Anfangsbestand != nil {
			return false
		}
	}
	return true
}

type demoPaechter struct {
	anrede, name, strasse string
	groesse               float64
	wasser, strom         [2]float64 // Vorjahr, aktuell (aktuell < 0: noch nicht abgelesen)
	stunden               float64    // < 0: noch nicht eingetragen
	post                  bool
	jahreDabei            int // Pächter seit so vielen Jahren (nur einer mit rundem Jubiläum)
}

var demoListe = []demoPaechter{
	{"Frau", "Brigitte Sommer", "Lindenweg 3", 312, [2]float64{148, 191}, [2]float64{2136, 2302}, 14, false, 23},
	{"Herr", "Klaus Winter", "Am Bach 12", 285, [2]float64{96, 121}, [2]float64{1480, 1555}, 12, true, 12},
	{"Familie", "Yılmaz", "Rosenstraße 7", 410, [2]float64{233, 301}, [2]float64{3105, 3390}, 24, false, 15},
	{"Frau", "Ingrid Herbst", "Gartenstraße 1", 260, [2]float64{55, 70}, [2]float64{805, 851}, 6, true, 31},
	{"Herr", "Tobias Frühling", "Ahornallee 22", 330, [2]float64{120, 158}, [2]float64{1990, 2120}, 18, false, 4},
	{"Frau", "Agnieszka Nowak", "Birkenweg 5", 298, [2]float64{77, 102}, [2]float64{1210, 1288}, 12, false, 19},
	{"Herr", "Peter Grün", "Lindenweg 9", 355, [2]float64{180, 236}, [2]float64{2650, 2805}, 10, false, 9},
	{"Frau", "Sabine Blume", "Am Bach 4", 275, [2]float64{64, 88}, [2]float64{990, 1043}, 16, false, 6},
	{"Herr und Frau", "Becker", "Kastanienweg 18", 390, [2]float64{205, 260}, [2]float64{2870, 3040}, 20, true, 16},
	{"Herr", "Mehmet Demir", "Rosenstraße 11", 305, [2]float64{110, 143}, [2]float64{1655, 1749}, 12, false, 2},
	{"Frau", "Helga Wiese", "Birkenweg 2", 268, [2]float64{90, -1}, [2]float64{1300, -1}, 12, false, 21},
	{"Herr", "Jan Sonnenberg", "Ahornallee 3", 320, [2]float64{132, 170}, [2]float64{2010, 2133}, -1, false, 1},
}

// ladeBeispieldaten füllt einen leeren Datenbestand mit einem erfundenen Verein. Der Aufrufer hält s.mu.
func (s *Store) ladeBeispieldaten() {
	set := &s.d.Settings
	set.VereinName = "Kleingartenverein Sonnenschein e. V."
	set.Ort = "12345 Musterstadt"
	set.Absenderzeile = "Kleingartenverein Sonnenschein e. V. – Kassenwart – kasse@example.org"
	// abgerechnet wird das Vorjahr, ausgestellt heute mit vier Wochen Zahlungsziel: so ist am Anfang
	// noch nichts überfällig, egal wann die Beispieldaten geladen werden
	heute := time.Now()
	jahr := heute.Year() - 1
	if set.Jahr != jahr {
		delete(s.d.Jahre, yearKey(set.Jahr))
		set.Jahr = jahr
	}
	set.Rechnungsdatum = heute.Format("2006-01-02")
	set.Zahlungsziel = heute.AddDate(0, 0, 28).Format("2006-01-02")
	j := s.ensureYear(jahr)
	for i, x := range demoListe {
		id := newID()
		versand := "Emailsendung"
		if x.post {
			versand = "Postversand"
		}
		p := Paechter{
			ID: id, Mitgliedsnr: fmt.Sprintf("%d-%02d", i+1, (heute.Year()-x.jahreDabei)%100), Gartennr: fmt.Sprint(i + 1), Anrede: x.anrede, Name: x.name,
			Strasse: x.strasse, PLZOrt: set.Ort, Versand: versand, Gartengroesse: x.groesse,
			WasserzaehlerNr: fmt.Sprintf("W-%04d", 1001+i), StromzaehlerNr: fmt.Sprintf("S-%04d", 2001+i),
		}
		if !x.post {
			p.Email = fmt.Sprintf("garten%d@example.org", i+1)
		}
		s.d.Paechter = append(s.d.Paechter, p)
		a := Ablesung{WasserVJ: fp(x.wasser[0]), StromVJ: fp(x.strom[0])}
		if x.wasser[1] >= 0 {
			a.WasserAkt = fp(x.wasser[1])
		}
		if x.strom[1] >= 0 {
			a.StromAkt = fp(x.strom[1])
		}
		if x.stunden >= 0 {
			a.Stunden = fp(x.stunden)
		}
		j.Ablesungen[id] = a
		s.d.GartenHistorie = append(s.d.GartenHistorie, GartenEintrag{
			Gartennr: p.Gartennr, PaechterID: id, Mitgliedsnr: p.Mitgliedsnr, Name: p.Name, Seit: fmt.Sprintf("%d-04-01", heute.Year()-x.jahreDabei),
		})
	}
	bestand := 2480.0
	j.Anfangsbestand = &bestand
	j.Ausgaben = []Ausgabe{
		{ID: newID(), Datum: fmt.Sprintf("%d-03-31", jahr), Beschreibung: "Kontoführung 1. Quartal", Kategorie: "Versicherung & Gebühren", Betrag: 15, Wiederkehrend: true},
		{ID: newID(), Datum: fmt.Sprintf("%d-05-14", jahr), Beschreibung: "Reparatur Vereinsrasenmäher", Kategorie: "Instandhaltung", Betrag: 145.5},
		{ID: newID(), Datum: fmt.Sprintf("%d-09-02", jahr), Beschreibung: "Druckerpapier und Briefumschläge", Kategorie: "Verwaltung", Betrag: 18.99},
	}
	// Lageplan: zwei Reihen Gärten links und rechts eines Weges, wie auf einem kleinen Vereinsgelände
	s.d.Lageplan = map[string]LagePos{}
	for i := range demoListe {
		x, y := 20+(i/2)*140, 20
		if i%2 == 1 {
			y = 160
		}
		s.d.Lageplan[fmt.Sprint(i+1)] = LagePos{X: x, Y: y}
	}
	s.d.Demo = true
	s.audit("Beispieldaten geladen")
}

func fp(x float64) *float64 { return &x }

// entferneBeispieldaten setzt den Datenbestand auf den Anfang zurück und löscht die in der Zwischenzeit
// erzeugten Rechnungs- und Belegdateien. Passwörter, der zweite Sicherungsordner und die Einführung
// bleiben, wie sie sind. Der Aufrufer hält s.mu.
func (s *Store) entferneBeispieldaten() {
	for _, r := range s.d.Rechnungen {
		s.entferneDatei("Rechnungen", r.Datei)
	}
	for _, j := range s.d.Jahre {
		for _, a := range j.Ausgaben {
			for _, b := range a.Belege {
				s.entferneDatei("Belege", b)
			}
		}
	}
	alt := s.d
	s.d = newData()
	s.d.Admin, s.d.PruefAuth, s.d.TutorialGesehen = alt.Admin, alt.PruefAuth, alt.TutorialGesehen
	s.d.Settings.ZweiteSicherung = alt.Settings.ZweiteSicherung
	s.audit("Beispieldaten entfernt")
	s.leereDruckOrdner()
}

// entferneDatei löscht eine gewöhnliche Datei, deren gespeicherter relativer Pfad im Unterordner
// ordner des Datenordners liegt (der Pfad stammt aus der Datendatei und wird deshalb geprüft).
func (s *Store) entferneDatei(ordner, rel string) {
	if rel == "" {
		return
	}
	base := filepath.Join(s.dir, ordner) + string(filepath.Separator)
	full := filepath.Join(s.dir, filepath.FromSlash(rel))
	if len(full) <= len(base) || full[:len(base)] != base {
		return
	}
	if fi, err := os.Lstat(full); err == nil && fi.Mode().IsRegular() {
		_ = os.Remove(full)
	}
}

func (a *App) handleDemoLaden(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if !a.st.datenbestandLeer() {
		writeErr(w, bad("Beispieldaten lassen sich nur laden, solange noch keine eigenen Daten eingegeben sind"))
		return
	}
	a.st.ladeBeispieldaten()
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]int{"paechter": len(a.st.d.Paechter)})
}

func (a *App) handleDemoEntfernen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if !a.st.d.Demo {
		writeErr(w, bad("Es sind keine Beispieldaten geladen"))
		return
	}
	a.st.snapshotBackup("vor-Entfernen-der-Beispieldaten")
	a.st.entferneBeispieldaten()
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"stand": time.Now().Format(time.RFC3339)})
}
