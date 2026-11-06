package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Mehrplatz ohne Cloud: Liegt der Datenordner auf einem USB-Stick oder Netzlaufwerk, können mehrere
// Vorstandsmitglieder abwechselnd damit arbeiten. Eine Sperrdatei im Datenordner verhindert, dass zwei Rechner
// gleichzeitig schreiben und sich gegenseitig Änderungen überschreiben. Das laufende Programm frischt die Sperre
// alle 30 Sekunden auf; ist sie älter als zwei Minuten (Absturz, Stromausfall, Stick abgezogen), gilt sie als
// verwaist und darf übernommen werden. Vor jedem Speichern wird geprüft, ob die Sperre noch diesem Programm gehört.

const (
	sperrDateiName   = "kleingarten-manager.sperre"
	sperrAuffrischen = 30 * time.Second
	sperrVerwaist    = 2 * time.Minute
)

type sperrInfo struct {
	Token    string    `json:"token"`
	Rechner  string    `json:"rechner"`
	Benutzer string    `json:"benutzer"`
	Seit     time.Time `json:"seit"`
	Zuletzt  time.Time `json:"zuletzt"`
}

func (s *Store) sperrPfad() string { return filepath.Join(s.dir, sperrDateiName) }

func leseSperre(pfad string) (sperrInfo, bool) {
	raw, err := os.ReadFile(pfad)
	if err != nil {
		return sperrInfo{}, false
	}
	var si sperrInfo
	if json.Unmarshal(raw, &si) != nil || si.Token == "" {
		return sperrInfo{}, false
	}
	return si, true
}

func diesesGeraet() (rechner, benutzer string) {
	rechner, _ = os.Hostname()
	benutzer = os.Getenv("USERNAME")
	if benutzer == "" {
		benutzer = os.Getenv("USER")
	}
	return trim(rechner, 60), trim(benutzer, 60)
}

// sperreText beschreibt, wo die Daten gerade geöffnet sind.
func (si sperrInfo) text() string {
	wer := si.Rechner
	if si.Benutzer != "" {
		wer += " (Benutzer " + si.Benutzer + ")"
	}
	return fmt.Sprintf("auf dem Rechner %s, geöffnet am %s um %s Uhr", wer, si.Seit.Local().Format("02.01.2006"), si.Seit.Local().Format("15:04"))
}

// sperreHolen übernimmt den Datenordner für dieses Programm. Ist er auf einem anderen Rechner geöffnet (frische
// Sperre), kommt ein verständlicher Fehler zurück.
func (s *Store) sperreHolen(jetzt time.Time) error {
	if si, ok := leseSperre(s.sperrPfad()); ok && jetzt.Sub(si.Zuletzt) < sperrVerwaist && si.Token != s.sperrToken {
		return fmt.Errorf("Die Daten sind gerade %s in Benutzung.\n\nDamit sich zwei Rechner nicht gegenseitig Änderungen überschreiben, kann immer nur einer damit arbeiten. "+
			"Bitte das Programm dort beenden und es hier dann noch einmal starten.\n\nWurde es dort nicht richtig beendet (z. B. Absturz), ist die Sperre nach zwei Minuten von selbst frei.", si.text())
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	rechner, benutzer := diesesGeraet()
	s.sperre = sperrInfo{Token: hex.EncodeToString(b), Rechner: rechner, Benutzer: benutzer, Seit: jetzt, Zuletzt: jetzt}
	s.sperrToken = s.sperre.Token
	return s.schreibeSperre()
}

func (s *Store) schreibeSperre() error {
	raw, _ := json.Marshal(s.sperre)
	return schreibeAtomar(s.sperrPfad(), raw, 0o600)
}

// sperrePruefenLocked: Gehört die Sperre noch diesem Programm? Wenn nicht, wird nicht mehr gespeichert.
// Ohne eigene Sperre (Tests, Kommandozeile) wird nichts geprüft. Der Aufrufer hält s.mu.
func (s *Store) sperrePruefenLocked() error {
	if s.sperrToken == "" {
		return nil
	}
	if s.sperreVerloren != "" {
		return bad(s.sperreVerloren)
	}
	si, ok := leseSperre(s.sperrPfad())
	if ok && si.Token != s.sperrToken {
		s.sperreVerloren = "Die Daten wurden inzwischen " + si.text() + ". Damit dort nichts überschrieben wird, speichert dieses Fenster nichts mehr. Bitte das Programm hier beenden."
		return bad(s.sperreVerloren)
	}
	if !ok { // Sperrdatei gelöscht (z. B. Stick kurz abgezogen): neu anlegen
		s.sperre.Zuletzt = time.Now()
		return s.schreibeSperre()
	}
	return nil
}

// sperreAuffrischen hält die Sperre lebendig, solange das Programm läuft.
func (s *Store) sperreAuffrischen() {
	for range time.Tick(sperrAuffrischen) {
		s.mu.Lock()
		if s.sperrePruefenLocked() == nil {
			s.sperre.Zuletzt = time.Now()
			_ = s.schreibeSperre()
		}
		s.mu.Unlock()
	}
}

// sperreFreigeben entfernt die eigene Sperre beim Beenden.
func (s *Store) sperreFreigeben() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if si, ok := leseSperre(s.sperrPfad()); ok && s.sperrToken != "" && si.Token == s.sperrToken {
		_ = os.Remove(s.sperrPfad())
	}
}

// sperrHinweisLocked: Text für die Oberfläche, wenn dieses Fenster nicht mehr speichern darf.
func (s *Store) sperrHinweisLocked() string { return strings.TrimSpace(s.sperreVerloren) }
