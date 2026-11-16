package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Wiederherstellungs-Probe: Eine Sicherung, die nie getestet wurde, ist ein Risiko. Die Probe liest die jüngste
// Sicherung (und die im zweiten Ordner) wirklich ein, wie es beim Wiederherstellen geschähe, und vergleicht sie
// mit dem heutigen Bestand. Es wird nichts verändert; gespeichert wird nur, wann und mit welchem Ergebnis geprüft wurde.

// probeIntervallTage: danach erinnert das Cockpit an eine neue Probe.
const probeIntervallTage = 90

// Wiederherstellungsprobe ist das gespeicherte Ergebnis der letzten Probe.
type Wiederherstellungsprobe struct {
	Datum string `json:"datum"` // JJJJ-MM-TT
	Ok    bool   `json:"ok"`
}

type probeZeile struct {
	Titel string `json:"titel"`
	Stufe string `json:"stufe"` // ok | warn | err
	Text  string `json:"text"`
}

type probeErgebnis struct {
	Datum  string       `json:"datum"`
	Ok     bool         `json:"ok"`
	Zeilen []probeZeile `json:"zeilen"`
}

// neuesteSicherungIn liefert Name und Zeit der jüngsten Sicherungsdatei eines Ordners.
func neuesteSicherungIn(dir string) (string, time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", time.Time{}
	}
	var namen []string
	for _, e := range entries {
		n := e.Name()
		if e.Type().IsRegular() && strings.HasSuffix(n, ".json") && (strings.HasPrefix(n, backupPrefix) || strings.HasPrefix(n, legacyBackupPrefix)) {
			namen = append(namen, n)
		}
	}
	if len(namen) == 0 {
		return "", time.Time{}
	}
	sort.Strings(namen)
	n := namen[len(namen)-1]
	fi, err := os.Stat(filepath.Join(dir, n))
	if err != nil {
		return n, time.Time{}
	}
	return n, fi.ModTime()
}

func (s *Store) probeSicherungLocked(titel, dir string, jetzt time.Time) probeZeile {
	name, zeit := neuesteSicherungIn(dir)
	if name == "" {
		return probeZeile{titel, "err", "Im Ordner liegt keine Sicherung."}
	}
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return probeZeile{titel, "err", "Die Sicherung " + name + " lässt sich nicht lesen."}
	}
	d, err := parseDaten(raw)
	if err != nil {
		return probeZeile{titel, "err", "Die Sicherung " + name + " ist beschädigt oder stammt aus einer neueren Programmversion."}
	}
	alter := int(jetzt.Sub(zeit).Hours() / 24)
	text := fmt.Sprintf("Lesbar: %d Pächter, %d Rechnungen, %d Abrechnungsjahre, vom %s.", len(d.Paechter), len(d.Rechnungen), len(d.Jahre), zeit.Format("02.01.2006"))
	stufe := "ok"
	switch {
	case alter > 7:
		stufe, text = "warn", text+fmt.Sprintf(" Die Sicherung ist %d Tage alt – seitdem hat sich etwas geändert oder es wurde nicht gesichert.", alter)
	}
	return probeZeile{titel, stufe, text}
}

// probeLocked führt die Probe aus. Der Aufrufer hält s.mu.
func (s *Store) probeLocked(jetzt time.Time) probeErgebnis {
	res := probeErgebnis{Datum: jetzt.Format("2006-01-02")}
	res.Zeilen = append(res.Zeilen, s.probeSicherungLocked("Sicherungsordner im Datenordner", s.backupDir(), jetzt))
	if dir := strings.TrimSpace(s.d.Settings.ZweiteSicherung); dir != "" {
		res.Zeilen = append(res.Zeilen, s.probeSicherungLocked("Zweiter Sicherungsordner (USB-Stick)", dir, jetzt))
	} else {
		res.Zeilen = append(res.Zeilen, probeZeile{"Zweiter Sicherungsordner (USB-Stick)", "warn",
			"Nicht eingerichtet. Liegen alle Sicherungen auf demselben Rechner, helfen sie bei einem Festplattenschaden nicht."})
	}
	// Verschlüsselte Sicherung: ohne Passwort lässt sich nur die Kennung prüfen.
	tdir := filepath.Join(s.dir, tresorOrdner)
	if entries, err := os.ReadDir(tdir); err == nil {
		var neueste string
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), tresorEndung) && e.Name() > neueste {
				neueste = e.Name()
			}
		}
		if neueste != "" {
			raw, err := os.ReadFile(filepath.Join(tdir, neueste))
			if err != nil || !strings.HasPrefix(string(raw), tresorKennung) {
				res.Zeilen = append(res.Zeilen, probeZeile{"Verschlüsselte Sicherung", "err", "Die Datei " + neueste + " ist beschädigt."})
			} else {
				res.Zeilen = append(res.Zeilen, probeZeile{"Verschlüsselte Sicherung", "ok",
					"Die Datei " + neueste + " ist unbeschädigt. Ob das Passwort noch bekannt ist, lässt sich nur durch Öffnen prüfen."})
			}
		}
	}
	// Datenbestand selbst: lässt er sich so, wie er jetzt gespeichert wird, wieder einlesen?
	if raw, err := os.ReadFile(s.path); err != nil {
		res.Zeilen = append(res.Zeilen, probeZeile{"Aktueller Datenbestand", "err", "Die Datendatei lässt sich nicht lesen."})
	} else if _, err := parseDaten(raw); err != nil {
		res.Zeilen = append(res.Zeilen, probeZeile{"Aktueller Datenbestand", "err", "Die Datendatei ist beschädigt."})
	} else {
		res.Zeilen = append(res.Zeilen, probeZeile{"Aktueller Datenbestand", "ok", "Die gespeicherte Datendatei lässt sich einlesen."})
	}
	res.Ok = true
	for _, z := range res.Zeilen {
		if z.Stufe == "err" {
			res.Ok = false
		}
	}
	return res
}

func (a *App) handleProbe(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	res := a.st.probeLocked(time.Now())
	a.st.d.Probe = &Wiederherstellungsprobe{Datum: res.Datum, Ok: res.Ok}
	a.st.audit("Wiederherstellungs-Probe durchgeführt")
	a.speichernUndAntworten(w, res)
}

// probeAufgabeLocked: Cockpit-Hinweis, wenn die Probe lange her ist oder zuletzt fehlschlug.
func (s *Store) probeAufgabeLocked(heute time.Time, add func(stufe, titel, text, ziel, knopf string)) {
	if s.d.Probe != nil && !s.d.Probe.Ok {
		add("err", "Die letzte Wiederherstellungs-Probe ist fehlgeschlagen", "Eine Sicherung ließ sich nicht einlesen. Neu sichern und die Probe wiederholen.", "admin:daten", "Probe")
		return
	}
	seit := 1 << 30
	if s.d.Probe != nil {
		seit = -tageBis(heute, s.d.Probe.Datum)
	}
	if seit < probeIntervallTage || len(s.d.Paechter) == 0 {
		return
	}
	text := "Noch nie getestet, ob sich die Sicherungen wirklich einlesen lassen. Das dauert eine Sekunde."
	if s.d.Probe != nil {
		text = fmt.Sprintf("Die letzte Probe ist %d Tage her.", seit)
	}
	add("info", "Wiederherstellungs-Probe machen", text, "admin:daten", "Probe")
}
