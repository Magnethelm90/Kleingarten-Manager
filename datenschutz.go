package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// aufbewahrungJahre ist die Frist, nach der Rechnungen und die daraus entstandenen
// Jahresunterlagen ihren Personenbezug verlieren. Bewusst die längere der gängigen
// Fristen (Bücher und Jahresabschlüsse 10 Jahre, reine Buchungsbelege seit 2025
// meist 8 Jahre). Die Frist beginnt mit dem Ende des Kalenderjahres der Ausstellung.
// Ob eine kürzere Frist gilt, klärt der Verein mit Steuerberatung oder Kassenprüfern.
const aufbewahrungJahre = 10

// datenVersion ist die Version des Dateiformats, das dieses Programm lesen und schreiben kann.
const datenVersion = 1

const (
	geloeschtText       = "(gelöscht)"
	fristAbgelaufenText = "(Aufbewahrungsfrist abgelaufen)"
)

// jahrAusZeit liest das Jahr aus einem Datum, das mit JJJJ beginnt (0, wenn unlesbar).
func jahrAusZeit(s string) int {
	if len(s) < 4 {
		return 0
	}
	n, err := strconv.Atoi(s[:4])
	if err != nil {
		return 0
	}
	return n
}

// fristAbgelaufen: Ausgestellt im Jahr y, die Frist endet am 31.12. von y+aufbewahrungJahre.
func fristAbgelaufen(y, aktuell int) bool { return y > 0 && aktuell > y+aufbewahrungJahre }

func (r *Rechnung) ausstellungsjahr() int {
	if y := jahrAusZeit(r.Ausgestellt); y > 0 {
		return y
	}
	return r.Jahr
}

// anonymisiere entfernt alle Angaben, die auf eine Person hinweisen. Gartengröße,
// Versandart und Umlage bleiben, sie sagen ohne Namen nichts über eine Person aus.
func anonymisiere(p *Paechter, text string) {
	p.Mitgliedsnr, p.Anrede, p.Name, p.Strasse, p.PLZOrt = "", "", text, "", ""
	p.WasserzaehlerNr, p.StromzaehlerNr, p.Notiz = "", "", ""
}

// entfernePerson löscht einen Pächter aus dem Datenbestand und entfernt seine Spuren, soweit
// keine Aufbewahrungspflicht besteht: Stammdatensatz, offene Zählerstände, Notizen, Namen in
// Garten-Historie und Änderungsprotokoll. Rechnungen samt Jahreskopien bleiben bis zum Ende
// der Aufbewahrungsfrist erhalten (siehe bereinigeAbgelaufene). Wirkt auf d, auch für Sicherungen.
func (d *Data) entfernePerson(id string) bool {
	var name, nr string
	idx := -1
	for i, p := range d.Paechter {
		if p.ID == id {
			idx, name, nr = i, p.Name, p.Mitgliedsnr
			break
		}
	}
	if idx >= 0 {
		d.Paechter = append(d.Paechter[:idx], d.Paechter[idx+1:]...)
	}
	changed := idx >= 0
	for i := range d.GartenHistorie {
		if e := &d.GartenHistorie[i]; e.PaechterID == id && (e.Name != geloeschtText || e.Mitgliedsnr != "") {
			e.Name, e.Mitgliedsnr, changed = geloeschtText, "", true
		}
	}
	for _, j := range d.Jahre {
		if a, ok := j.Ablesungen[id]; ok {
			if !j.Abgeschlossen {
				delete(j.Ablesungen, id)
				changed = true
			} else if a.Hinweis != "" {
				a.Hinweis = ""
				j.Ablesungen[id] = a
				changed = true
			}
		}
		for i := range j.Paechter {
			if j.Paechter[i].ID == id && j.Paechter[i].Notiz != "" {
				j.Paechter[i].Notiz, changed = "", true
			}
		}
	}
	for _, r := range d.Rechnungen {
		if r.PaechterID == id && (r.Notiz != "" || r.Paechter.Notiz != "") {
			r.Notiz, r.Paechter.Notiz, changed = "", "", true
		}
	}
	if name != "" && d.schwaerzeProtokoll(nr, name) {
		changed = true
	}
	return changed
}

// schwaerzeProtokoll ersetzt Name (und »Nr Name«) in den Texten des Änderungsprotokolls.
func (d *Data) schwaerzeProtokoll(nr, name string) bool {
	changed := false
	for i := range d.AuditLog {
		t := d.AuditLog[i].Aktion
		n := t
		if nr != "" {
			n = strings.ReplaceAll(n, nr+" "+name, geloeschtText)
		}
		if len([]rune(name)) >= 3 {
			n = strings.ReplaceAll(n, name, geloeschtText)
		}
		if n != t {
			d.AuditLog[i].Aktion, changed = n, true
		}
	}
	return changed
}

// bereinigeAbgelaufene entfernt den Personenbezug aller Unterlagen, deren Aufbewahrungsfrist
// abgelaufen ist: ausgestellte Rechnungen (die PDF-Dateien liefert der Rückgabewert zum Löschen)
// und die eingefrorenen Pächterlisten abgeschlossener Jahre. Idempotent.
func (d *Data) bereinigeAbgelaufene(aktuell int) (rechnungen int, jahre int, dateien []string) {
	for _, r := range d.Rechnungen {
		if r.Bereinigt || !fristAbgelaufen(r.ausstellungsjahr(), aktuell) {
			continue
		}
		anonymisiere(&r.Paechter, fristAbgelaufenText)
		r.Notiz, r.Bereinigt = "", true
		if r.Datei != "" {
			dateien = append(dateien, r.Datei)
			r.Datei = ""
		}
		rechnungen++
	}
	for key, j := range d.Jahre {
		y, err := strconv.Atoi(key)
		if err != nil || !j.Abgeschlossen || !fristAbgelaufen(y, aktuell) {
			continue
		}
		geaendert := false
		for i := range j.Paechter {
			if j.Paechter[i].Name != fristAbgelaufenText {
				anonymisiere(&j.Paechter[i], fristAbgelaufenText)
				geaendert = true
			}
		}
		for id, a := range j.Ablesungen {
			if a.Hinweis != "" {
				a.Hinweis = ""
				j.Ablesungen[id] = a
				geaendert = true
			}
		}
		if geaendert {
			jahre++
		}
	}
	return
}

// sicherungsDateien listet alle Sicherungsdateien im Sicherungsordner und im
// (optionalen) zweiten Sicherungsordner.
func (s *Store) sicherungsDateien() []string {
	dirs := []string{s.backupDir()}
	if z := strings.TrimSpace(s.d.Settings.ZweiteSicherung); z != "" {
		dirs = append(dirs, z)
	}
	var out []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			n := e.Name()
			if !e.IsDir() && strings.HasSuffix(n, ".json") &&
				(strings.HasPrefix(n, backupPrefix) || strings.HasPrefix(n, legacyBackupPrefix)) {
				out = append(out, filepath.Join(dir, n))
			}
		}
	}
	return out
}

// bereinigeSicherungen wendet fn auf jede Sicherung an und schreibt geänderte Dateien
// zurück. Ohne das würden gelöschte Daten in alten Sicherungen weiterleben.
// Beschädigte oder unlesbare Dateien werden nicht angefasst. Gibt die Zahl der
// geänderten Dateien zurück.
func (s *Store) bereinigeSicherungen(fn func(d *Data) bool) int {
	n := 0
	for _, f := range s.sicherungsDateien() {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var d Data
		// Sicherungen einer neueren Datenversion nicht anfassen: unbekannte Felder gingen beim
		// Zurückschreiben verloren
		if json.Unmarshal(raw, &d) != nil || d.Version > datenVersion || !fn(&d) {
			continue
		}
		out, err := json.MarshalIndent(&d, "", "  ")
		if err != nil {
			continue
		}
		tmp := f + ".tmp"
		if os.WriteFile(tmp, out, 0o600) != nil {
			_ = os.Remove(tmp)
			continue
		}
		if os.Rename(tmp, f) != nil {
			_ = os.Remove(tmp)
			continue
		}
		n++
	}
	return n
}

// ---------------------------------------------------------------- Auskunft (Art. 15, 20 DSGVO)

type auskunftJahr struct {
	Jahr     int      `json:"jahr"`
	Zaehler  Ablesung `json:"zaehlerstaendeUndAngaben"`
	Gesperrt bool     `json:"abgeschlossen"`
}

type auskunftRechnung struct {
	Nummer        string   `json:"rechnungsnummer"`
	Jahr          int      `json:"abrechnungsjahr"`
	Version       int      `json:"version"`
	Status        string   `json:"status"`
	Ausgestellt   string   `json:"ausgestelltAm"`
	Gesamtbetrag  float64  `json:"gesamtbetrag"`
	BezahltAm     string   `json:"bezahltAm,omitempty"`
	BezahltBetrag *float64 `json:"bezahlterBetrag,omitempty"`
	Notiz         string   `json:"notiz,omitempty"`
	Bereinigt     bool     `json:"personenbezugEntfernt,omitempty"`
}

type auskunft struct {
	Erstellt    string             `json:"erstelltAm"`
	Verantwort  string             `json:"verantwortlich"`
	Hinweis     string             `json:"hinweis"`
	Stammdaten  Paechter           `json:"stammdaten"`
	Garten      []GartenEintrag    `json:"gartenhistorie"`
	Jahre       []auskunftJahr     `json:"jahre"`
	Rechnungen  []auskunftRechnung `json:"rechnungen"`
	Protokoll   []AuditEntry       `json:"aenderungsprotokoll"`
	Aufbewahrng string             `json:"aufbewahrung"`
}

// auskunftLocked stellt alle zu einer Person gespeicherten Daten zusammen. Der Aufrufer hält s.mu.
func (s *Store) auskunftLocked(id string) (auskunft, bool) {
	var p *Paechter
	for i := range s.d.Paechter {
		if s.d.Paechter[i].ID == id {
			p = &s.d.Paechter[i]
			break
		}
	}
	if p == nil {
		return auskunft{}, false
	}
	a := auskunft{
		Erstellt:   time.Now().Format(time.RFC3339),
		Verantwort: s.d.Settings.VereinName,
		Hinweis: "Auskunft nach Art. 15 DSGVO (zugleich Datenkopie nach Art. 20 in maschinenlesbarer Form): alle zu dieser Person " +
			"im " + appName + " gespeicherten Daten.",
		Stammdaten: *p,
		Garten:     []GartenEintrag{},
		Jahre:      []auskunftJahr{},
		Rechnungen: []auskunftRechnung{},
		Protokoll:  []AuditEntry{},
		Aufbewahrng: fmt.Sprintf("Rechnungen und Jahresunterlagen werden %d Jahre ab Ende des Ausstellungsjahres aufbewahrt "+
			"(steuerrechtliche Pflicht), danach wird der Personenbezug entfernt.", aufbewahrungJahre),
	}
	for _, e := range s.d.GartenHistorie {
		if e.PaechterID == id {
			a.Garten = append(a.Garten, e)
		}
	}
	keys := make([]string, 0, len(s.d.Jahre))
	for k := range s.d.Jahre {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if ab, ok := s.d.Jahre[k].Ablesungen[id]; ok {
			y, _ := strconv.Atoi(k)
			a.Jahre = append(a.Jahre, auskunftJahr{Jahr: y, Zaehler: ab, Gesperrt: s.d.Jahre[k].Abgeschlossen})
		}
	}
	for _, r := range s.d.Rechnungen {
		if r.PaechterID == id {
			a.Rechnungen = append(a.Rechnungen, auskunftRechnung{
				Nummer: r.Nummer, Jahr: r.Jahr, Version: r.Version, Status: r.Status, Ausgestellt: r.Ausgestellt,
				Gesamtbetrag: r.Result.Gesamt, BezahltAm: r.BezahltAm, BezahltBetrag: r.BezahltBetrag, Notiz: r.Notiz, Bereinigt: r.Bereinigt,
			})
		}
	}
	for _, e := range s.d.AuditLog {
		if (p.Name != "" && strings.Contains(e.Aktion, p.Name)) || (p.Mitgliedsnr != "" && strings.Contains(e.Aktion, p.Mitgliedsnr+" ")) {
			a.Protokoll = append(a.Protokoll, e)
		}
	}
	return a, true
}

func (a *App) handleAuskunft(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	res, ok := a.st.auskunftLocked(r.PathValue("id"))
	a.st.mu.Unlock()
	if !ok {
		writeErr(w, notFound("Pächter nicht gefunden"))
		return
	}
	raw, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		writeErr(w, err)
		return
	}
	name := safeName("Datenauskunft_"+res.Stammdaten.Mitgliedsnr, 60) + ".json"
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(raw)
}

// ---------------------------------------------------------------- Aufbewahrung / Bereinigung

type datenschutzInfo struct {
	AufbewahrungJahre int `json:"aufbewahrungJahre"`
	// Faellig: wie viele Unterlagen ihre Aufbewahrungsfrist überschritten haben
	FaelligRechnungen int `json:"faelligRechnungen"`
	FaelligJahre      int `json:"faelligJahre"`
	// BereinigtBisJahr: Rechnungen, die bis einschließlich diesem Jahr ausgestellt wurden, sind frei zur Bereinigung
	BereinigtBisJahr int `json:"bereinigtBisJahr"`
	// Papierkorb: Pächter, die noch auf die endgültige Löschung warten
	Papierkorb int `json:"papierkorb"`
	// NaechsteFrist: Jahr, in dem die nächste noch aufbewahrte Rechnung freigegeben wird (0 = keine)
	NaechsteFrist int `json:"naechsteFrist"`
}

func (s *Store) datenschutzInfoLocked(aktuell int) datenschutzInfo {
	info := datenschutzInfo{AufbewahrungJahre: aufbewahrungJahre, BereinigtBisJahr: aktuell - aufbewahrungJahre - 1}
	for _, r := range s.d.Rechnungen {
		if r.Bereinigt {
			continue
		}
		y := r.ausstellungsjahr()
		if fristAbgelaufen(y, aktuell) {
			info.FaelligRechnungen++
		} else if y > 0 {
			if f := y + aufbewahrungJahre + 1; info.NaechsteFrist == 0 || f < info.NaechsteFrist {
				info.NaechsteFrist = f
			}
		}
	}
	for key, j := range s.d.Jahre {
		y, err := strconv.Atoi(key)
		if err != nil || !j.Abgeschlossen || !fristAbgelaufen(y, aktuell) {
			continue
		}
		for _, p := range j.Paechter {
			if p.Name != fristAbgelaufenText {
				info.FaelligJahre++
				break
			}
		}
	}
	for _, p := range s.d.Paechter {
		if p.Geloescht {
			info.Papierkorb++
		}
	}
	return info
}

func (a *App) handleDatenschutz(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	info := a.st.datenschutzInfoLocked(time.Now().Year())
	a.st.mu.Unlock()
	writeJSON(w, 200, info)
}

func (a *App) handleBereinigen(w http.ResponseWriter, r *http.Request) {
	aktuell := time.Now().Year()
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	rechnungen, jahre, dateien := a.st.d.bereinigeAbgelaufene(aktuell)
	if rechnungen == 0 && jahre == 0 {
		writeJSON(w, 200, map[string]int{"rechnungen": 0, "jahre": 0, "dateien": 0, "sicherungen": 0})
		return
	}
	a.st.audit("Aufbewahrungsfrist abgelaufen: Personenbezug von %d Rechnung(en) und %d Jahresunterlage(n) entfernt", rechnungen, jahre)
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	geloescht := 0
	for _, rel := range dateien {
		// dieselbe strenge Prüfung wie beim Öffnen: nur gewöhnliche .pdf-Dateien im Rechnungsordner,
		// keine Verweise (Symlinks); der Pfad stammt aus der Datendatei und gilt nicht als vertrauenswürdig
		full, err := a.rechnungsDatei(rel)
		if err != nil {
			continue
		}
		if fi, err := os.Lstat(full); err == nil && fi.Mode().IsRegular() && os.Remove(full) == nil {
			geloescht++
		}
		// die Sammel-Druckdatei dieses Jahres enthält dieselben Angaben, sie lässt sich neu erzeugen
		sammel := filepath.Join(filepath.Dir(full), sammelDateiName)
		if fi, err := os.Lstat(sammel); err == nil && fi.Mode().IsRegular() {
			_ = os.Remove(sammel)
		}
	}
	sicherungen := a.st.bereinigeSicherungen(func(d *Data) bool {
		r, j, _ := d.bereinigeAbgelaufene(aktuell)
		return r > 0 || j > 0
	})
	writeJSON(w, 200, map[string]int{"rechnungen": rechnungen, "jahre": jahre, "dateien": geloescht, "sicherungen": sicherungen})
}
