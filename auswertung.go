package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------- Jahresvergleich

// histEntry ist ein Jahr im Verlauf eines Pächters. Gibt es für das Jahr eine
// gültige ausgestellte Rechnung, stammen die Werte aus ihr, sonst aus der
// aktuellen Berechnung.
type histEntry struct {
	Jahr        int      `json:"jahr"`
	Wasser      *float64 `json:"wasser"`
	Strom       *float64 `json:"strom"`
	Stunden     *float64 `json:"stunden"`
	Gesamt      *float64 `json:"gesamt"` // leer = Rechnung unvollständig
	Ausgestellt bool     `json:"ausgestellt"`
}

// historyLocked liefert je Pächter-ID die Werte aller Jahre, neuestes zuerst.
// Der Aufrufer hält s.mu.
func (s *Store) historyLocked() map[string][]histEntry {
	out := map[string][]histEntry{}
	for _, y := range s.years() {
		v, ok := s.viewLocked(y)
		if !ok {
			continue
		}
		for _, p := range v.Paechter {
			a := v.Ablesungen[p.ID]
			res := calculate(v.Settings, p, a)
			e := histEntry{Jahr: y, Wasser: res.WasserVerbrauch, Strom: res.EnergieVerbrauch, Stunden: a.Stunden}
			if inv := s.validInvoiceLocked(y, p.ID); inv != nil {
				g := inv.Result.Gesamt
				e.Wasser, e.Strom, e.Stunden, e.Gesamt, e.Ausgestellt = inv.Result.WasserVerbrauch, inv.Result.EnergieVerbrauch, inv.Ablesung.Stunden, &g, true
			} else if res.Vollstaendig {
				g := res.Gesamt
				e.Gesamt = &g
			}
			if e.Wasser == nil && e.Strom == nil && e.Stunden == nil && e.Gesamt == nil {
				continue
			}
			out[p.ID] = append(out[p.ID], e)
		}
	}
	return out
}

// ---------------------------------------------------------------- Plausibilitätsprüfung

// Unterhalb dieser Abweichung wird nicht gewarnt (kleine Gärten schwanken stark).
const (
	minAbweichungWasser = 10.0  // m³
	minAbweichungStrom  = 100.0 // kWh
)

// verbrauchHinweis vergleicht einen Verbrauch mit dem Vorjahr, oder, wenn es
// keins gibt, mit dem typischen Verbrauch aller Pächter (Median).
func verbrauchHinweis(art, einheit string, cur, prev *float64, prevJahr int, median, minAbw float64) string {
	if cur == nil || *cur < 0 {
		return "" // fehlt oder Zählerstand kleiner als Vorjahr: meldet schon der Status
	}
	c := *cur
	fmtU := func(x float64) string { return fmtFlex(x) + " " + einheit }
	if prev != nil && *prev > 0 {
		p := *prev
		switch {
		case c > 2*p && c-p >= minAbw:
			return fmt.Sprintf("%s: %s, im Vorjahr (%d) %s – mehr als doppelt so viel", art, fmtU(c), prevJahr, fmtU(p))
		case c*3 < p && p-c >= minAbw:
			return fmt.Sprintf("%s: %s, im Vorjahr (%d) %s – deutlich weniger", art, fmtU(c), prevJahr, fmtU(p))
		}
		return ""
	}
	if median > 0 && c > 4*median && c-median >= minAbw {
		return fmt.Sprintf("%s: %s – mehr als das Vierfache des üblichen Verbrauchs aller Pächter (%s)", art, fmtU(c), fmtU(median))
	}
	return ""
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

// plausiLocked prüft die Verbräuche aller Pächter eines Jahres auf Auffälligkeiten
// (meist Tippfehler beim Zählerstand). Es sind nur Hinweise, sie sperren nichts.
func plausiLocked(v yearView, results map[string]Result, hist map[string][]histEntry) map[string][]string {
	var ws, ss []float64
	for _, r := range results {
		if r.WasserVerbrauch != nil && *r.WasserVerbrauch > 0 {
			ws = append(ws, *r.WasserVerbrauch)
		}
		if r.EnergieVerbrauch != nil && *r.EnergieVerbrauch > 0 {
			ss = append(ss, *r.EnergieVerbrauch)
		}
	}
	// Median erst ab 5 Werten, sonst ist er nicht aussagekräftig
	mw, ms := 0.0, 0.0
	if len(ws) >= 5 {
		mw = median(ws)
	}
	if len(ss) >= 5 {
		ms = median(ss)
	}
	out := map[string][]string{}
	for _, p := range v.Paechter {
		r := results[p.ID]
		var pw, ps *float64
		jw, js := 0, 0
		for _, e := range hist[p.ID] {
			if e.Jahr >= v.Jahr {
				continue
			}
			if pw == nil && e.Wasser != nil {
				pw, jw = e.Wasser, e.Jahr
			}
			if ps == nil && e.Strom != nil {
				ps, js = e.Strom, e.Jahr
			}
		}
		var hs []string
		if t := verbrauchHinweis("Wasser", "m³", r.WasserVerbrauch, pw, jw, mw, minAbweichungWasser); t != "" {
			hs = append(hs, t)
		}
		if t := verbrauchHinweis("Strom", "kWh", r.EnergieVerbrauch, ps, js, ms, minAbweichungStrom); t != "" {
			hs = append(hs, t)
		}
		if len(hs) > 0 {
			out[p.ID] = hs
		}
	}
	return out
}

// ---------------------------------------------------------------- Sonstige Ausgaben

// Ausgabe ist eine einzelne Buchung der Vereinskasse, die nicht über die
// Pächterabrechnung läuft (z. B. Kontoführungsgebühren, Anschaffungen,
// Reparaturen). Jede Ausgabe steht für sich, es gibt keine Pauschalsumme.
type Ausgabe struct {
	ID           string  `json:"id"`
	Datum        string  `json:"datum"` // JJJJ-MM-TT
	Beschreibung string  `json:"beschreibung"`
	Kategorie    string  `json:"kategorie"`
	Betrag       float64 `json:"betrag"`
	// Beleg: Pfad einer hochgeladenen Quittung/Rechnung, relativ zum Datenordner.
	Beleg string `json:"beleg,omitempty"`
	// Geprueft: vom Kassenprüfer abgehakt (z. B. bei der jährlichen Kassenprüfung).
	Geprueft   bool   `json:"geprueft"`
	GeprueftAm string `json:"geprueftAm,omitempty"` // JJJJ-MM-TT
}

// AusgabenKategorien sind Vorschläge für die Kategorie-Auswahl. Es ist keine
// feste Liste: eine abweichende Eingabe wird unverändert übernommen, damit
// niemand durch eine Kategorie blockiert wird, die gerade nicht passt.
var AusgabenKategorien = []string{"Instandhaltung", "Anschaffung", "Verwaltung", "Versicherung & Gebühren", "Sonstiges"}

const kategorieStandard = "Sonstiges"

func cleanAusgabe(in Ausgabe) (Ausgabe, error) {
	in.Beschreibung = trim(in.Beschreibung, 120)
	if in.Beschreibung == "" {
		return in, bad("Bitte eine Beschreibung eintragen")
	}
	in.Kategorie = trim(in.Kategorie, 40)
	if in.Kategorie == "" {
		in.Kategorie = kategorieStandard
	}
	if _, err := parseDate(in.Datum); err != nil {
		return in, bad("Bitte ein gültiges Datum angeben")
	}
	if !validNum(in.Betrag) || in.Betrag <= 0 {
		return in, bad("Der Betrag muss eine Zahl größer 0 sein")
	}
	return in, nil
}

func (a *App) handleAusgabeCreate(w http.ResponseWriter, r *http.Request) {
	var in Ausgabe
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in, err := cleanAusgabe(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	year := a.yearParam(r)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if _, ok := a.st.d.Jahre[yearKey(year)]; !ok {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	in.ID = newID()
	j := a.st.ensureYear(year)
	j.Ausgaben = append(j.Ausgaben, in)
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, in)
}

func (a *App) handleAusgabeUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in Ausgabe
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in, err := cleanAusgabe(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	year := a.yearParam(r)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	j := a.st.d.Jahre[yearKey(year)]
	if j == nil {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	for i := range j.Ausgaben {
		if j.Ausgaben[i].ID == id {
			in.ID = id
			in.Beleg = j.Ausgaben[i].Beleg                                                // wird nur über die eigenen Beleg-Endpunkte geändert
			in.Geprueft, in.GeprueftAm = j.Ausgaben[i].Geprueft, j.Ausgaben[i].GeprueftAm // nur über den eigenen Endpunkt
			j.Ausgaben[i] = in
			if err := a.st.saveLocked(); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, in)
			return
		}
	}
	writeErr(w, notFound("Ausgabe nicht gefunden"))
}

func (a *App) handleAusgabeDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	year := a.yearParam(r)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	j := a.st.d.Jahre[yearKey(year)]
	if j == nil {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	for i := range j.Ausgaben {
		if j.Ausgaben[i].ID == id {
			if j.Ausgaben[i].Beleg != "" {
				_ = os.Remove(filepath.Join(a.st.dir, filepath.FromSlash(j.Ausgaben[i].Beleg)))
			}
			j.Ausgaben = append(j.Ausgaben[:i], j.Ausgaben[i+1:]...)
			if err := a.st.saveLocked(); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, map[string]bool{"ok": true})
			return
		}
	}
	writeErr(w, notFound("Ausgabe nicht gefunden"))
}

// handleAusgabeGeprueft setzt oder entfernt den Kassenprüfer-Haken einer Ausgabe.
func (a *App) handleAusgabeGeprueft(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Geprueft bool `json:"geprueft"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	year := a.yearParam(r)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	j := a.st.d.Jahre[yearKey(year)]
	if j == nil {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	for i := range j.Ausgaben {
		if j.Ausgaben[i].ID == id {
			j.Ausgaben[i].Geprueft = in.Geprueft
			if in.Geprueft {
				j.Ausgaben[i].GeprueftAm = time.Now().Format("2006-01-02")
			} else {
				j.Ausgaben[i].GeprueftAm = ""
			}
			if err := a.st.saveLocked(); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, j.Ausgaben[i])
			return
		}
	}
	writeErr(w, notFound("Ausgabe nicht gefunden"))
}

// ---------------------------------------------------------------- Beleg-Anhang

// belegExtensions sind die erlaubten Dateitypen für Belege (Fotos und PDF-Scans).
var belegExtensions = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".pdf": true}

const maxBelegSize = 12 << 20 // 12 MB

// handleBelegUpload speichert einen Beleg (Foto oder PDF) zu einer Ausgabe.
func (a *App) handleBelegUpload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	year := a.yearParam(r)
	r.Body = http.MaxBytesReader(w, r.Body, maxBelegSize)
	if err := r.ParseMultipartForm(maxBelegSize); err != nil {
		writeErr(w, bad("Die Datei ist zu groß oder konnte nicht gelesen werden (höchstens 12 MB)"))
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, bad("Bitte eine Datei auswählen"))
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	if !belegExtensions[ext] {
		writeErr(w, bad("Bitte ein Foto (JPG, PNG, WebP) oder eine PDF-Datei auswählen"))
		return
	}
	data, err := io.ReadAll(file)
	if err != nil {
		writeErr(w, bad("Datei konnte nicht gelesen werden"))
		return
	}

	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	j := a.st.d.Jahre[yearKey(year)]
	if j == nil {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	idx := -1
	for i := range j.Ausgaben {
		if j.Ausgaben[i].ID == id {
			idx = i
		}
	}
	if idx < 0 {
		writeErr(w, notFound("Ausgabe nicht gefunden"))
		return
	}
	dir := filepath.Join(a.st.dir, "Belege", yearKey(year))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		writeErr(w, err)
		return
	}
	if old := j.Ausgaben[idx].Beleg; old != "" {
		_ = os.Remove(filepath.Join(a.st.dir, filepath.FromSlash(old)))
	}
	name := safeName(id, 20) + ext
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		writeErr(w, err)
		return
	}
	j.Ausgaben[idx].Beleg = filepath.ToSlash(filepath.Join("Belege", yearKey(year), name))
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, j.Ausgaben[idx])
}

func (a *App) handleBelegDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	year := a.yearParam(r)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	j := a.st.d.Jahre[yearKey(year)]
	if j == nil {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	for i := range j.Ausgaben {
		if j.Ausgaben[i].ID == id {
			if j.Ausgaben[i].Beleg != "" {
				_ = os.Remove(filepath.Join(a.st.dir, filepath.FromSlash(j.Ausgaben[i].Beleg)))
				j.Ausgaben[i].Beleg = ""
			}
			if err := a.st.saveLocked(); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, j.Ausgaben[i])
			return
		}
	}
	writeErr(w, notFound("Ausgabe nicht gefunden"))
}

// handleBelegServe liefert die Beleg-Datei einer Ausgabe aus.
func (a *App) handleBelegServe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	year := a.yearParam(r)
	a.st.mu.Lock()
	var path string
	j := a.st.d.Jahre[yearKey(year)]
	if j != nil {
		for _, x := range j.Ausgaben {
			if x.ID == id {
				path = x.Beleg
			}
		}
	}
	dir := a.st.dir
	a.st.mu.Unlock()
	if path == "" {
		writeErr(w, notFound("Für diese Ausgabe ist kein Beleg hinterlegt"))
		return
	}
	full := filepath.Join(dir, filepath.FromSlash(path))
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, full)
}

// ---------------------------------------------------------------- Kassenbericht

// Versorger sind die Werte der Hauptzähler bzw. der Rechnungen des Versorgers
// für ein Jahr. Sie dienen nur dem Vergleich im Kassenbericht.
type Versorger struct {
	WasserM3  *float64 `json:"wasserM3"`
	WasserEUR *float64 `json:"wasserEUR"`
	StromKWh  *float64 `json:"stromKWh"`
	StromEUR  *float64 `json:"stromEUR"`
}

// kategorieSumme ist die Summe der sonstigen Ausgaben einer Kategorie.
type kategorieSumme struct {
	Kategorie string  `json:"kategorie"`
	Summe     float64 `json:"summe"`
}

type kassenbericht struct {
	Jahr            int      `json:"jahr"`
	Paechter        int      `json:"paechter"`
	Ausgestellt     int      `json:"ausgestellt"`    // Werte aus ausgestellten Rechnungen
	Berechnet       int      `json:"berechnet"`      // vollständig, aber noch nicht ausgestellt
	Unvollstaendig  []string `json:"unvollstaendig"` // nicht enthalten
	WasserVerbrauch float64  `json:"wasserVerbrauch"`
	StromVerbrauch  float64  `json:"stromVerbrauch"`
	Summen          Result   `json:"summen"` // Summe je Posten aller einbezogenen Rechnungen
	Versicherung    float64  `json:"versicherung"`
	Grundsteuer     float64  `json:"grundsteuer"`
	Auslagen        float64  `json:"auslagen"`
	Abschlag        float64  `json:"abschlag"`

	Versorger Versorger `json:"versorger"`

	// Tatsächlich geflossenes Geld (nach dem heutigen Zahlungsstand)
	EinnahmenBezahlt   float64 `json:"einnahmenBezahlt"`   // von Pächtern eingegangen
	GuthabenAusgezahlt float64 `json:"guthabenAusgezahlt"` // an Pächter ausgezahlte Guthaben

	// Sonstige Ausgaben der Vereinskasse (Kontoführung, Anschaffungen, Reparaturen, ...)
	Ausgaben          []Ausgabe        `json:"ausgaben"`
	AusgabenSumme     float64          `json:"ausgabenSumme"`
	AusgabenKategorie []kategorieSumme `json:"ausgabenKategorie"`

	// Saldo = EinnahmenBezahlt - GuthabenAusgezahlt - AusgabenSumme: die Kassenbewegung
	// dieses Jahres. Anfangsbestand ist der Kassenbestand zu Jahresbeginn (von Hand
	// gepflegt bzw. beim Jahreswechsel aus dem Vorjahr übernommen), Kassenbestand ist
	// Anfangsbestand + Saldo – der tatsächliche Kontostand am heutigen Tag.
	Saldo          float64 `json:"saldo"`
	Anfangsbestand float64 `json:"anfangsbestand"`
	Kassenbestand  float64 `json:"kassenbestand"`
}

// kassenberichtLocked summiert alle Posten eines Jahres. Grundlage der Beträge je
// Pächter ist die gültige ausgestellte Rechnung, sonst die aktuelle Berechnung.
// Der Aufrufer hält s.mu.
func (s *Store) kassenberichtLocked(v yearView) kassenbericht {
	k := kassenbericht{Jahr: v.Jahr, Unvollstaendig: []string{}, Ausgaben: []Ausgabe{}, AusgabenKategorie: []kategorieSumme{}}
	if j := s.d.Jahre[yearKey(v.Jahr)]; j != nil {
		if j.Versorger != nil {
			k.Versorger = *j.Versorger
		}
		if j.Anfangsbestand != nil {
			k.Anfangsbestand = *j.Anfangsbestand
		}
		k.Ausgaben = append(k.Ausgaben, j.Ausgaben...)
	}
	sort.SliceStable(k.Ausgaben, func(i, j int) bool { return k.Ausgaben[i].Datum < k.Ausgaben[j].Datum })
	kat := map[string]float64{}
	var katNamen []string
	for _, x := range k.Ausgaben {
		k.AusgabenSumme += x.Betrag
		if _, ok := kat[x.Kategorie]; !ok {
			katNamen = append(katNamen, x.Kategorie)
		}
		kat[x.Kategorie] += x.Betrag
	}
	sort.Strings(katNamen)
	for _, name := range katNamen {
		k.AusgabenKategorie = append(k.AusgabenKategorie, kategorieSumme{Kategorie: name, Summe: round2(kat[name])})
	}

	add := func(a Ablesung, r Result) {
		t := &k.Summen
		if r.WasserVerbrauch != nil {
			k.WasserVerbrauch += *r.WasserVerbrauch
		}
		if r.EnergieVerbrauch != nil {
			k.StromVerbrauch += *r.EnergieVerbrauch
		}
		t.KostenWasser += r.KostenWasser
		t.KostenEnergie += r.KostenEnergie
		t.NachzahlungStunden += r.NachzahlungStunden
		t.Zwischensumme1 += r.Zwischensumme1
		t.PachtGarten += r.PachtGarten
		t.PachtVerein += r.PachtVerein
		t.PachtFrei += r.PachtFrei
		t.Mitgliedsbeitrag += r.Mitgliedsbeitrag
		t.Umlage += r.Umlage
		t.Zwischensumme2 += r.Zwischensumme2
		t.Verguetung += r.Verguetung
		t.Gesamt += r.Gesamt
		k.Versicherung += a.Versicherung
		k.Grundsteuer += a.Grundsteuer
		k.Auslagen += a.Auslagen
		k.Abschlag += a.Abschlag
	}
	seen := map[string]bool{}
	for _, inv := range s.d.Rechnungen {
		if inv.Jahr != v.Jahr || inv.Status != statusGueltig {
			continue
		}
		add(inv.Ablesung, inv.Result)
		seen[inv.PaechterID] = true
		k.Ausgestellt++
		if inv.Result.Gesamt >= 0 {
			k.EinnahmenBezahlt += paidAmount(inv)
		} else {
			k.GuthabenAusgezahlt += paidAmount(inv)
		}
	}
	list := append([]Paechter(nil), v.Paechter...)
	sort.SliceStable(list, func(i, j int) bool { return natLess(list[i].Mitgliedsnr, list[j].Mitgliedsnr) })
	for _, p := range list {
		if seen[p.ID] {
			continue
		}
		a := v.Ablesungen[p.ID]
		r := calculate(v.Settings, p, a)
		if !r.Vollstaendig {
			k.Unvollstaendig = append(k.Unvollstaendig, p.Mitgliedsnr+" "+p.Name)
			continue
		}
		add(a, r)
		k.Berechnet++
	}
	k.Paechter = k.Ausgestellt + k.Berechnet + len(k.Unvollstaendig)
	k.Saldo = k.EinnahmenBezahlt - k.GuthabenAusgezahlt - k.AusgabenSumme
	k.Kassenbestand = k.Anfangsbestand + k.Saldo
	for _, x := range []*float64{&k.WasserVerbrauch, &k.StromVerbrauch, &k.Summen.KostenWasser, &k.Summen.KostenEnergie,
		&k.Summen.NachzahlungStunden, &k.Summen.Zwischensumme1, &k.Summen.PachtGarten, &k.Summen.PachtVerein, &k.Summen.PachtFrei,
		&k.Summen.Mitgliedsbeitrag, &k.Summen.Umlage, &k.Summen.Zwischensumme2, &k.Summen.Verguetung, &k.Summen.Gesamt,
		&k.Versicherung, &k.Grundsteuer, &k.Auslagen, &k.Abschlag, &k.EinnahmenBezahlt, &k.GuthabenAusgezahlt, &k.AusgabenSumme, &k.Saldo,
		&k.Kassenbestand} {
		*x = round2(*x)
	}
	return k
}

func (a *App) handleKassenbericht(w http.ResponseWriter, r *http.Request) {
	year := a.yearParam(r)
	a.st.mu.Lock()
	v, ok := a.st.viewLocked(year)
	var k kassenbericht
	if ok {
		k = a.st.kassenberichtLocked(v)
	}
	a.st.mu.Unlock()
	if !ok {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	writeJSON(w, 200, k)
}

// handleVersorger speichert die Hauptzähler- und Versorgerwerte eines Jahres. Das geht
// auch für abgeschlossene Jahre, weil die Versorgerrechnung oft erst später kommt.
func (a *App) handleVersorger(w http.ResponseWriter, r *http.Request) {
	var in Versorger
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	for _, p := range []*float64{in.WasserM3, in.WasserEUR, in.StromKWh, in.StromEUR} {
		if !validNumPtr(p) {
			writeErr(w, bad("Bitte nur Zahlen ab 0 eingeben"))
			return
		}
	}
	year := a.yearParam(r)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	j := a.st.d.Jahre[yearKey(year)]
	if j == nil {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	j.Versorger = &in
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	v, _ := a.st.viewLocked(year)
	writeJSON(w, 200, a.st.kassenberichtLocked(v))
}

// handleAnfangsbestand setzt den Kassenbestand zu Jahresbeginn von Hand.
func (a *App) handleAnfangsbestand(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Betrag *float64 `json:"betrag"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if !validNumPtr(in.Betrag) {
		writeErr(w, bad("Bitte eine Zahl ab 0 eingeben"))
		return
	}
	year := a.yearParam(r)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	j := a.st.d.Jahre[yearKey(year)]
	if j == nil {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	j.Anfangsbestand = in.Betrag
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	v, _ := a.st.viewLocked(year)
	writeJSON(w, 200, a.st.kassenberichtLocked(v))
}

func (a *App) handleKassenberichtExport(w http.ResponseWriter, r *http.Request) {
	year := a.yearParam(r)
	a.st.mu.Lock()
	v, ok := a.st.viewLocked(year)
	var k kassenbericht
	if ok {
		k = a.st.kassenberichtLocked(v)
	}
	vereinsname := v.Settings.VereinName
	a.st.mu.Unlock()
	if !ok {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	data, err := writeXLSX("Kassenbericht "+strconv.Itoa(year), []float64{44, 16, 16, 16, 14}, kassenberichtRows(k, vereinsname))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Kassenbericht_%d.xlsx"`, year))
	_, _ = w.Write(data)
}

func kassenberichtRows(k kassenbericht, verein string) [][]xCell {
	s := k.Summen
	rows := [][]xCell{
		{{fmt.Sprintf("Kassenbericht %d – %s", k.Jahr, verein), stBold}},
		{{fmt.Sprintf("%d Pächter: %d aus ausgestellten Rechnungen, %d berechnet (noch nicht ausgestellt), %d unvollständig (nicht enthalten)",
			k.Paechter, k.Ausgestellt, k.Berechnet, len(k.Unvollstaendig)), stNormal}},
		{},
		{{"Abrechnung der Pächter (in Rechnung gestellt)", stHeader}, {"Betrag (€)", stHeader}},
	}
	posten := []struct {
		name string
		v    float64
	}{
		{"Wasser", s.KostenWasser}, {"Energie", s.KostenEnergie}, {"Nachzahlung fehlende Arbeitsstunden", s.NachzahlungStunden},
		{"Pacht Garten", s.PachtGarten}, {"Pacht Vereinsfläche", s.PachtVerein}, {"Pacht freie Gärten", s.PachtFrei},
		{"Mitgliedsbeitrag", s.Mitgliedsbeitrag}, {"Umlage", s.Umlage}, {"Versicherung", k.Versicherung},
		{"Grundsteuer", k.Grundsteuer}, {"Sonstige Auslagen", k.Auslagen},
		{"abzüglich Vergütung Arbeitsstunden", -s.Verguetung}, {"abzüglich Abschlagszahlungen", -k.Abschlag},
	}
	for _, p := range posten {
		rows = append(rows, []xCell{{p.name, stNormal}, {p.v, stEUR}})
	}
	rows = append(rows,
		[]xCell{{"Gesamtbetrag aller Rechnungen", stBold}, {s.Gesamt, stEURb}},
		[]xCell{},
		[]xCell{{"Vergleich mit dem Versorger", stHeader}, {"Pächter gesamt", stHeader}, {"Hauptzähler / Versorger", stHeader}, {"Differenz", stHeader}},
	)
	opt := func(p *float64) any {
		if p == nil {
			return nil
		}
		return *p
	}
	diff := func(total float64, p *float64) any {
		if p == nil {
			return nil
		}
		return round2(*p - total)
	}
	rows = append(rows,
		[]xCell{{"Wasser (m³)", stNormal}, {k.WasserVerbrauch, stNum}, {opt(k.Versorger.WasserM3), stNum}, {diff(k.WasserVerbrauch, k.Versorger.WasserM3), stNum}},
		[]xCell{{"Wasser (€)", stNormal}, {s.KostenWasser, stEUR}, {opt(k.Versorger.WasserEUR), stEUR}, {diff(s.KostenWasser, k.Versorger.WasserEUR), stEUR}},
		[]xCell{{"Strom (kWh)", stNormal}, {k.StromVerbrauch, stNum}, {opt(k.Versorger.StromKWh), stNum}, {diff(k.StromVerbrauch, k.Versorger.StromKWh), stNum}},
		[]xCell{{"Strom (€)", stNormal}, {s.KostenEnergie, stEUR}, {opt(k.Versorger.StromEUR), stEUR}, {diff(s.KostenEnergie, k.Versorger.StromEUR), stEUR}},
		[]xCell{},
		[]xCell{{"Sonstige Ausgaben der Vereinskasse", stHeader}, {"Datum", stHeader}, {"Kategorie", stHeader}, {"Betrag (€)", stHeader}, {"Geprüft", stHeader}},
	)
	for _, x := range k.Ausgaben {
		geprueft := ""
		if x.Geprueft {
			geprueft = "✓ " + germanDate(x.GeprueftAm)
		}
		rows = append(rows, []xCell{{x.Beschreibung, stNormal}, {germanDate(x.Datum), stNormal}, {x.Kategorie, stNormal}, {x.Betrag, stEUR}, {geprueft, stNormal}})
	}
	rows = append(rows, []xCell{{"Summe sonstige Ausgaben", stBold}, {}, {}, {k.AusgabenSumme, stEURb}})
	if len(k.AusgabenKategorie) > 1 {
		rows = append(rows, []xCell{}, []xCell{{"davon nach Kategorie", stHeader}, {"Betrag (€)", stHeader}})
		for _, kat := range k.AusgabenKategorie {
			rows = append(rows, []xCell{{kat.Kategorie, stNormal}, {kat.Summe, stEUR}})
		}
	}
	rows = append(rows,
		[]xCell{},
		[]xCell{{"Kassenbestand", stHeader}, {"Betrag (€)", stHeader}},
		[]xCell{{"Anfangsbestand", stNormal}, {k.Anfangsbestand, stEUR}},
		[]xCell{{"+ Von Pächtern eingegangene Zahlungen", stNormal}, {k.EinnahmenBezahlt, stEUR}},
		[]xCell{{"− An Pächter ausgezahlte Guthaben", stNormal}, {-k.GuthabenAusgezahlt, stEUR}},
		[]xCell{{"− Sonstige Ausgaben", stNormal}, {-k.AusgabenSumme, stEUR}},
		[]xCell{{"Kassenbestand (Kontostand heute)", stBold}, {k.Kassenbestand, stEURb}},
	)
	if len(k.Unvollstaendig) > 0 {
		rows = append(rows, []xCell{}, []xCell{{"Nicht in der Abrechnung enthalten (unvollständig)", stHeader}})
		for _, n := range k.Unvollstaendig {
			rows = append(rows, []xCell{{n, stNormal}})
		}
	}
	return rows
}

// ---------------------------------------------------------------- Mehrjahresvergleich

// kassenberichtJahr ist die Kurzfassung eines Kassenberichts für den Vergleich
// über mehrere Jahre (Verein insgesamt, nicht je Pächter).
type kassenberichtJahr struct {
	Jahr             int     `json:"jahr"`
	Rechnungssumme   float64 `json:"rechnungssumme"`   // Summe aller Pächterrechnungen (in Rechnung gestellt)
	EinnahmenBezahlt float64 `json:"einnahmenBezahlt"` // tatsächlich eingegangen
	AusgabenSumme    float64 `json:"ausgabenSumme"`
	Anfangsbestand   float64 `json:"anfangsbestand"`
	Kassenbestand    float64 `json:"kassenbestand"`
}

// kassenberichtVerlaufLocked liefert die Kurzfassung aller Jahre, neuestes zuerst.
// Der Aufrufer hält s.mu.
func (s *Store) kassenberichtVerlaufLocked() []kassenberichtJahr {
	var out []kassenberichtJahr
	for _, y := range s.years() {
		v, ok := s.viewLocked(y)
		if !ok {
			continue
		}
		k := s.kassenberichtLocked(v)
		out = append(out, kassenberichtJahr{
			Jahr: y, Rechnungssumme: k.Summen.Gesamt, EinnahmenBezahlt: k.EinnahmenBezahlt,
			AusgabenSumme: k.AusgabenSumme, Anfangsbestand: k.Anfangsbestand, Kassenbestand: k.Kassenbestand,
		})
	}
	return out
}

func (a *App) handleKassenberichtVerlauf(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	out := a.st.kassenberichtVerlaufLocked()
	a.st.mu.Unlock()
	writeJSON(w, 200, out)
}
