package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Wartungs-Check: Eine Karte »Ist alles in Ordnung?« für den Rechner, auf dem das Programm läuft. Es wird nur gelesen
// (und kurz eine leere Testdatei angelegt und wieder gelöscht, um zu prüfen, ob der Datenordner beschreibbar ist).

type wartungZeile = probeZeile

type wartungErgebnis struct {
	Zeilen []wartungZeile `json:"zeilen"`
	Stufe  string         `json:"stufe"` // schlechteste Stufe: ok | warn | err
}

const (
	wartungMaxDateien    = 200000
	wartungGrosserOrdner = 2 << 30 // 2 GB
)

func ordnerGroesse(dir string) (bytes int64, dateien int, abgebrochen bool) {
	_ = filepath.WalkDir(dir, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !e.IsDir() {
			if fi, err := e.Info(); err == nil && fi.Mode().IsRegular() {
				bytes += fi.Size()
			}
			dateien++
			if dateien >= wartungMaxDateien {
				abgebrochen = true
				return fs.SkipAll
			}
		}
		return nil
	})
	return
}

func groesseKurz(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%s GB", fmtFlex(round2(float64(b)/(1<<30))))
	case b >= 1<<20:
		return fmt.Sprintf("%s MB", fmtFlex(round2(float64(b)/(1<<20))))
	}
	return fmt.Sprintf("%d KB", max(1, b>>10))
}

// beschreibbar prüft einen Ordner mit einer kurzen Testdatei.
func beschreibbar(dir string) bool {
	f, err := os.CreateTemp(dir, ".wartung-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name) == nil
}

// wartungLocked führt die Prüfungen aus. Der Aufrufer hält s.mu; a liefert Passwort-Stand.
func (a *App) wartungLocked(jetzt time.Time) wartungErgebnis {
	s := a.st
	var z []wartungZeile
	add := func(titel, stufe, text string) { z = append(z, wartungZeile{titel, stufe, text}) }

	if beschreibbar(s.dir) {
		add("Datenordner", "ok", "Der Datenordner ist beschreibbar: "+s.dir)
	} else {
		add("Datenordner", "err", "In den Datenordner lässt sich nicht schreiben. Änderungen gehen verloren: "+s.dir)
	}
	low := strings.ToLower(s.dir)
	for _, w := range []string{"onedrive", "dropbox", "google drive", "googledrive", "icloud", "nextcloud"} {
		if strings.Contains(low, w) {
			add("Synchronisierter Ordner", "warn", "Der Datenordner liegt in einem Cloud-Ordner ("+w+"). Dann liegen Mitgliederdaten außerhalb des Vereins, und gleichzeitiges Arbeiten auf zwei Rechnern kann Daten überschreiben.")
			break
		}
	}
	b, n, abgebrochen := ordnerGroesse(s.dir)
	switch {
	case b >= wartungGrosserOrdner:
		add("Größe des Datenordners", "warn", fmt.Sprintf("%s in %d Dateien – ungewöhnlich groß. Alte Sicherungen, Vereinsakten oder Belege prüfen.", groesseKurz(b), n))
	default:
		t := fmt.Sprintf("%s in %d Dateien.", groesseKurz(b), n)
		if abgebrochen {
			t = "Sehr viele Dateien (" + fmt.Sprint(n) + "+)."
		}
		add("Größe des Datenordners", "ok", t)
	}

	name, zeit := neuesteSicherungIn(s.backupDir())
	switch {
	case name == "":
		add("Sicherung", "err", "Es gibt noch keine Sicherung im Datenordner.")
	case jetzt.Sub(zeit) > 8*24*time.Hour:
		add("Sicherung", "warn", fmt.Sprintf("Die letzte Sicherung ist %d Tage alt (%s).", int(jetzt.Sub(zeit).Hours()/24), zeit.Format("02.01.2006")))
	default:
		add("Sicherung", "ok", "Die letzte Sicherung ist vom "+zeit.Format("02.01.2006")+".")
	}
	if dir := strings.TrimSpace(s.d.Settings.ZweiteSicherung); dir == "" {
		add("Zweiter Sicherungsordner", "warn", "Nicht eingerichtet. Ein USB-Stick oder Netzlaufwerk schützt bei einem Festplattenschaden.")
	} else if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		add("Zweiter Sicherungsordner", "err", "Der Ordner ist nicht erreichbar (USB-Stick nicht eingesteckt?): "+dir)
	} else if !beschreibbar(dir) {
		add("Zweiter Sicherungsordner", "err", "In den Ordner lässt sich nicht schreiben: "+dir)
	} else {
		add("Zweiter Sicherungsordner", "ok", "Erreichbar und beschreibbar.")
	}
	switch {
	case s.d.Probe == nil:
		add("Wiederherstellungs-Probe", "warn", "Noch nie getestet, ob sich die Sicherungen einlesen lassen.")
	case !s.d.Probe.Ok:
		add("Wiederherstellungs-Probe", "err", "Die letzte Probe vom "+germanDate(s.d.Probe.Datum)+" ist fehlgeschlagen.")
	case tageBis(jetzt, s.d.Probe.Datum) < -probeIntervallTage:
		add("Wiederherstellungs-Probe", "warn", "Die letzte Probe ist über "+fmt.Sprint(probeIntervallTage)+" Tage her ("+germanDate(s.d.Probe.Datum)+").")
	default:
		add("Wiederherstellungs-Probe", "ok", "Zuletzt am "+germanDate(s.d.Probe.Datum)+" bestanden.")
	}
	if a.hasPassword() {
		t := "Ein Admin-Passwort ist gesetzt."
		if !s.d.AllesGeschuetzt {
			t += " Zählerstände und Termine sind ohne Anmeldung sichtbar; mit »Alles nur mit Anmeldung« lässt sich das schließen."
		}
		add("Zugriffsschutz", "ok", t)
	} else {
		add("Zugriffsschutz", "warn", "Es ist kein Admin-Passwort gesetzt. Jeder, der den Rechner bedient, kann alles ändern.")
	}
	add("Programmversion", "ok", "Version "+appVersion+". Nach Updates sucht das Programm nur auf Knopfdruck (Einstellungen).")

	stufe := "ok"
	for _, x := range z {
		if x.Stufe == "err" {
			stufe = "err"
		} else if x.Stufe == "warn" && stufe == "ok" {
			stufe = "warn"
		}
	}
	return wartungErgebnis{Zeilen: z, Stufe: stufe}
}

func (a *App) handleWartung(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	res := a.wartungLocked(time.Now())
	a.st.mu.Unlock()
	writeJSON(w, 200, res)
}
