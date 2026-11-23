package main

import (
	"net/http"
	"strings"
	"unicode"
)

// Gesamtsuche: Ein Suchbegriff durchsucht Pächter-fremde Bereiche (Mängel, Beschlüsse, Verträge, Termine, Dokumente,
// Warteliste, Ämter, Spenden, Rundschreiben, Ausgaben) und liefert Sprungziele. Nur für den Admin; die Pächtersuche
// bleibt im Kopfbereich schnell und lokal.

const (
	sucheMinZeichen = 2
	sucheMax        = 40
)

type sucheTreffer struct {
	Art    string `json:"art"`
	Titel  string `json:"titel"`
	Text   string `json:"text,omitempty"`
	Ziel   string `json:"ziel,omitempty"`   // wie bei den Cockpit-Zielen (admin:… oder tab:…)
	Garten string `json:"garten,omitempty"` // öffnet stattdessen die Gartenakte
}

// sucheNorm: Kleinbuchstaben, ä/ö/ü/ß ausgeschrieben, damit »Mueller« auch »Müller« findet.
func sucheNorm(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch r {
		case 'ä':
			b.WriteString("ae")
		case 'ö':
			b.WriteString("oe")
		case 'ü':
			b.WriteString("ue")
		case 'ß':
			b.WriteString("ss")
		default:
			if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// sucheLocked durchsucht die Bereiche. Alle Wörter der Anfrage müssen vorkommen. Der Aufrufer hält s.mu.
func (s *Store) sucheLocked(q string) []sucheTreffer {
	woerter := strings.Fields(sucheNorm(q))
	out := []sucheTreffer{}
	if len(woerter) == 0 {
		return out
	}
	passt := func(felder ...string) bool {
		heu := sucheNorm(strings.Join(felder, " "))
		for _, w := range woerter {
			if !strings.Contains(heu, w) && !tippfehlerTreffer(heu, w) {
				return false
			}
		}
		return true
	}
	add := func(t sucheTreffer) bool {
		out = append(out, t)
		return len(out) >= sucheMax
	}
	for _, m := range s.d.Maengel {
		if passt("Garten", m.Gartennr, m.Art, m.Beschreibung) && add(sucheTreffer{Art: "Mangel", Titel: "Garten " + m.Gartennr + ": " + m.Art, Text: m.Beschreibung, Ziel: "admin:maengel"}) {
			return out
		}
	}
	for _, b := range s.d.Beschluesse {
		if passt(b.Nummer, b.Titel, b.Text, b.Stichworte) && add(sucheTreffer{Art: "Beschluss", Titel: b.Nummer + " " + b.Titel, Text: germanDate(b.Datum), Ziel: "admin:verwaltung:beschluesse"}) {
			return out
		}
	}
	for _, v := range s.d.Vertraege {
		if passt(v.Bezeichnung, v.Art, v.Partner, v.Nummer) && add(sucheTreffer{Art: "Vertrag", Titel: v.Bezeichnung, Text: v.Partner, Ziel: "admin:verwaltung:vertraege"}) {
			return out
		}
	}
	for _, a := range s.d.Aemter {
		if passt(a.Amt, a.Name, a.Notiz) && add(sucheTreffer{Art: "Amt", Titel: a.Amt + ": " + a.Name, Ziel: "admin:verwaltung:aemter"}) {
			return out
		}
	}
	for _, t := range s.d.Termine {
		if passt(t.Titel, t.Ort, t.Notiz, t.Art) && add(sucheTreffer{Art: "Termin", Titel: t.Titel, Text: germanDate(t.Datum), Ziel: "tab:termine"}) {
			return out
		}
	}
	for _, d := range s.d.Dokumente {
		if passt(d.Titel, d.Art, "Garten", d.Gartennr) && add(sucheTreffer{Art: "Dokument", Titel: d.Titel, Text: "Garten " + d.Gartennr + " · " + d.Art, Garten: d.Gartennr}) {
			return out
		}
	}
	for _, w := range s.d.Warteliste {
		if passt(w.Name, w.Strasse, w.PLZOrt, w.Email) && add(sucheTreffer{Art: "Warteliste", Titel: w.Name, Text: "seit " + germanDate(w.Seit), Ziel: "admin:warteliste"}) {
			return out
		}
	}
	for _, sp := range s.d.Spenden {
		if passt(sp.Name, sp.Strasse, sp.PLZOrt) && add(sucheTreffer{Art: "Spende", Titel: sp.Name, Text: germanDate(sp.Datum) + " · " + fmtEUR(sp.Betrag), Ziel: "admin:spenden"}) {
			return out
		}
	}
	for _, r := range s.d.Rundschreiben {
		if passt(r.Betreff, r.Text) && add(sucheTreffer{Art: "Rundschreiben", Titel: r.Betreff, Text: germanDate(r.Datum), Ziel: "admin:rundschreiben"}) {
			return out
		}
	}
	for _, g := range s.d.Geraete {
		if passt(g.Name, g.Inventarnr, g.Standort) && add(sucheTreffer{Art: "Gerät", Titel: g.Name, Text: g.Standort, Ziel: "admin:inventar:geraete"}) {
			return out
		}
	}
	for _, j := range s.d.Jahre {
		for _, a := range j.Ausgaben {
			if passt(a.Beschreibung, a.Kategorie) && add(sucheTreffer{Art: "Ausgabe", Titel: a.Beschreibung, Text: germanDate(a.Datum) + " · " + fmtEUR(a.Betrag), Ziel: "admin:kassenbericht"}) {
				return out
			}
		}
	}
	return out
}

func (a *App) handleSuche(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q)) < sucheMinZeichen || len(q) > 100 {
		writeJSON(w, 200, []sucheTreffer{})
		return
	}
	a.st.mu.Lock()
	res := a.st.sucheLocked(q)
	a.st.mu.Unlock()
	writeJSON(w, 200, res)
}

// tippfehlerTreffer: ein Wort ab 5 Buchstaben findet auch Wörter, die sich in einem Buchstaben unterscheiden
// (»Meyer« findet »Meier«, »Hekce« findet »Hecke«).
func tippfehlerTreffer(heu, w string) bool {
	if len([]rune(w)) < 5 {
		return false
	}
	for _, t := range strings.Fields(heu) {
		if einFehler(t, w) {
			return true
		}
	}
	return false
}

// einFehler: höchstens ein Buchstabe ersetzt, vergessen, zu viel oder zwei Buchstaben vertauscht.
func einFehler(a, b string) bool {
	x, y := []rune(a), []rune(b)
	if len(x) < len(y) {
		x, y = y, x
	}
	if len(x)-len(y) > 1 {
		return false
	}
	i := 0
	for i < len(y) && x[i] == y[i] {
		i++
	}
	if i == len(y) {
		return len(x)-len(y) <= 1
	}
	if len(x) == len(y) {
		if string(x[i+1:]) == string(y[i+1:]) {
			return true // ein Buchstabe ersetzt
		}
		return i+1 < len(x) && x[i] == y[i+1] && x[i+1] == y[i] && string(x[i+2:]) == string(y[i+2:]) // vertauscht
	}
	return string(x[i+1:]) == string(y[i:])
}
