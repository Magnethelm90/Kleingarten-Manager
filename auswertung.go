package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
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
	Betrag       float64 `json:"betrag"`
}

func cleanAusgabe(in Ausgabe) (Ausgabe, error) {
	in.Beschreibung = trim(in.Beschreibung, 120)
	if in.Beschreibung == "" {
		return in, bad("Bitte eine Beschreibung eintragen")
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

// ---------------------------------------------------------------- Kassenbericht

// Versorger sind die Werte der Hauptzähler bzw. der Rechnungen des Versorgers
// für ein Jahr. Sie dienen nur dem Vergleich im Kassenbericht.
type Versorger struct {
	WasserM3  *float64 `json:"wasserM3"`
	WasserEUR *float64 `json:"wasserEUR"`
	StromKWh  *float64 `json:"stromKWh"`
	StromEUR  *float64 `json:"stromEUR"`
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
	Ausgaben      []Ausgabe `json:"ausgaben"`
	AusgabenSumme float64   `json:"ausgabenSumme"`

	// Saldo = EinnahmenBezahlt - GuthabenAusgezahlt - AusgabenSumme. Kein vollständiger
	// Kontostand (ein Anfangsbestand fließt nicht ein), sondern eine Kontrollrechnung:
	// was ist dieses Jahr für die Vereinskasse tatsächlich geflossen.
	Saldo float64 `json:"saldo"`
}

// kassenberichtLocked summiert alle Posten eines Jahres. Grundlage der Beträge je
// Pächter ist die gültige ausgestellte Rechnung, sonst die aktuelle Berechnung.
// Der Aufrufer hält s.mu.
func (s *Store) kassenberichtLocked(v yearView) kassenbericht {
	k := kassenbericht{Jahr: v.Jahr, Unvollstaendig: []string{}}
	if j := s.d.Jahre[yearKey(v.Jahr)]; j != nil {
		if j.Versorger != nil {
			k.Versorger = *j.Versorger
		}
		k.Ausgaben = append([]Ausgabe(nil), j.Ausgaben...)
	}
	sort.SliceStable(k.Ausgaben, func(i, j int) bool { return k.Ausgaben[i].Datum < k.Ausgaben[j].Datum })
	for _, x := range k.Ausgaben {
		k.AusgabenSumme += x.Betrag
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
	for _, x := range []*float64{&k.WasserVerbrauch, &k.StromVerbrauch, &k.Summen.KostenWasser, &k.Summen.KostenEnergie,
		&k.Summen.NachzahlungStunden, &k.Summen.Zwischensumme1, &k.Summen.PachtGarten, &k.Summen.PachtVerein, &k.Summen.PachtFrei,
		&k.Summen.Mitgliedsbeitrag, &k.Summen.Umlage, &k.Summen.Zwischensumme2, &k.Summen.Verguetung, &k.Summen.Gesamt,
		&k.Versicherung, &k.Grundsteuer, &k.Auslagen, &k.Abschlag, &k.EinnahmenBezahlt, &k.GuthabenAusgezahlt, &k.AusgabenSumme, &k.Saldo} {
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
	data, err := writeXLSX("Kassenbericht "+strconv.Itoa(year), []float64{44, 16, 16, 16}, kassenberichtRows(k, vereinsname))
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
		[]xCell{{"Sonstige Ausgaben der Vereinskasse", stHeader}, {"Datum", stHeader}, {"Betrag (€)", stHeader}},
	)
	for _, x := range k.Ausgaben {
		rows = append(rows, []xCell{{x.Beschreibung, stNormal}, {germanDate(x.Datum), stNormal}, {x.Betrag, stEUR}})
	}
	rows = append(rows,
		[]xCell{{"Summe sonstige Ausgaben", stBold}, {}, {k.AusgabenSumme, stEURb}},
		[]xCell{},
		[]xCell{{"Tatsächlich geflossenes Geld (nach heutigem Zahlungsstand)", stHeader}, {"Betrag (€)", stHeader}},
		[]xCell{{"Von Pächtern eingegangene Zahlungen", stNormal}, {}, {k.EinnahmenBezahlt, stEUR}},
		[]xCell{{"An Pächter ausgezahlte Guthaben", stNormal}, {}, {-k.GuthabenAusgezahlt, stEUR}},
		[]xCell{{"Sonstige Ausgaben", stNormal}, {}, {-k.AusgabenSumme, stEUR}},
		[]xCell{{"Saldo (ohne Anfangsbestand der Kasse)", stBold}, {}, {k.Saldo, stEURb}},
	)
	if len(k.Unvollstaendig) > 0 {
		rows = append(rows, []xCell{}, []xCell{{"Nicht in der Abrechnung enthalten (unvollständig)", stHeader}})
		for _, n := range k.Unvollstaendig {
			rows = append(rows, []xCell{{n, stNormal}})
		}
	}
	return rows
}
