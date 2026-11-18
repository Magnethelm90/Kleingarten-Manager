package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// Daten-Aufräumen: sucht Dubletten und Lücken im Bestand (doppelte Pächter, doppelt vergebene Gartennummern,
// fehlende Anschrift oder E-Mail, ungültige IBAN in Lastschrift-Mandaten, Gärten ohne Pächter) und nennt zu jedem Fund
// den Pächter, damit man ihn mit einem Klick öffnen und verbessern kann. Das Programm verändert hier nichts selbst.

type datencheckFund struct {
	Art        string   `json:"art"`   // doppelt | luecke | iban | garten
	Stufe      string   `json:"stufe"` // warn | info
	Text       string   `json:"text"`
	Tipp       string   `json:"tipp"`
	PaechterID []string `json:"paechterIds"`
}

type datencheckErgebnis struct {
	Funde    []datencheckFund `json:"funde"`
	Gruppe   map[string]int   `json:"gruppen"`
	Geprueft int              `json:"geprueft"` // Anzahl geprüfter Pächter
}

func normName(s string) string {
	s = strings.ToLower(strings.Join(strings.Fields(s), " "))
	return strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss", ",", "", ".", "").Replace(s)
}

func (s *Store) datencheckLocked() datencheckErgebnis {
	pa := make([]Paechter, 0)
	for _, p := range s.d.Paechter {
		if !p.Geloescht && p.AusgeschiedenAm == "" {
			pa = append(pa, p)
		}
	}
	sort.SliceStable(pa, func(i, j int) bool { return naturalLess(pa[i].Gartennr, pa[j].Gartennr) })
	var out []datencheckFund
	add := func(art, stufe, text, tipp string, ids ...string) {
		if ids == nil {
			ids = []string{}
		}
		out = append(out, datencheckFund{Art: art, Stufe: stufe, Text: text, Tipp: tipp, PaechterID: ids})
	}
	bez := func(p Paechter) string {
		if p.Gartennr != "" {
			return fmt.Sprintf("%s (Garten %s)", p.Name, p.Gartennr)
		}
		return p.Name
	}

	// Gruppen gleicher Werte bilden
	gruppen := func(schluessel func(Paechter) string) [][]Paechter {
		m := map[string][]Paechter{}
		var reihe []string
		for _, p := range pa {
			k := schluessel(p)
			if k == "" {
				continue
			}
			if _, ok := m[k]; !ok {
				reihe = append(reihe, k)
			}
			m[k] = append(m[k], p)
		}
		var res [][]Paechter
		for _, k := range reihe {
			if len(m[k]) > 1 {
				res = append(res, m[k])
			}
		}
		return res
	}
	ids := func(g []Paechter) []string {
		r := make([]string, len(g))
		for i, p := range g {
			r[i] = p.ID
		}
		return r
	}
	namen := func(g []Paechter) string {
		r := make([]string, len(g))
		for i, p := range g {
			r[i] = bez(p)
		}
		return strings.Join(r, " und ")
	}
	for _, g := range gruppen(func(p Paechter) string { return normName(p.Name) }) {
		add("doppelt", "warn", "Der Name kommt mehrfach vor: "+namen(g)+".", "Ist es dieselbe Person mit zwei Gärten, ist alles in Ordnung. Sonst einen Eintrag löschen.", ids(g)...)
	}
	for _, g := range gruppen(func(p Paechter) string { return strings.TrimSpace(p.Mitgliedsnr) }) {
		add("doppelt", "warn", "Die Mitgliedsnummer "+g[0].Mitgliedsnr+" ist mehrfach vergeben: "+namen(g)+".", "Jede Nummer darf nur einmal vorkommen. Bei einem Eintrag die Nummer ändern.", ids(g)...)
	}
	for _, g := range gruppen(func(p Paechter) string { return strings.TrimSpace(p.Gartennr) }) {
		add("doppelt", "warn", "Garten "+g[0].Gartennr+" hat mehrere Pächter: "+namen(g)+".", "Ein Garten hat nur einen Pächter. Bei einem Pächterwechsel im Mehr-Menü »Pächterwechsel« benutzen.", ids(g)...)
	}
	for _, g := range gruppen(func(p Paechter) string { return strings.ToLower(strings.TrimSpace(p.Email)) }) {
		add("doppelt", "info", "Die E-Mail-Adresse "+g[0].Email+" gehört zu mehreren Pächtern: "+namen(g)+".", "Das ist bei Ehepaaren normal. Sonst die richtige Adresse eintragen.", ids(g)...)
	}
	for _, g := range gruppen(func(p Paechter) string { return strings.TrimSpace(p.WasserzaehlerNr) }) {
		add("doppelt", "warn", "Die Wasserzähler-Nummer "+g[0].WasserzaehlerNr+" steht bei mehreren Gärten: "+namen(g)+".", "Jeder Zähler gehört zu einem Garten. Nummer am Zähler ablesen und verbessern.", ids(g)...)
	}
	for _, g := range gruppen(func(p Paechter) string { return strings.TrimSpace(p.StromzaehlerNr) }) {
		add("doppelt", "warn", "Die Stromzähler-Nummer "+g[0].StromzaehlerNr+" steht bei mehreren Gärten: "+namen(g)+".", "Jeder Zähler gehört zu einem Garten. Nummer am Zähler ablesen und verbessern.", ids(g)...)
	}

	// Lücken
	for _, p := range pa {
		var fehlt []string
		if strings.TrimSpace(p.Gartennr) == "" {
			fehlt = append(fehlt, "die Gartennummer")
		}
		if p.Gartengroesse <= 0 {
			fehlt = append(fehlt, "die Gartengröße")
		}
		if strings.EqualFold(strings.TrimSpace(p.Versand), "Postversand") && (strings.TrimSpace(p.Strasse) == "" || strings.TrimSpace(p.PLZOrt) == "") {
			fehlt = append(fehlt, "die Anschrift (Post kann nicht verschickt werden)")
		}
		if !strings.EqualFold(strings.TrimSpace(p.Versand), "Postversand") && strings.TrimSpace(p.Versand) != "" && strings.TrimSpace(p.Email) == "" {
			fehlt = append(fehlt, "die E-Mail-Adresse (Versand per E-Mail ist gewählt)")
		}
		if len(fehlt) > 0 {
			add("luecke", "warn", bez(p)+": Es fehlt "+strings.Join(fehlt, ", ")+".", "Pächter öffnen und ergänzen.", p.ID)
		}
		if e := strings.TrimSpace(p.Email); e != "" && (!strings.Contains(e, "@") || strings.ContainsAny(e, " ,;") || strings.HasSuffix(e, ".")) {
			add("luecke", "warn", bez(p)+": Die E-Mail-Adresse »"+e+"« sieht nicht richtig aus.", "Tippfehler suchen (@-Zeichen, Leerzeichen).", p.ID)
		}
	}

	// IBAN in Mandaten
	byID := map[string]Paechter{}
	for _, p := range s.d.Paechter {
		byID[p.ID] = p
	}
	for _, m := range s.d.Mandate {
		if m.Widerrufen != "" {
			continue
		}
		p, ok := byID[m.PaechterID]
		if !ok || p.Geloescht {
			continue
		}
		if !ibanGueltig(m.IBAN) {
			add("iban", "warn", bez(p)+": Die IBAN im Lastschrift-Mandat ist ungültig.", "IBAN mit der Unterschrift auf dem Mandat vergleichen und verbessern (Lastschrift → Mandat).", p.ID)
		}
	}
	if len(pa) > 0 && !s.d.Demo && s.d.Settings.IBAN != "" && !ibanGueltig(s.d.Settings.IBAN) {
		add("iban", "warn", "Die Vereins-IBAN in den Einstellungen ist ungültig. Rechnungen und QR-Codes würden falsch.", "Unter »Preise & Einstellungen« verbessern.")
	}

	// Gärten ohne Pächter (nur wenn ein Lageplan gepflegt ist)
	belegt := map[string]bool{}
	for _, p := range pa {
		belegt[strings.TrimSpace(p.Gartennr)] = true
	}
	var leer []string
	for nr := range s.d.Lageplan {
		if !belegt[nr] {
			leer = append(leer, nr)
		}
	}
	sort.Slice(leer, func(i, j int) bool { return naturalLess(leer[i], leer[j]) })
	if len(leer) > 0 {
		add("garten", "info", "Auf dem Lageplan ohne Pächter: Garten "+strings.Join(leer, ", ")+".", "Frei oder vergessen einzutragen? Freie Gärten kannst du über die Warteliste vergeben.")
	}

	res := datencheckErgebnis{Funde: out, Gruppe: map[string]int{}, Geprueft: len(pa)}
	if res.Funde == nil {
		res.Funde = []datencheckFund{}
	}
	for _, f := range out {
		res.Gruppe[f.Art]++
	}
	return res
}

func (a *App) handleDatencheck(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	res := a.st.datencheckLocked()
	a.st.mu.Unlock()
	writeJSON(w, 200, res)
}
