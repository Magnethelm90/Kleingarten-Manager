package main

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Spenden und Zuwendungsbestätigungen: Geldspenden an den Verein erfassen und dafür die Bestätigung nach dem
// amtlichen Muster des Bundesfinanzministeriums ausstellen (einzeln oder als Sammelbestätigung für ein Jahr).
// Mitgliedsbeiträge an Kleingartenvereine sind nicht abziehbar (§ 10b Abs. 1 Satz 8 EStG), deshalb gibt es nur
// Geldzuwendungen. Sachspenden werden nicht unterstützt. Die Bestätigung wird immer von Hand unterschrieben.

// Gemeinnuetzigkeit enthält die Angaben aus dem Bescheid des Finanzamts.
type Gemeinnuetzigkeit struct {
	Anschrift     string `json:"anschrift"`     // Anschrift des Vereins, mehrzeilig
	Zweck         string `json:"zweck"`         // begünstigter Zweck, z. B. »der Kleingärtnerei«
	Finanzamt     string `json:"finanzamt"`     // z. B. »Musterstadt«
	Steuernummer  string `json:"steuernummer"`  // StNr.
	Bescheid      string `json:"bescheid"`      // "freistellung" oder "feststellung" (§ 60a AO)
	BescheidDatum string `json:"bescheidDatum"` // JJJJ-MM-TT
	Veranlagung   string `json:"veranlagung"`   // letzter Veranlagungszeitraum (nur Freistellungsbescheid), z. B. »2021–2023«
}

// Spende ist eine Geldzuwendung an den Verein.
type Spende struct {
	ID           string  `json:"id"`
	Datum        string  `json:"datum"` // Tag der Zuwendung
	Jahr         int     `json:"jahr"`  // Abrechnungsjahr, in dessen Kassenbericht die Spende zählt
	Betrag       float64 `json:"betrag"`
	PaechterID   string  `json:"paechterId,omitempty"`
	Name         string  `json:"name"`
	Strasse      string  `json:"strasse"`
	PLZOrt       string  `json:"plzOrt"`
	Verzicht     bool    `json:"verzicht,omitempty"` // Verzicht auf Erstattung von Aufwendungen
	Notiz        string  `json:"notiz,omitempty"`
	BestaetigtAm string  `json:"bestaetigtAm,omitempty"` // wann die Zuwendungsbestätigung ausgestellt wurde
	Bereinigt    bool    `json:"bereinigt,omitempty"`    // Aufbewahrungsfrist abgelaufen, Personenbezug entfernt
}

const (
	maxSpenden          = 5000
	freistellungGueltig = 5 // Jahre (§ 63 Abs. 5 AO)
	feststellungGueltig = 3
)

func cleanGemeinnuetzigkeit(g Gemeinnuetzigkeit) (Gemeinnuetzigkeit, error) {
	einzeilig := func(s string, n int) string { return trim(strings.Join(strings.Fields(s), " "), n) }
	var zeilen []string
	for _, z := range strings.Split(g.Anschrift, "\n") {
		if z = einzeilig(z, 80); z != "" && len(zeilen) < 4 {
			zeilen = append(zeilen, z)
		}
	}
	g.Anschrift = strings.Join(zeilen, "\n")
	g.Zweck = einzeilig(g.Zweck, 200)
	g.Finanzamt = einzeilig(g.Finanzamt, 80)
	g.Steuernummer = einzeilig(g.Steuernummer, 30)
	g.Veranlagung = einzeilig(g.Veranlagung, 30)
	g.BescheidDatum = strings.TrimSpace(g.BescheidDatum)
	if g.Bescheid != "feststellung" {
		g.Bescheid = "freistellung"
	}
	if g.BescheidDatum != "" {
		if d, err := parseDate(g.BescheidDatum); err != nil || d.Year() < 1990 || d.Year() > 2100 {
			return g, bad("Bitte das Datum des Bescheids als gültiges Datum eintragen")
		}
	}
	return g, nil
}

// pruefeGemeinnuetzigkeit stellt sicher, dass alle Angaben vorhanden sind und der Bescheid noch verwendet werden
// darf: Freistellungsbescheid höchstens 5 Jahre, Feststellung nach § 60a AO höchstens 3 Jahre alt (§ 63 Abs. 5 AO).
func pruefeGemeinnuetzigkeit(g Gemeinnuetzigkeit, heute time.Time) error {
	if g.Anschrift == "" || g.Zweck == "" || g.Finanzamt == "" || g.Steuernummer == "" || g.BescheidDatum == "" ||
		(g.Bescheid == "freistellung" && g.Veranlagung == "") {
		return bad("Bitte zuerst die Angaben zur Gemeinnützigkeit vollständig eintragen (Anschrift, Zweck, Finanzamt, Steuernummer, Bescheid)")
	}
	d, err := parseDate(g.BescheidDatum)
	if err != nil {
		return bad("Das Datum des Bescheids ist ungültig")
	}
	jahre := freistellungGueltig
	if g.Bescheid == "feststellung" {
		jahre = feststellungGueltig
	}
	if d.AddDate(jahre, 0, 0).Before(heute) {
		return bad(fmt.Sprintf("Der Bescheid vom %s ist älter als %d Jahre. Zuwendungsbestätigungen dürfen damit nicht mehr ausgestellt werden (§ 63 Abs. 5 AO) – bitte den neuen Bescheid eintragen", germanDate(g.BescheidDatum), jahre))
	}
	return nil
}

func cleanSpende(x Spende) (Spende, error) {
	einzeilig := func(s string, n int) string { return trim(strings.Join(strings.Fields(s), " "), n) }
	x.Name, x.Strasse, x.PLZOrt = einzeilig(x.Name, 100), einzeilig(x.Strasse, 100), einzeilig(x.PLZOrt, 80)
	x.Notiz = trim(x.Notiz, 200)
	if d, err := parseDate(x.Datum); err != nil || d.Year() < 2000 || d.Year() > 2100 {
		return x, bad("Bitte den Tag der Zuwendung eintragen")
	}
	if !validNum(x.Betrag) || x.Betrag <= 0 || x.Betrag > 1e7 {
		return x, bad("Bitte einen Betrag größer als 0 eintragen")
	}
	x.Betrag = round2(x.Betrag)
	if x.Name == "" {
		return x, bad("Bitte den Namen des Spenders eintragen")
	}
	return x, nil
}

// ---------------------------------------------------------------- Betrag in Worten

var zahlEiner = []string{"", "ein", "zwei", "drei", "vier", "fünf", "sechs", "sieben", "acht", "neun", "zehn", "elf", "zwölf",
	"dreizehn", "vierzehn", "fünfzehn", "sechzehn", "siebzehn", "achtzehn", "neunzehn"}
var zahlZehner = []string{"", "", "zwanzig", "dreißig", "vierzig", "fünfzig", "sechzig", "siebzig", "achtzig", "neunzig"}

// zahlWort schreibt eine ganze Zahl von 0 bis 999 999 999 aus (»dreihunderteinundzwanzig«).
func zahlWort(n int) string {
	if n == 0 {
		return "null"
	}
	unter1000 := func(n int) string {
		s := ""
		if h := n / 100; h > 0 {
			s += zahlEiner[h] + "hundert"
		}
		r := n % 100
		switch {
		case r == 0:
		case r < 20:
			s += zahlEiner[r]
		case r%10 == 0:
			s += zahlZehner[r/10]
		default:
			s += zahlEiner[r%10] + "und" + zahlZehner[r/10]
		}
		return s
	}
	s := ""
	if m := n / 1000000; m > 0 {
		if m == 1 {
			s += "einemillion"
		} else {
			s += unter1000(m) + "millionen"
		}
		n %= 1000000
	}
	if t := n / 1000; t > 0 {
		s += unter1000(t) + "tausend"
		n %= 1000
	}
	s += unter1000(n)
	if strings.HasSuffix(s, "ein") {
		s += "s" // »einundzwanzig«, aber »hunderteins«, »eins«
	}
	return s
}

// betragInWorten: 1234.5 → »eintausendzweihundertvierunddreißig Euro fünfzig Cent«
func betragInWorten(b float64) string {
	cent := int(math.Round(b * 100))
	euro, rest := cent/100, cent%100
	s := zahlWort(euro) + " Euro"
	if euro == 1 {
		s = "ein Euro"
	}
	switch {
	case rest == 1:
		s += " ein Cent"
	case rest > 0:
		s += " " + zahlWort(rest) + " Cent"
	}
	return s
}

// ---------------------------------------------------------------- Handler

type spendenAnsicht struct {
	Gemeinnuetzigkeit Gemeinnuetzigkeit `json:"gemeinnuetzigkeit"`
	Spenden           []Spende          `json:"spenden"`
	Hinweis           string            `json:"hinweis,omitempty"` // warum gerade keine Bestätigung möglich ist
}

func (a *App) handleSpendenListe(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	out := spendenAnsicht{Gemeinnuetzigkeit: a.st.d.Gemeinnuetzigkeit, Spenden: append([]Spende{}, a.st.d.Spenden...)}
	if err := pruefeGemeinnuetzigkeit(a.st.d.Gemeinnuetzigkeit, time.Now()); err != nil {
		out.Hinweis = err.Error()
	}
	sort.SliceStable(out.Spenden, func(i, j int) bool { return out.Spenden[i].Datum > out.Spenden[j].Datum })
	writeJSON(w, 200, out)
}

func (a *App) handleGemeinnuetzigkeit(w http.ResponseWriter, r *http.Request) {
	var in Gemeinnuetzigkeit
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in, err := cleanGemeinnuetzigkeit(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	a.st.d.Gemeinnuetzigkeit = in
	a.st.audit("Angaben zur Gemeinnützigkeit geändert")
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, in)
}

func (a *App) handleSpendeSpeichern(neu bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in Spende
		if err := readJSON(w, r, &in); err != nil {
			writeErr(w, err)
			return
		}
		in, err := cleanSpende(in)
		if err != nil {
			writeErr(w, err)
			return
		}
		a.st.mu.Lock()
		defer a.st.mu.Unlock()
		if in.PaechterID != "" {
			gefunden := false
			for _, p := range a.st.d.Paechter {
				gefunden = gefunden || p.ID == in.PaechterID
			}
			if !gefunden {
				in.PaechterID = ""
			}
		}
		if j := a.st.d.Jahre[yearKey(in.Jahr)]; j == nil || j.Abgeschlossen {
			in.Jahr = a.st.d.Settings.Jahr // Kasse: ins laufende Abrechnungsjahr
		}
		if neu {
			if len(a.st.d.Spenden) >= maxSpenden {
				writeErr(w, bad("Es sind sehr viele Spenden gespeichert"))
				return
			}
			in.ID, in.BestaetigtAm, in.Bereinigt = newID(), "", false
			a.st.d.Spenden = append(a.st.d.Spenden, in)
			a.st.audit("Spende erfasst: %s am %s", fmtEUR(in.Betrag), germanDate(in.Datum))
		} else {
			i := a.st.spendeIndexLocked(r.PathValue("id"))
			if i < 0 {
				writeErr(w, notFound("Spende nicht gefunden"))
				return
			}
			alt := a.st.d.Spenden[i]
			if alt.Bereinigt {
				writeErr(w, bad("Diese Spende ist wegen abgelaufener Aufbewahrungsfrist bereinigt"))
				return
			}
			if alt.BestaetigtAm != "" && (alt.Betrag != in.Betrag || alt.Datum != in.Datum || alt.Name != in.Name || alt.Verzicht != in.Verzicht) {
				writeErr(w, bad("Für diese Spende wurde schon eine Bestätigung ausgestellt. Betrag, Datum und Spender lassen sich nicht mehr ändern"))
				return
			}
			in.ID, in.BestaetigtAm = alt.ID, alt.BestaetigtAm
			a.st.d.Spenden[i] = in
			a.st.audit("Spende geändert: %s am %s", fmtEUR(in.Betrag), germanDate(in.Datum))
		}
		if err := a.st.saveLocked(); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 200, in)
	}
}

func (s *Store) spendeIndexLocked(id string) int {
	for i, x := range s.d.Spenden {
		if x.ID == id {
			return i
		}
	}
	return -1
}

func (a *App) handleSpendeLoeschen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	i := a.st.spendeIndexLocked(r.PathValue("id"))
	if i < 0 {
		writeErr(w, notFound("Spende nicht gefunden"))
		return
	}
	x := a.st.d.Spenden[i]
	if x.BestaetigtAm != "" && !x.Bereinigt {
		writeErr(w, bad("Für diese Spende wurde eine Bestätigung ausgestellt; das Doppel muss aufbewahrt werden. Sie lässt sich deshalb nicht löschen"))
		return
	}
	a.st.d.Spenden = append(a.st.d.Spenden[:i], a.st.d.Spenden[i+1:]...)
	a.st.audit("Spende gelöscht: %s am %s", fmtEUR(x.Betrag), germanDate(x.Datum))
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// spendenFuerBestaetigungLocked sucht die Spenden zu den IDs und prüft, dass eine Bestätigung möglich ist:
// vollständige Anschrift, bei mehreren derselbe Spender und dasselbe Kalenderjahr (Sammelbestätigung).
func (s *Store) spendenFuerBestaetigungLocked(ids []string, heute time.Time) ([]Spende, error) {
	if len(ids) == 0 || len(ids) > 500 {
		return nil, bad("Bitte die Spenden auswählen, die bestätigt werden sollen")
	}
	if err := pruefeGemeinnuetzigkeit(s.d.Gemeinnuetzigkeit, heute); err != nil {
		return nil, err
	}
	var l []Spende
	doppelt := map[string]bool{}
	for _, id := range ids {
		i := s.spendeIndexLocked(id)
		if i < 0 || doppelt[id] {
			return nil, notFound("Spende nicht gefunden")
		}
		doppelt[id] = true
		x := s.d.Spenden[i]
		if x.Bereinigt {
			return nil, bad("Diese Spende ist wegen abgelaufener Aufbewahrungsfrist bereinigt")
		}
		if x.Strasse == "" || x.PLZOrt == "" {
			return nil, bad(fmt.Sprintf("Für die Bestätigung fehlt die Anschrift von %s", x.Name))
		}
		l = append(l, x)
	}
	key := func(x Spende) string { return strings.ToLower(x.Name + "|" + x.Strasse + "|" + x.PLZOrt) }
	for _, x := range l[1:] {
		if key(x) != key(l[0]) {
			return nil, bad("Eine Sammelbestätigung gilt nur für einen Spender – bitte nur Spenden derselben Person auswählen")
		}
		if jahrVon(x.Datum) != jahrVon(l[0].Datum) {
			return nil, bad("Eine Sammelbestätigung gilt für ein Kalenderjahr – bitte nur Spenden aus einem Jahr auswählen")
		}
	}
	sort.SliceStable(l, func(i, j int) bool { return l[i].Datum < l[j].Datum })
	return l, nil
}

// jahrVon liefert das Jahr eines Datums JJJJ-MM-TT (leer bei unlesbarem Datum, z. B. aus einer beschädigten Datei).
func jahrVon(datum string) string {
	if len(datum) < 4 {
		return ""
	}
	return datum[:4]
}

func idsParam(s string) []string {
	var out []string
	for _, id := range strings.Split(s, ",") {
		if id = strings.TrimSpace(id); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// handleSpendenBestaetigen vermerkt, dass für die Spenden eine Bestätigung ausgestellt wird (das Programm warnt
// dann vor einer zweiten). Das PDF selbst liefert zuwendungsbestaetigungPDF.
func (a *App) handleSpendenBestaetigen(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	l, err := a.st.spendenFuerBestaetigungLocked(in.IDs, time.Now())
	if err != nil {
		writeErr(w, err)
		return
	}
	heute := time.Now().Format("2006-01-02")
	for _, x := range l {
		a.st.d.Spenden[a.st.spendeIndexLocked(x.ID)].BestaetigtAm = heute
	}
	a.st.audit("Zuwendungsbestätigung ausgestellt: %d Spende(n)", len(l))
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]int{"bestaetigt": len(l)})
}

func (a *App) zuwendungsbestaetigungPDF(r *http.Request) ([]byte, string, error) {
	a.st.mu.Lock()
	l, err := a.st.spendenFuerBestaetigungLocked(idsParam(r.URL.Query().Get("ids")), time.Now())
	s, g := a.st.d.Settings, a.st.d.Gemeinnuetzigkeit
	a.st.mu.Unlock()
	if err != nil {
		return nil, "", err
	}
	pdf, err := buildZuwendungsbestaetigung(s, g, l, time.Now())
	return pdf, safeName("Zuwendungsbestaetigung_"+l[0].Name+"_"+jahrVon(l[0].Datum), 80) + ".pdf", err
}

// ---------------------------------------------------------------- Kasse und Datenschutz

// spendenSummeLocked: Spenden, die im Kassenbericht des Abrechnungsjahres als Einnahme zählen.
func (s *Store) spendenSummeLocked(jahr int) float64 {
	sum := 0.0
	for _, x := range s.d.Spenden {
		if x.Jahr == jahr {
			sum += x.Betrag
		}
	}
	return round2(sum)
}

// spenderEntfernen: Beim endgültigen Löschen einer Person bleiben bestätigte Spenden erhalten (das Doppel der
// Bestätigung ist aufzubewahren), nur die Verknüpfung entfällt; unbestätigte verlieren Name und Anschrift,
// der Betrag bleibt für die Kasse.
func (d *Data) spenderEntfernen(id string) bool {
	changed := false
	for i := range d.Spenden {
		x := &d.Spenden[i]
		if x.PaechterID != id {
			continue
		}
		x.PaechterID, changed = "", true
		if x.BestaetigtAm == "" {
			x.Name, x.Strasse, x.PLZOrt, x.Notiz = geloeschtText, "", "", ""
		}
	}
	return changed
}

// spendenBereinigen entfernt nach Ablauf der Aufbewahrungsfrist den Personenbezug (Jahr der Zuwendung zählt).
func (d *Data) spendenBereinigen(aktuell int) int {
	n := 0
	for i := range d.Spenden {
		x := &d.Spenden[i]
		y, _ := strconv.Atoi(jahrVon(x.Datum))
		if x.Bereinigt || !fristAbgelaufen(y, aktuell) {
			continue
		}
		x.Name, x.Strasse, x.PLZOrt, x.Notiz, x.PaechterID, x.Bereinigt = fristAbgelaufenText, "", "", "", "", true
		n++
	}
	return n
}
