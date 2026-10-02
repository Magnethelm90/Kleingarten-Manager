package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const dataFileName = "kleingarten-manager-daten.json"

// legacyDataFileName war der Dateiname vor der Umbenennung von
// "Gartenabrechnung". Beim ersten Start nach einem Update wird eine
// vorhandene alte Datendatei automatisch übernommen, damit niemand von Hand
// etwas umbenennen muss.
const legacyDataFileName = "gartenabrechnung-daten.json"

// Store hält den Datenbestand im Speicher und schreibt ihn sicher in eine Datei.
type Store struct {
	mu   sync.Mutex
	dir  string
	path string
	d    *Data

	// Cache für pruefeSicherung, damit nicht bei jedem Seitenaufruf erneut die
	// komplette jüngste Sicherungsdatei gelesen und geparst wird.
	sicherungGeprueft string // Dateiname der zuletzt geprüften Sicherung
	sicherungFehler   string
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func openStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	// Schreibrechte prüfen
	probe := filepath.Join(dir, ".schreibtest")
	if err := os.WriteFile(probe, []byte("x"), 0o644); err != nil {
		return nil, fmt.Errorf("Ordner %s ist nicht beschreibbar: %w", dir, err)
	}
	_ = os.Remove(probe)

	s := &Store{dir: dir, path: filepath.Join(dir, dataFileName)}
	// Umstieg von der alten Datendatei (vor der Umbenennung): einmalig übernehmen,
	// falls noch keine neue Datei existiert.
	if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
		legacy := filepath.Join(dir, legacyDataFileName)
		if _, lerr := os.Stat(legacy); lerr == nil {
			_ = os.Rename(legacy, s.path)
		}
	}
	raw, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		s.d = newData()
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	default:
		var d Data
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, fmt.Errorf("Datendatei %s ist beschädigt (%v). Bitte eine Sicherung aus dem Ordner »Sicherungen« zurückkopieren", s.path, err)
		}
		if d.Jahre == nil {
			d.Jahre = map[string]*Jahr{}
		}
		if d.Paechter == nil {
			d.Paechter = []Paechter{}
		}
		if d.Rechnungen == nil {
			d.Rechnungen = []*Rechnung{}
		}
		s.d = &d
		s.ensureYear(d.Settings.Jahr)
		if s.migriereBelege() {
			if err := s.saveLocked(); err != nil {
				return nil, err
			}
		}
	}
	return s, nil
}

// migriereBelege überführt das alte einzelne Beleg-Feld (vor Mehrfach-Belegen)
// in die neue Belege-Liste. Läuft nur beim Öffnen, bevor andere Anfragen
// möglich sind, daher ohne zusätzliche Sperre. Meldet zurück, ob sich etwas
// geändert hat.
func (s *Store) migriereBelege() bool {
	changed := false
	for _, j := range s.d.Jahre {
		for i := range j.Ausgaben {
			if j.Ausgaben[i].Beleg != "" {
				j.Ausgaben[i].Belege = append(j.Ausgaben[i].Belege, j.Ausgaben[i].Beleg)
				j.Ausgaben[i].Beleg = ""
				changed = true
			}
		}
	}
	return changed
}

func (s *Store) ensureYear(y int) *Jahr {
	k := yearKey(y)
	j := s.d.Jahre[k]
	if j == nil {
		j = &Jahr{}
		s.d.Jahre[k] = j
	}
	if j.Ablesungen == nil {
		j.Ablesungen = map[string]Ablesung{}
	}
	return j
}

// saveLocked schreibt atomar (erst temporäre Datei, dann umbenennen) und legt
// höchstens eine Tagessicherung an. Der Aufrufer hält s.mu.
func (s *Store) saveLocked() error {
	raw, err := json.MarshalIndent(s.d, "", "  ")
	if err != nil {
		return err
	}
	s.dailyBackup()
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) backupDir() string { return filepath.Join(s.dir, "Sicherungen") }

// audit hängt einen Eintrag ans Änderungsprotokoll an (höchstens 1000 behalten).
// Der Aufrufer hält s.mu; speichert nicht selbst, das übernimmt der anschließende saveLocked.
func (s *Store) audit(format string, args ...any) {
	s.d.AuditLog = append(s.d.AuditLog, AuditEntry{Zeit: time.Now().Format(time.RFC3339), Aktion: fmt.Sprintf(format, args...)})
	if n := len(s.d.AuditLog); n > 1000 {
		s.d.AuditLog = s.d.AuditLog[n-1000:]
	}
}

// mirrorToSecondary kopiert eine Sicherungsdatei zusätzlich in den (optionalen,
// vom Vorstand frei gewählten) zweiten Sicherungsordner, z. B. einen USB-Stick
// oder ein Netzlaufwerk. Fehler (Ordner nicht erreichbar, USB-Stick nicht
// eingesteckt, ...) werden bewusst nur ins Konsolenfenster geschrieben, damit
// das Hauptprogramm trotzdem normal weiterläuft.
func (s *Store) mirrorToSecondary(name string, data []byte) {
	dir := strings.TrimSpace(s.d.Settings.ZweiteSicherung)
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, "Zweiter Sicherungsordner nicht erreichbar:", err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "Sicherung konnte nicht in den zweiten Ordner geschrieben werden:", err)
	}
}

// ---------------------------------------------------------------- Gartenverlauf
//
// Die Belegungshistorie wird unabhängig vom Abrechnungsjahr anhand des echten
// Kalenderdatums geführt. Der Aufrufer hält jeweils s.mu.

// gartenOeffnen trägt einen neuen, noch offenen Abschnitt ein (Bis bleibt leer).
func (s *Store) gartenOeffnen(id, gartennr, mitgliedsnr, name string) {
	gartennr = strings.TrimSpace(gartennr)
	if gartennr == "" {
		return
	}
	s.d.GartenHistorie = append(s.d.GartenHistorie, GartenEintrag{
		Gartennr: gartennr, PaechterID: id, Mitgliedsnr: mitgliedsnr, Name: name, Seit: time.Now().Format("2006-01-02"),
	})
}

// gartenSchliessen beendet den offenen Abschnitt eines Pächters (z. B. bei
// Gartenwechsel oder Löschung).
func (s *Store) gartenSchliessen(id, bis string) {
	for i := range s.d.GartenHistorie {
		e := &s.d.GartenHistorie[i]
		if e.PaechterID == id && e.Bis == "" {
			e.Bis = bis
		}
	}
}

// gartenWiederOeffnen macht eine Schließung rückgängig (Papierkorb wiederhergestellt).
func (s *Store) gartenWiederOeffnen(id, bis string) {
	for i := range s.d.GartenHistorie {
		e := &s.d.GartenHistorie[i]
		if e.PaechterID == id && e.Bis == bis {
			e.Bis = ""
		}
	}
}

// gartenAktualisieren hält Name/Mitgliedsnr im offenen Abschnitt aktuell, wenn
// sich diese ändern, ohne dass der Garten selbst wechselt.
func (s *Store) gartenAktualisieren(id, mitgliedsnr, name string) {
	for i := range s.d.GartenHistorie {
		e := &s.d.GartenHistorie[i]
		if e.PaechterID == id && e.Bis == "" {
			e.Mitgliedsnr, e.Name = mitgliedsnr, name
		}
	}
}

// gartenWechsel behandelt einen Garten- oder Stammdatenwechsel beim Speichern
// eines Pächters: schließt den alten Abschnitt bei Gartenwechsel und eröffnet
// bei Bedarf einen neuen, sonst werden nur Name/Mitgliedsnr nachgezogen.
func (s *Store) gartenWechsel(id, altGartennr, neuGartennr, mitgliedsnr, name string) {
	altGartennr, neuGartennr = strings.TrimSpace(altGartennr), strings.TrimSpace(neuGartennr)
	if altGartennr == neuGartennr {
		s.gartenAktualisieren(id, mitgliedsnr, name)
		return
	}
	if altGartennr != "" {
		s.gartenSchliessen(id, time.Now().Format("2006-01-02"))
	}
	s.gartenOeffnen(id, neuGartennr, mitgliedsnr, name)
}

// checkWritableDir prüft, ob in einen Ordner geschrieben werden kann (legt ihn
// bei Bedarf an). Für die Validierung eines vom Benutzer eingetragenen Pfades,
// z. B. des zweiten Sicherungsordners, damit Tippfehler sofort auffallen.
func checkWritableDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("Ordner %s kann nicht angelegt werden: %w", dir, err)
	}
	probe := filepath.Join(dir, ".schreibtest")
	if err := os.WriteFile(probe, []byte("x"), 0o644); err != nil {
		return fmt.Errorf("Ordner %s ist nicht beschreibbar: %w", dir, err)
	}
	_ = os.Remove(probe)
	return nil
}

// letzteSicherung liefert den Zeitpunkt der jüngsten Sicherungsdatei (leer,
// wenn noch keine existiert). Für die Übersicht auf der Startseite.
func (s *Store) letzteSicherung() time.Time {
	t, _ := s.letzteSicherungMitName()
	return t
}

func (s *Store) letzteSicherungMitName() (time.Time, string) {
	entries, err := os.ReadDir(s.backupDir())
	if err != nil {
		return time.Time{}, ""
	}
	var newest time.Time
	var name string
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newest) {
			newest, name = info.ModTime(), e.Name()
		}
	}
	return newest, name
}

// pruefeSicherung liest die jüngste Sicherungsdatei probeweise ein und meldet
// einen kurzen Hinweistext zurück, falls sie beschädigt ist (z. B. durch einen
// Festplattenfehler oder einen abgebrochenen Schreibvorgang). Leer = alles gut
// oder noch keine Sicherung vorhanden.
func (s *Store) pruefeSicherung() string {
	_, name := s.letzteSicherungMitName()
	if name == "" {
		return ""
	}
	if name == s.sicherungGeprueft {
		return s.sicherungFehler // schon geprüft, nicht erneut von der Platte lesen
	}
	fehler := ""
	raw, err := os.ReadFile(filepath.Join(s.backupDir(), name))
	if err != nil {
		fehler = "Die jüngste Sicherung (" + name + ") konnte nicht gelesen werden"
	} else {
		var d Data
		if err := json.Unmarshal(raw, &d); err != nil {
			fehler = "Die jüngste Sicherung (" + name + ") ist beschädigt und sollte geprüft werden"
		}
	}
	s.sicherungGeprueft, s.sicherungFehler = name, fehler
	return fehler
}

func (s *Store) dailyBackup() {
	old, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	dir := s.backupDir()
	if os.MkdirAll(dir, 0o700) != nil {
		return
	}
	base := backupPrefix + time.Now().Format("2006-01-02") + ".json"
	name := filepath.Join(dir, base)
	if _, err := os.Stat(name); err == nil {
		return // heute schon gesichert
	}
	_ = os.WriteFile(name, old, 0o600)
	pruneBackups(dir, isDailyBackup, 60)
	s.mirrorToSecondary(base, old)
}

// backupPrefix ist der aktuelle Dateiname-Präfix für Sicherungen.
// legacyBackupPrefix (vor der Umbenennung) wird beim Aufräumen weiterhin
// erkannt, damit alte Sicherungen nicht als Datenmüll liegen bleiben, der
// nie mitgezählt oder aufgeräumt wird.
const (
	backupPrefix       = "kleingarten-manager-daten-"
	legacyBackupPrefix = "gartenabrechnung-daten-"
)

// isDailyBackup erkennt Tagessicherungen (…-daten-JJJJ-MM-TT.json).
func isDailyBackup(name string) bool {
	d, ok := strings.CutPrefix(name, backupPrefix)
	if !ok {
		d, ok = strings.CutPrefix(name, legacyBackupPrefix)
		if !ok {
			return false
		}
	}
	d, ok = strings.CutSuffix(d, ".json")
	if !ok {
		return false
	}
	_, err := parseDate(d)
	return err == nil
}

// pruneBackups behält von den Sicherungen, auf die match passt, nur die neuesten keep.
// Tages- und Ereignissicherungen werden getrennt gezählt, damit viele Ereignisse
// keine Tagessicherungen verdrängen.
func pruneBackups(dir string, match func(string) bool, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if match(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for len(names) > keep {
		_ = os.Remove(filepath.Join(dir, names[0]))
		names = names[1:]
	}
}

// snapshotBackup legt eine benannte Sicherung an (z. B. vor dem Jahreswechsel).
func (s *Store) snapshotBackup(label string) {
	old, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	dir := s.backupDir()
	if os.MkdirAll(dir, 0o700) != nil {
		return
	}
	name := fmt.Sprintf("%s%s-%s.json", backupPrefix, time.Now().Format("2006-01-02_150405"), label)
	_ = os.WriteFile(filepath.Join(dir, name), old, 0o600)
	pruneBackups(dir, func(n string) bool {
		return (strings.HasPrefix(n, backupPrefix) || strings.HasPrefix(n, legacyBackupPrefix)) && strings.HasSuffix(n, ".json") && !isDailyBackup(n)
	}, 60)
	s.mirrorToSecondary(name, old)
}

// yearView liefert Einstellungen, Pächter und Ablesungen für ein Jahr.
// Für abgeschlossene Jahre kommen die eingefrorenen Kopien zurück.
type yearView struct {
	Jahr       int
	Settings   Settings
	Paechter   []Paechter
	Ablesungen map[string]Ablesung
	ReadOnly   bool
}

func (s *Store) viewLocked(year int) (yearView, bool) {
	j := s.d.Jahre[yearKey(year)]
	if j == nil {
		return yearView{}, false
	}
	v := yearView{Jahr: year, Ablesungen: j.Ablesungen}
	if j.Abgeschlossen && j.Settings != nil {
		v.Settings = *j.Settings
		v.Paechter = j.Paechter
		v.ReadOnly = true
	} else {
		v.Settings = s.d.Settings
		v.Paechter = aktivePaechter(s.d.Paechter)
	}
	return v, true
}

// aktivePaechter blendet Pächter im Papierkorb aus dem laufenden Jahr aus.
// Abgeschlossene Jahre sind davon nicht betroffen: sie zeigen weiterhin die
// eingefrorene Kopie von damals, egal ob der Pächter inzwischen gelöscht wurde.
func aktivePaechter(all []Paechter) []Paechter {
	out := make([]Paechter, 0, len(all))
	for _, p := range all {
		if !p.Geloescht {
			out = append(out, p)
		}
	}
	return out
}

func (s *Store) years() []int {
	var ys []int
	for k := range s.d.Jahre {
		var y int
		if _, err := fmt.Sscanf(k, "%d", &y); err == nil {
			ys = append(ys, y)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ys)))
	return ys
}
