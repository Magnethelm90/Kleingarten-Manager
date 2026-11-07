package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Schlüssel und Geräte des Vereins: wer welchen Tor-, Vereinsheim- oder Schachtschlüssel hat (mit Pfand und
// Verlauf), und welche Vereinsgeräte (Häcksler, Rasenmäher, Leiter ...) gerade verliehen sind oder zur
// Wartung müssen. Das Cockpit erinnert an Schlüssel ausgeschiedener Pächter, überfällige Rückgaben und
// fällige Wartungen. Namen im Verlauf gehören zum Pächter und werden mit ihm gelöscht.

const (
	maxInventar      = 1000
	maxVerlauf       = 50
	geloeschtePerson = "(gelöschte Person)"
)

// Vorgang ist ein Eintrag im Verlauf eines Schlüssels oder Geräts.
type Vorgang struct {
	Datum      string `json:"datum"`
	Aktion     string `json:"aktion"` // ausgegeben | zurück | verliehen | Wartung
	Name       string `json:"name,omitempty"`
	PaechterID string `json:"paechterId,omitempty"`
	Notiz      string `json:"notiz,omitempty"`
}

type Schluessel struct {
	ID          string  `json:"id"`
	Nummer      string  `json:"nummer"`
	Art         string  `json:"art"`
	Bezeichnung string  `json:"bezeichnung,omitempty"`
	Pfand       float64 `json:"pfand,omitempty"`
	Notiz       string  `json:"notiz,omitempty"`
	// aktueller Inhaber: ein Pächter (PaechterID) oder jemand anderes (Inhaber, z. B. »Vorstand«)
	PaechterID string    `json:"paechterId,omitempty"`
	Inhaber    string    `json:"inhaber,omitempty"`
	Seit       string    `json:"seit,omitempty"`
	Verlauf    []Vorgang `json:"verlauf,omitempty"`
}

type Geraet struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Inventarnr  string  `json:"inventarnr,omitempty"`
	Standort    string  `json:"standort,omitempty"`
	Angeschafft string  `json:"angeschafft,omitempty"`
	Wert        float64 `json:"wert,omitempty"`
	Notiz       string  `json:"notiz,omitempty"`
	// Wartung alle WartungMonate Monate (0 = keine regelmäßige Wartung)
	WartungMonate int    `json:"wartungMonate,omitempty"`
	LetzteWartung string `json:"letzteWartung,omitempty"`
	// Ausleihe: verliehen an Pächter oder Name, seit wann, zurück bis
	PaechterID string    `json:"paechterId,omitempty"`
	Entleiher  string    `json:"entleiher,omitempty"`
	Seit       string    `json:"seit,omitempty"`
	Bis        string    `json:"bis,omitempty"`
	Verlauf    []Vorgang `json:"verlauf,omitempty"`
}

var schluesselArten = []string{"Tor", "Vereinsheim", "Wasserschacht", "Stromkasten", "Geräteschuppen", "Sonstiges"}

func gueltigesDatum(s string, leerErlaubt bool) bool {
	if s == "" {
		return leerErlaubt
	}
	_, err := parseDate(s)
	return err == nil
}

func neuerVorgang(l []Vorgang, v Vorgang) []Vorgang {
	l = append(l, v)
	if len(l) > maxVerlauf {
		l = l[len(l)-maxVerlauf:]
	}
	return l
}

// inhaberName: Name des Pächters (auch ausgeschiedener) oder der frei eingetragene Name.
func (s *Store) inhaberName(paechterID, sonst string) (string, bool) {
	if paechterID == "" {
		return sonst, false
	}
	for _, p := range s.d.Paechter {
		if p.ID == paechterID {
			return p.Name, p.AusgeschiedenAm != "" || p.Geloescht
		}
	}
	return geloeschtePerson, true
}

// verlaufMitNamen: Kopie des Verlaufs mit den Namen der Pächter (gespeichert ist nur die ID). Der Aufrufer hält s.mu.
func (s *Store) verlaufMitNamen(l []Vorgang) []Vorgang {
	out := make([]Vorgang, len(l))
	for i, v := range l {
		if v.Name == "" && v.PaechterID != "" {
			v.Name, _ = s.inhaberName(v.PaechterID, "")
		}
		out[i] = v
	}
	return out
}

// ------------------------------------------------------------------ Schlüssel

type schluesselAnsicht struct {
	Schluessel
	InhaberName   string `json:"inhaberName,omitempty"`
	Gartennr      string `json:"gartennr,omitempty"`
	Ausgeschieden bool   `json:"ausgeschieden,omitempty"`
}

func (s *Store) gartennrVon(id string) string {
	for _, p := range s.d.Paechter {
		if p.ID == id {
			return p.Gartennr
		}
	}
	return ""
}

func (a *App) handleSchluesselListe(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	out := []schluesselAnsicht{}
	for _, k := range a.st.d.Schluessel {
		v := schluesselAnsicht{Schluessel: k}
		v.Verlauf = a.st.verlaufMitNamen(k.Verlauf)
		v.InhaberName, v.Ausgeschieden = a.st.inhaberName(k.PaechterID, k.Inhaber)
		v.Gartennr = a.st.gartennrVon(k.PaechterID)
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Art != out[j].Art {
			return out[i].Art < out[j].Art
		}
		return naturalLess(out[i].Nummer, out[j].Nummer)
	})
	writeJSON(w, 200, map[string]any{"schluessel": out, "arten": schluesselArten})
}

func (a *App) handleSchluesselSpeichern(w http.ResponseWriter, r *http.Request) {
	var in Schluessel
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in.Nummer, in.Art = trim(in.Nummer, 30), trim(in.Art, 40)
	in.Bezeichnung, in.Notiz = trim(in.Bezeichnung, 100), trim(in.Notiz, 500)
	if in.Nummer == "" || in.Art == "" {
		writeErr(w, bad("Bitte Nummer und Art des Schlüssels angeben"))
		return
	}
	if !validNum(in.Pfand) {
		writeErr(w, bad("Das Pfand ist ungültig"))
		return
	}
	in.Pfand = round2(in.Pfand)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Schluessel {
		k := &a.st.d.Schluessel[i]
		if k.ID != in.ID && k.Art == in.Art && strings.EqualFold(k.Nummer, in.Nummer) {
			writeErr(w, bad(fmt.Sprintf("Den %sschlüssel Nr. %s gibt es schon", strings.ToLower(in.Art), in.Nummer)))
			return
		}
	}
	for i := range a.st.d.Schluessel {
		k := &a.st.d.Schluessel[i]
		if in.ID != "" && k.ID == in.ID {
			// Inhaber und Verlauf ändern sich nur über Ausgabe und Rückgabe
			k.Nummer, k.Art, k.Bezeichnung, k.Pfand, k.Notiz = in.Nummer, in.Art, in.Bezeichnung, in.Pfand, in.Notiz
			a.st.audit("Schlüssel geändert: %s %s", k.Art, k.Nummer)
			a.speichernUndAntworten(w, k)
			return
		}
	}
	if len(a.st.d.Schluessel) >= maxInventar {
		writeErr(w, bad("Zu viele Schlüssel"))
		return
	}
	k := Schluessel{ID: newID(), Nummer: in.Nummer, Art: in.Art, Bezeichnung: in.Bezeichnung, Pfand: in.Pfand, Notiz: in.Notiz}
	a.st.d.Schluessel = append(a.st.d.Schluessel, k)
	a.st.audit("Schlüssel angelegt: %s %s", k.Art, k.Nummer)
	a.speichernUndAntworten(w, k)
}

func (a *App) handleSchluesselLoeschen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i, k := range a.st.d.Schluessel {
		if k.ID == id {
			a.st.d.Schluessel = append(a.st.d.Schluessel[:i], a.st.d.Schluessel[i+1:]...)
			a.st.audit("Schlüssel gelöscht: %s %s", k.Art, k.Nummer)
			a.speichernUndAntworten(w, map[string]bool{"ok": true})
			return
		}
	}
	writeErr(w, notFound("Schlüssel nicht gefunden"))
}

type uebergabeEingabe struct {
	PaechterID string `json:"paechterId"`
	Name       string `json:"name"`
	Datum      string `json:"datum"`
	Bis        string `json:"bis"`
	Notiz      string `json:"notiz"`
}

// pruefeEmpfaenger: ein aktiver Pächter oder ein frei eingetragener Name. Liefert den Namen. Der Aufrufer hält s.mu.
func (s *Store) pruefeEmpfaenger(in *uebergabeEingabe) (string, error) {
	in.Name = trim(in.Name, 80)
	if in.PaechterID != "" {
		for _, p := range s.d.Paechter {
			if p.ID == in.PaechterID && !p.Geloescht && p.AusgeschiedenAm == "" {
				in.Name = ""
				return p.Name, nil
			}
		}
		return "", bad("Pächter nicht gefunden")
	}
	if in.Name == "" {
		return "", bad("Bitte angeben, wer den Schlüssel bekommt")
	}
	return in.Name, nil
}

func (a *App) handleSchluesselAusgabe(w http.ResponseWriter, r *http.Request) {
	var in uebergabeEingabe
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if in.Datum == "" {
		in.Datum = time.Now().Format("2006-01-02")
	}
	if !gueltigesDatum(in.Datum, false) {
		writeErr(w, bad("Das Datum ist ungültig"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	name, err := a.st.pruefeEmpfaenger(&in)
	if err != nil {
		writeErr(w, err)
		return
	}
	for i := range a.st.d.Schluessel {
		k := &a.st.d.Schluessel[i]
		if k.ID != r.PathValue("id") {
			continue
		}
		if k.PaechterID != "" || k.Inhaber != "" {
			writeErr(w, bad("Der Schlüssel ist noch ausgegeben. Bitte zuerst die Rückgabe eintragen"))
			return
		}
		k.PaechterID, k.Inhaber, k.Seit = in.PaechterID, in.Name, in.Datum
		k.Verlauf = neuerVorgang(k.Verlauf, Vorgang{Datum: in.Datum, Aktion: "ausgegeben", Name: in.Name, PaechterID: in.PaechterID, Notiz: trim(in.Notiz, 200)})
		a.st.audit("Schlüssel %s %s ausgegeben an %s", k.Art, k.Nummer, name)
		a.speichernUndAntworten(w, k)
		return
	}
	writeErr(w, notFound("Schlüssel nicht gefunden"))
}

func (a *App) handleSchluesselRueckgabe(w http.ResponseWriter, r *http.Request) {
	var in uebergabeEingabe
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if in.Datum == "" {
		in.Datum = time.Now().Format("2006-01-02")
	}
	if !gueltigesDatum(in.Datum, false) {
		writeErr(w, bad("Das Datum ist ungültig"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Schluessel {
		k := &a.st.d.Schluessel[i]
		if k.ID != r.PathValue("id") {
			continue
		}
		if k.PaechterID == "" && k.Inhaber == "" {
			writeErr(w, bad("Der Schlüssel ist nicht ausgegeben"))
			return
		}
		name, _ := a.st.inhaberName(k.PaechterID, k.Inhaber)
		k.Verlauf = neuerVorgang(k.Verlauf, Vorgang{Datum: in.Datum, Aktion: "zurück", Name: k.Inhaber, PaechterID: k.PaechterID, Notiz: trim(in.Notiz, 200)})
		k.PaechterID, k.Inhaber, k.Seit = "", "", ""
		a.st.audit("Schlüssel %s %s zurückgegeben von %s", k.Art, k.Nummer, name)
		a.speichernUndAntworten(w, k)
		return
	}
	writeErr(w, notFound("Schlüssel nicht gefunden"))
}

// ------------------------------------------------------------------ Geräte

type geraetAnsicht struct {
	Geraet
	EntleiherName  string `json:"entleiherName,omitempty"`
	WartungFaellig string `json:"wartungFaellig,omitempty"` // Datum der nächsten Wartung
}

// naechsteWartung: Datum der fälligen Wartung (leer = keine regelmäßige Wartung).
func (g Geraet) naechsteWartung() string {
	if g.WartungMonate <= 0 {
		return ""
	}
	basis := g.LetzteWartung
	if basis == "" {
		basis = g.Angeschafft
	}
	t, err := parseDate(basis)
	if err != nil {
		return time.Now().Format("2006-01-02")
	}
	return t.AddDate(0, g.WartungMonate, 0).Format("2006-01-02")
}

func (a *App) handleGeraeteListe(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	out := []geraetAnsicht{}
	for _, g := range a.st.d.Geraete {
		v := geraetAnsicht{Geraet: g, WartungFaellig: g.naechsteWartung()}
		v.Verlauf = a.st.verlaufMitNamen(g.Verlauf)
		v.EntleiherName, _ = a.st.inhaberName(g.PaechterID, g.Entleiher)
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	writeJSON(w, 200, out)
}

func (a *App) handleGeraetSpeichern(w http.ResponseWriter, r *http.Request) {
	var in Geraet
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in.Name, in.Inventarnr, in.Standort = trim(in.Name, 80), trim(in.Inventarnr, 30), trim(in.Standort, 80)
	in.Notiz = trim(in.Notiz, 500)
	if in.Name == "" {
		writeErr(w, bad("Bitte den Namen des Geräts angeben"))
		return
	}
	if !validNum(in.Wert) || in.WartungMonate < 0 || in.WartungMonate > 120 || !gueltigesDatum(in.Angeschafft, true) || !gueltigesDatum(in.LetzteWartung, true) {
		writeErr(w, bad("Bitte Wert, Datum und Wartungsabstand prüfen"))
		return
	}
	in.Wert = round2(in.Wert)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Geraete {
		g := &a.st.d.Geraete[i]
		if in.ID != "" && g.ID == in.ID {
			g.Name, g.Inventarnr, g.Standort, g.Angeschafft, g.Wert, g.Notiz = in.Name, in.Inventarnr, in.Standort, in.Angeschafft, in.Wert, in.Notiz
			g.WartungMonate, g.LetzteWartung = in.WartungMonate, in.LetzteWartung
			a.st.audit("Gerät geändert: %s", g.Name)
			a.speichernUndAntworten(w, g)
			return
		}
	}
	if len(a.st.d.Geraete) >= maxInventar {
		writeErr(w, bad("Zu viele Geräte"))
		return
	}
	g := Geraet{ID: newID(), Name: in.Name, Inventarnr: in.Inventarnr, Standort: in.Standort, Angeschafft: in.Angeschafft, Wert: in.Wert,
		Notiz: in.Notiz, WartungMonate: in.WartungMonate, LetzteWartung: in.LetzteWartung}
	a.st.d.Geraete = append(a.st.d.Geraete, g)
	a.st.audit("Gerät angelegt: %s", g.Name)
	a.speichernUndAntworten(w, g)
}

func (a *App) handleGeraetLoeschen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i, g := range a.st.d.Geraete {
		if g.ID == id {
			a.st.d.Geraete = append(a.st.d.Geraete[:i], a.st.d.Geraete[i+1:]...)
			a.st.audit("Gerät gelöscht: %s", g.Name)
			a.speichernUndAntworten(w, map[string]bool{"ok": true})
			return
		}
	}
	writeErr(w, notFound("Gerät nicht gefunden"))
}

// handleGeraetVorgang: verleihen, zurück oder Wartung erledigt.
func (a *App) handleGeraetVorgang(aktion string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in uebergabeEingabe
		if err := readJSON(w, r, &in); err != nil {
			writeErr(w, err)
			return
		}
		if in.Datum == "" {
			in.Datum = time.Now().Format("2006-01-02")
		}
		if !gueltigesDatum(in.Datum, false) || !gueltigesDatum(in.Bis, true) || (in.Bis != "" && in.Bis < in.Datum) {
			writeErr(w, bad("Bitte die Daten prüfen"))
			return
		}
		in.Notiz = trim(in.Notiz, 200)
		a.st.mu.Lock()
		defer a.st.mu.Unlock()
		var g *Geraet
		for i := range a.st.d.Geraete {
			if a.st.d.Geraete[i].ID == r.PathValue("id") {
				g = &a.st.d.Geraete[i]
			}
		}
		if g == nil {
			writeErr(w, notFound("Gerät nicht gefunden"))
			return
		}
		verliehen := g.PaechterID != "" || g.Entleiher != ""
		switch aktion {
		case "verliehen":
			if verliehen {
				writeErr(w, bad("Das Gerät ist noch verliehen. Bitte zuerst die Rückgabe eintragen"))
				return
			}
			name, err := a.st.pruefeEmpfaenger(&in)
			if err != nil {
				writeErr(w, err)
				return
			}
			g.PaechterID, g.Entleiher, g.Seit, g.Bis = in.PaechterID, in.Name, in.Datum, in.Bis
			g.Verlauf = neuerVorgang(g.Verlauf, Vorgang{Datum: in.Datum, Aktion: "verliehen", Name: in.Name, PaechterID: in.PaechterID, Notiz: in.Notiz})
			a.st.audit("Gerät %s verliehen an %s", g.Name, name)
		case "zurück":
			if !verliehen {
				writeErr(w, bad("Das Gerät ist nicht verliehen"))
				return
			}
			name, _ := a.st.inhaberName(g.PaechterID, g.Entleiher)
			g.Verlauf = neuerVorgang(g.Verlauf, Vorgang{Datum: in.Datum, Aktion: "zurück", Name: g.Entleiher, PaechterID: g.PaechterID, Notiz: in.Notiz})
			g.PaechterID, g.Entleiher, g.Seit, g.Bis = "", "", "", ""
			a.st.audit("Gerät %s zurück von %s", g.Name, name)
		default:
			g.LetzteWartung = in.Datum
			g.Verlauf = neuerVorgang(g.Verlauf, Vorgang{Datum: in.Datum, Aktion: "Wartung", Notiz: in.Notiz})
			a.st.audit("Gerät %s gewartet", g.Name)
		}
		a.speichernUndAntworten(w, g)
	}
}

// ------------------------------------------------------------------ Cockpit und Datenschutz

// inventarAufgabenLocked: Schlüssel bei ausgeschiedenen Pächtern, überfällige Geräte, fällige Wartungen.
func (s *Store) inventarAufgabenLocked(heute time.Time, add func(stufe, titel, text, ziel, knopf string)) {
	var zurueck []string
	for _, k := range s.d.Schluessel {
		if name, weg := s.inhaberName(k.PaechterID, k.Inhaber); (k.PaechterID != "" && weg) || k.Inhaber == geloeschtePerson {
			zurueck = append(zurueck, fmt.Sprintf("%s %s (%s)", k.Art, k.Nummer, name))
		}
	}
	if len(zurueck) > 0 {
		add("warn", mehrzahl(len(zurueck), "Schlüssel bei ausgeschiedenem Pächter", "Schlüssel bei ausgeschiedenen Pächtern"),
			"Bitte zurückfordern: "+strings.Join(zurueck, ", ")+".", "admin:inventar", "Schlüssel")
	}
	var ueberfaellig, wartung []string
	h := heute.Format("2006-01-02")
	for _, g := range s.d.Geraete {
		if (g.PaechterID != "" || g.Entleiher != "") && g.Bis != "" && g.Bis < h {
			name, _ := s.inhaberName(g.PaechterID, g.Entleiher)
			ueberfaellig = append(ueberfaellig, fmt.Sprintf("%s (%s, zurück bis %s)", g.Name, name, germanDate(g.Bis)))
		}
		if n := g.naechsteWartung(); n != "" && tageBis(heute, n) <= 30 {
			wartung = append(wartung, fmt.Sprintf("%s (%s)", g.Name, germanDate(n)))
		}
	}
	if len(ueberfaellig) > 0 {
		add("warn", mehrzahl(len(ueberfaellig), "Gerät nicht zurückgegeben", "Geräte nicht zurückgegeben"), "Überfällig: "+strings.Join(ueberfaellig, ", ")+".", "admin:inventar:geraete", "Geräte")
	}
	if len(wartung) > 0 {
		add("info", mehrzahl(len(wartung), "Gerät zur Wartung", "Geräte zur Wartung"), "Fällig: "+strings.Join(wartung, ", ")+".", "admin:inventar:geraete", "Geräte")
	}
}

// inventarPersonEntfernen löscht den Namen einer Person aus Schlüsseln und Geräten (der Schlüssel bleibt als
// »bei gelöschter Person« sichtbar, damit er nicht vergessen wird).
func (d *Data) inventarPersonEntfernen(id string) bool {
	changed := false
	verlauf := func(l []Vorgang) {
		for i := range l {
			if l[i].PaechterID == id {
				l[i].PaechterID, l[i].Name, changed = "", geloeschtePerson, true
			}
		}
	}
	for i := range d.Schluessel {
		k := &d.Schluessel[i]
		if k.PaechterID == id {
			k.PaechterID, k.Inhaber, changed = "", geloeschtePerson, true
		}
		verlauf(k.Verlauf)
	}
	for i := range d.Geraete {
		g := &d.Geraete[i]
		if g.PaechterID == id {
			g.PaechterID, g.Entleiher, changed = "", geloeschtePerson, true
		}
		verlauf(g.Verlauf)
	}
	return changed
}

type auskunftInventar struct {
	Was     string    `json:"was"`
	Aktuell bool      `json:"aktuellBeiDerPerson"`
	Verlauf []Vorgang `json:"verlauf"`
}

func (s *Store) inventarAuskunftLocked(id string) []auskunftInventar {
	var out []auskunftInventar
	eigene := func(l []Vorgang) []Vorgang {
		var r []Vorgang
		for _, v := range l {
			if v.PaechterID == id {
				r = append(r, v)
			}
		}
		return r
	}
	for _, k := range s.d.Schluessel {
		if v := eigene(k.Verlauf); len(v) > 0 || k.PaechterID == id {
			out = append(out, auskunftInventar{"Schlüssel " + k.Art + " " + k.Nummer, k.PaechterID == id, v})
		}
	}
	for _, g := range s.d.Geraete {
		if v := eigene(g.Verlauf); len(v) > 0 || g.PaechterID == id {
			out = append(out, auskunftInventar{"Gerät " + g.Name, g.PaechterID == id, v})
		}
	}
	return out
}
