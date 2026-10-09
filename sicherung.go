package main

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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

// sicherungen liest die neuesten Sicherungen und prüft, ob sie sich wiederherstellen lassen.
func (s *Store) sicherungen() []sicherungInfo {
	names := s.sicherungsNamen()
	if len(names) > maxAngezeigteSicherungen {
		names = names[:maxAngezeigteSicherungen]
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
	// nur Namen aus der Liste der vorhandenen Sicherungen, nie ein frei angegebener Pfad
	gefunden := false
	for _, n := range a.st.sicherungsNamen() {
		if n == in.Name {
			gefunden = true
			break
		}
	}
	if !gefunden {
		writeErr(w, notFound("Sicherung nicht gefunden"))
		return
	}
	raw, err := os.ReadFile(filepath.Join(a.st.backupDir(), in.Name))
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
	a.st.ensureYear(d.Settings.Jahr)
	a.st.migriereBelege()
	a.st.audit("Sicherung wiederhergestellt: %s", in.Name)
	a.st.sicherungGeprueft = ""
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	a.st.leereDruckOrdner()
	writeJSON(w, 200, map[string]bool{"ok": true})
}
