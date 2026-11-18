package main

import (
	"io/fs"
	"time"
)

// Selbsttest nach einem Update: Beim ersten Start einer neuen Programmversion prüft das Programm, ob die
// Oberfläche vollständig ist und ob sich Daten und Sicherungen noch einlesen lassen. Das Ergebnis zeigt
// der Dialog »Neu in Version …«.

type Selbsttest struct {
	Version string       `json:"version"`
	Datum   string       `json:"datum"`
	Ok      bool         `json:"ok"`
	Zeilen  []probeZeile `json:"zeilen"`
}

// selbsttestLocked läuft einmal je Programmversion. Liefert true, wenn gespeichert werden muss. Der Aufrufer hält s.mu.
func (s *Store) selbsttestLocked(jetzt time.Time, web fs.FS) bool {
	if s.d.LetzteVersion == appVersion {
		return false
	}
	erste := s.d.LetzteVersion == ""
	s.d.LetzteVersion = appVersion
	if erste && len(s.d.Paechter) == 0 {
		return true // neue Installation: nichts zu prüfen
	}
	t := &Selbsttest{Version: appVersion, Datum: jetzt.Format("2006-01-02"), Ok: true}
	fehlt := ""
	for _, n := range []string{"index.html", "app.js", "style.css"} {
		if _, err := fs.Stat(web, n); err != nil {
			fehlt += n + " "
		}
	}
	if fehlt != "" {
		t.Zeilen = append(t.Zeilen, probeZeile{"Programmdateien", "err", "Es fehlen: " + fehlt + "– bitte das Programm neu herunterladen."})
	} else {
		t.Zeilen = append(t.Zeilen, probeZeile{"Programmdateien", "ok", "Die Oberfläche ist vollständig."})
	}
	res := s.probeLocked(jetzt)
	t.Zeilen = append(t.Zeilen, res.Zeilen...)
	for _, z := range t.Zeilen {
		if z.Stufe == "err" {
			t.Ok = false
		}
	}
	s.d.Selbsttest = t
	s.d.Probe = &Wiederherstellungsprobe{Datum: res.Datum, Ok: res.Ok}
	s.audit("Selbsttest nach Update auf Version %s durchgeführt", appVersion)
	return true
}

func (s *Store) selbsttestNachUpdate(web fs.FS) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.selbsttestLocked(time.Now(), web) {
		_ = s.saveLocked()
	}
}
