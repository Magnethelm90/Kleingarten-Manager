package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// sicherungInfo beschreibt eine Sicherungsdatei für die Auswahl im Programm.
type sicherungInfo struct {
	Name       string `json:"name"`
	Zeit       string `json:"zeit"`    // RFC3339 (Änderungszeit der Datei)
	Art        string `json:"art"`     // "Tagessicherung" oder Anlass, z. B. "vor-Loeschen"
	Groesse    int64  `json:"groesse"` // Bytes
	Lesbar     bool   `json:"lesbar"`  // lässt sich einlesen und wiederherstellen
	Hinweis    string `json:"hinweis,omitempty"`
	Paechter   int    `json:"paechter"`
	Rechnungen int    `json:"rechnungen"`
	Jahre      int    `json:"jahre"`
}

// maxAngezeigteSicherungen begrenzt die Liste auf die neuesten Dateien (jede muss zur Anzeige gelesen werden).
const maxAngezeigteSicherungen = 40

// sicherungsNamen liefert die Namen aller Sicherungsdateien im Sicherungsordner, neueste zuerst.
func (s *Store) sicherungsNamen() []string {
	entries, err := os.ReadDir(s.backupDir())
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if e.Type().IsRegular() && strings.HasSuffix(n, ".json") &&
			(strings.HasPrefix(n, backupPrefix) || strings.HasPrefix(n, legacyBackupPrefix)) {
			names = append(names, n)
		}
	}
	// Dateiname beginnt nach dem Präfix mit Datum (und Uhrzeit): lexikografisch sortiert = zeitlich
	sort.Slice(names, func(i, j int) bool { return names[i] > names[j] })
	return names
}

func sicherungArt(name string) string {
	n := strings.TrimSuffix(name, ".json")
	n = strings.TrimPrefix(strings.TrimPrefix(n, backupPrefix), legacyBackupPrefix)
	// JJJJ-MM-TT oder JJJJ-MM-TT_HHMMSS-anlass
	if len(n) > 18 && n[10] == '_' {
		if i := strings.Index(n[11:], "-"); i >= 0 {
			return strings.ReplaceAll(n[11+i+1:], "-", " ")
		}
	}
	return "Tagessicherung"
}

// sicherungen liest die neuesten Sicherungen und prüft, ob sie sich wiederherstellen lassen. Die
// Sicherungen vor dem Jahreswechsel erscheinen immer, auch wenn sie älter als die übrigen sind.
func (s *Store) sicherungen() []sicherungInfo {
	alle := s.sicherungsNamen()
	names := alle
	if len(alle) > maxAngezeigteSicherungen {
		names = append([]string(nil), alle[:maxAngezeigteSicherungen]...)
		for _, n := range alle[maxAngezeigteSicherungen:] {
			if isJahresSicherung(n) {
				names = append(names, n)
			}
		}
	}
	out := make([]sicherungInfo, 0, len(names))
	for _, n := range names {
		full := filepath.Join(s.backupDir(), n)
		info := sicherungInfo{Name: n, Art: sicherungArt(n)}
		if fi, err := os.Stat(full); err == nil {
			info.Zeit, info.Groesse = fi.ModTime().Format(time.RFC3339), fi.Size()
		}
		raw, err := os.ReadFile(full)
		if err == nil {
			var d *Data
			if d, err = parseDaten(raw); err == nil {
				info.Lesbar, info.Paechter, info.Rechnungen, info.Jahre = true, len(d.Paechter), len(d.Rechnungen), len(d.Jahre)
			} else if errors.Is(err, errNeuereVersion) {
				info.Hinweis = "stammt aus einer neueren Programmversion"
			} else {
				info.Hinweis = "beschädigt"
			}
		} else {
			info.Hinweis = "nicht lesbar"
		}
		out = append(out, info)
	}
	return out
}

func (a *App) handleSicherungen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	list := a.st.sicherungen()
	a.st.mu.Unlock()
	writeJSON(w, 200, list)
}

// sicherungInhalt: was in einer Sicherung steckt – zum Ansehen, ohne etwas zu verändern.
type sicherungInhalt struct {
	Name            string      `json:"name"`
	Verein          string      `json:"verein"`
	Jahr            int         `json:"jahr"`
	Jahre           []int       `json:"jahre"`
	LetzteAenderung string      `json:"letzteAenderung,omitempty"`
	LetzteAktion    string      `json:"letzteAktion,omitempty"`
	Zahlen          [][2]string `json:"zahlen"` // Bezeichnung, Wert in der Sicherung
	Heute           [][2]string `json:"heute"`  // dieselben Zahlen im aktuellen Bestand
	Paechter        [][2]string `json:"paechter"`
}

func bestandsZahlen(d *Data) [][2]string {
	aktiv := 0
	for _, p := range d.Paechter {
		if !p.Geloescht && p.AusgeschiedenAm == "" {
			aktiv++
		}
	}
	ausgaben, ablesungen := 0, 0
	if j := d.Jahre[yearKey(d.Settings.Jahr)]; j != nil {
		ausgaben, ablesungen = len(j.Ausgaben), len(j.Ablesungen)
	}
	z := func(n int) string { return fmt.Sprint(n) }
	return [][2]string{{"Abrechnungsjahr", z(d.Settings.Jahr)}, {"Jahre im Bestand", z(len(d.Jahre))}, {"Aktive Pächter", z(aktiv)},
		{"Zählerstände im laufenden Jahr", z(ablesungen)}, {"Rechnungen im Archiv", z(len(d.Rechnungen))}, {"Ausgaben im laufenden Jahr", z(ausgaben)},
		{"Termine", z(len(d.Termine))}, {"Dokumente in Gartenakten", z(len(d.Dokumente))}}
}

// handleSicherungAnsehen öffnet eine Sicherung nur zum Lesen: So lässt sich prüfen, ob sie vollständig ist,
// ohne den heutigen Stand anzutasten.
func (a *App) handleSicherungAnsehen(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	name := ""
	for _, n := range a.st.sicherungsNamen() { // nur Namen aus dem Ordner, nie ein frei angegebener Pfad
		if n == in.Name {
			name = n
			break
		}
	}
	if name == "" {
		writeErr(w, notFound("Sicherung nicht gefunden"))
		return
	}
	raw, err := os.ReadFile(filepath.Join(a.st.backupDir(), name))
	if err != nil {
		writeErr(w, bad("Die Sicherung lässt sich nicht lesen"))
		return
	}
	d, err := parseDaten(raw)
	if err != nil {
		writeErr(w, bad("Diese Sicherung ist beschädigt oder stammt aus einer neueren Programmversion"))
		return
	}
	out := sicherungInhalt{Name: name, Verein: d.Settings.VereinName, Jahr: d.Settings.Jahr, Jahre: []int{}, Zahlen: bestandsZahlen(d), Heute: bestandsZahlen(a.st.d), Paechter: [][2]string{}}
	for k := range d.Jahre {
		if j, err := strconv.Atoi(k); err == nil {
			out.Jahre = append(out.Jahre, j)
		}
	}
	sort.Ints(out.Jahre)
	if n := len(d.AuditLog); n > 0 {
		out.LetzteAenderung, out.LetzteAktion = d.AuditLog[n-1].Zeit, d.AuditLog[n-1].Aktion
	}
	for _, p := range d.Paechter {
		if !p.Geloescht && p.AusgeschiedenAm == "" {
			out.Paechter = append(out.Paechter, [2]string{p.Gartennr, p.Name})
		}
	}
	sort.SliceStable(out.Paechter, func(i, j int) bool { return naturalLess(out.Paechter[i][0], out.Paechter[j][0]) })
	writeJSON(w, 200, out)
}

// handleSicherungWiederherstellen ersetzt den Datenbestand durch eine Sicherung. Vorher wird der aktuelle
// Stand als eigene Sicherung abgelegt, sodass sich auch dieser Schritt rückgängig machen lässt.
func (a *App) handleSicherungWiederherstellen(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	// nur Namen aus der Liste der vorhandenen Sicherungen, nie ein frei angegebener Pfad: gelesen wird mit dem
	// Namen aus dem Ordner, nicht mit dem aus der Anfrage
	name := ""
	for _, n := range a.st.sicherungsNamen() {
		if n == in.Name {
			name = n
			break
		}
	}
	if name == "" {
		writeErr(w, notFound("Sicherung nicht gefunden"))
		return
	}
	raw, err := os.ReadFile(filepath.Join(a.st.backupDir(), name))
	if err != nil {
		writeErr(w, bad("Die Sicherung lässt sich nicht lesen"))
		return
	}
	d, err := parseDaten(raw)
	switch {
	case errors.Is(err, errNeuereVersion):
		writeErr(w, bad("Diese Sicherung stammt aus einer neueren Programmversion und lässt sich hier nicht einspielen"))
		return
	case err != nil:
		writeErr(w, bad("Diese Sicherung ist beschädigt und lässt sich nicht einspielen"))
		return
	}
	a.st.snapshotBackup("vor-Wiederherstellung")
	a.st.d = d
	a.st.zweiteSicherungGeaendert() // die Sicherung kann einen anderen zweiten Ordner enthalten
	a.st.ensureYear(d.Settings.Jahr)
	a.st.migriereBelege()
	a.st.audit("Sicherung wiederhergestellt: %s", name)
	a.st.sicherungGeprueft = ""
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	a.st.leereDruckOrdner()
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// handleSicherungJetzt legt sofort eine Sicherung an und kopiert sie in den zweiten Sicherungsordner,
// z. B. gleich nachdem der USB-Stick eingesteckt wurde.
func (a *App) handleSicherungJetzt(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if err := a.st.saveLocked(); err != nil { // aktueller Stand liegt sicher auf der Platte
		writeErr(w, err)
		return
	}
	a.st.snapshotBackup("manuell")
	writeJSON(w, 200, a.st.zweiteSicherungLocked())
}
