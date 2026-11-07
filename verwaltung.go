package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Beschlussbuch und Verträge des Vereins. Das Beschlussbuch hält fest, was Vorstand und Mitgliederversammlung
// wann beschlossen haben (durchsuchbar, als PDF). Die Vertragsliste kennt Laufzeiten und Kündigungsfristen von
// Versicherungen, Versorgern und anderen Verträgen; das Cockpit erinnert rechtzeitig vor dem Kündigungstermin.
// Beides enthält keine Personendaten der Pächter.

type Beschluss struct {
	ID         string `json:"id"`
	Datum      string `json:"datum"`
	Gremium    string `json:"gremium"` // Mitgliederversammlung | Vorstand
	Nummer     string `json:"nummer"`
	Titel      string `json:"titel"`
	Text       string `json:"text,omitempty"`
	Ja         int    `json:"ja,omitempty"`
	Nein       int    `json:"nein,omitempty"`
	Enthaltung int    `json:"enthaltung,omitempty"`
	Ergebnis   string `json:"ergebnis"` // angenommen | abgelehnt | zur Kenntnis
	Stichworte string `json:"stichworte,omitempty"`
}

const maxVerwaltung = 2000

func (a *App) handleBeschluesse(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	out := append([]Beschluss{}, a.st.d.Beschluesse...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Datum > out[j].Datum })
	writeJSON(w, 200, out)
}

func (a *App) handleBeschlussSpeichern(w http.ResponseWriter, r *http.Request) {
	var in Beschluss
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in.Titel, in.Text, in.Stichworte, in.Nummer = trim(in.Titel, 200), trim(in.Text, 5000), trim(in.Stichworte, 200), trim(in.Nummer, 30)
	if in.Titel == "" || !gueltigesDatum(in.Datum, false) {
		writeErr(w, bad("Bitte Datum und Beschluss eintragen"))
		return
	}
	if in.Gremium != "Vorstand" {
		in.Gremium = "Mitgliederversammlung"
	}
	if in.Ergebnis != "abgelehnt" && in.Ergebnis != "zur Kenntnis" {
		in.Ergebnis = "angenommen"
	}
	if in.Ja < 0 || in.Nein < 0 || in.Enthaltung < 0 || in.Ja > 100000 || in.Nein > 100000 || in.Enthaltung > 100000 {
		writeErr(w, bad("Bitte die Stimmen prüfen"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if in.Nummer == "" {
		in.Nummer = a.st.beschlussNummerLocked(in)
	}
	for i := range a.st.d.Beschluesse {
		if in.ID != "" && a.st.d.Beschluesse[i].ID == in.ID {
			a.st.d.Beschluesse[i] = in
			a.st.audit("Beschluss %s geändert", in.Nummer)
			a.speichernUndAntworten(w, in)
			return
		}
	}
	if len(a.st.d.Beschluesse) >= maxVerwaltung {
		writeErr(w, bad("Zu viele Beschlüsse"))
		return
	}
	in.ID = newID()
	a.st.d.Beschluesse = append(a.st.d.Beschluesse, in)
	a.st.audit("Beschluss %s eingetragen", in.Nummer)
	a.speichernUndAntworten(w, in)
}

// beschlussNummerLocked vergibt die nächste Nummer je Gremium und Jahr, z. B. »MV 2025/3«. Der Aufrufer hält s.mu.
func (s *Store) beschlussNummerLocked(in Beschluss) string {
	kurz := "MV"
	if in.Gremium == "Vorstand" {
		kurz = "V"
	}
	// höchste vergebene Nummer + 1, damit nach dem Löschen eines Beschlusses keine Nummer doppelt vorkommt
	praefix := fmt.Sprintf("%s %s/", kurz, in.Datum[:4])
	n := 0
	for _, b := range s.d.Beschluesse {
		if b.ID != in.ID && strings.HasPrefix(b.Nummer, praefix) {
			if x, err := strconv.Atoi(strings.TrimPrefix(b.Nummer, praefix)); err == nil {
				n = max(n, x)
			}
		}
	}
	return fmt.Sprintf("%s%d", praefix, n+1)
}

func (a *App) handleBeschlussLoeschen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i, b := range a.st.d.Beschluesse {
		if b.ID == r.PathValue("id") {
			a.st.d.Beschluesse = append(a.st.d.Beschluesse[:i], a.st.d.Beschluesse[i+1:]...)
			a.st.audit("Beschluss %s gelöscht", b.Nummer)
			a.speichernUndAntworten(w, map[string]bool{"ok": true})
			return
		}
	}
	writeErr(w, notFound("Beschluss nicht gefunden"))
}

// beschlussbuchPDF: alle Beschlüsse (oder eines Jahres), ältester zuerst.
func (a *App) beschlussbuchPDF(r *http.Request) ([]byte, string, error) {
	jahr := strings.TrimSpace(r.URL.Query().Get("jahr"))
	a.st.mu.Lock()
	s := a.st.d.Settings
	var l []Beschluss
	for _, b := range a.st.d.Beschluesse {
		if jahr == "" || strings.HasPrefix(b.Datum, jahr+"-") {
			l = append(l, b)
		}
	}
	a.st.mu.Unlock()
	if len(l) == 0 {
		return nil, "", bad("Keine Beschlüsse vorhanden")
	}
	sort.SliceStable(l, func(i, j int) bool { return l[i].Datum < l[j].Datum })
	titel := "Beschlussbuch"
	if jahr != "" {
		titel += " " + jahr
	}
	pdf := neuesPDF(s, titel, false)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	y := mappeAbschnitt(pdf, s, 36, titel)
	for _, b := range l {
		if y > 250 {
			mappeFuss(pdf, s, s.VereinName+" · "+titel)
			pdf.AddPage()
			y = 20
		}
		pdf.SetFont("lib", "B", 10)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(rightX-leftX, 5.5, fmt.Sprintf("%s · %s · %s", b.Nummer, germanDate(b.Datum), b.Gremium), "", 0, "L", false, 0, "")
		y += 5.5
		pdf.SetFont("lib", "B", 10.5)
		pdf.SetXY(leftX, y)
		pdf.MultiCell(rightX-leftX, 5.3, b.Titel, "", "L", false)
		y = pdf.GetY()
		if b.Text != "" {
			pdf.SetFont("lib", "", 9.5)
			pdf.SetXY(leftX, y)
			pdf.MultiCell(rightX-leftX, 4.8, b.Text, "", "L", false)
			y = pdf.GetY()
		}
		ergebnis := b.Ergebnis
		if b.Ja+b.Nein+b.Enthaltung > 0 {
			ergebnis += fmt.Sprintf(" (Ja %d, Nein %d, Enthaltung %d)", b.Ja, b.Nein, b.Enthaltung)
		}
		pdf.SetFont("lib", "", 9)
		pdf.SetTextColor(90, 90, 90)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(rightX-leftX, 5, "Ergebnis: "+ergebnis, "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		y += 9
	}
	mappeFuss(pdf, s, s.VereinName+" · "+titel+" · Stand "+time.Now().Format("02.01.2006"))
	out, err := pdfBytes(pdf)
	return out, "Beschlussbuch.pdf", err
}

// ------------------------------------------------------------------ Verträge

type Vertrag struct {
	ID          string  `json:"id"`
	Art         string  `json:"art"`
	Bezeichnung string  `json:"bezeichnung"`
	Partner     string  `json:"partner,omitempty"`
	Nummer      string  `json:"nummer,omitempty"`
	Beginn      string  `json:"beginn,omitempty"`
	Ende        string  `json:"ende,omitempty"`        // Ende der aktuellen Laufzeit (leer = unbefristet)
	FristMonate int     `json:"fristMonate,omitempty"` // Kündigungsfrist vor dem Ende
	Verlaengert int     `json:"verlaengert,omitempty"` // verlängert sich ohne Kündigung um so viele Monate (0 = endet)
	Kosten      float64 `json:"kosten,omitempty"`
	Turnus      string  `json:"turnus,omitempty"` // jährlich | vierteljährlich | monatlich | einmalig
	Notiz       string  `json:"notiz,omitempty"`
	Gekuendigt  string  `json:"gekuendigt,omitempty"` // Datum der Kündigung
}

var vertragsArten = []string{"Versicherung", "Wasser", "Strom", "Pacht (Zwischenpacht)", "Bank und Konto", "Wartung", "Sonstiges"}

type vertragAnsicht struct {
	Vertrag
	LaeuftBis    string  `json:"laeuftBis,omitempty"`    // Ende der laufenden Periode (mit Verlängerungen)
	KuendigenBis string  `json:"kuendigenBis,omitempty"` // letzter Tag für die Kündigung zum LaeuftBis
	Jaehrlich    float64 `json:"jaehrlich"`              // Kosten aufs Jahr gerechnet
}

func (v Vertrag) ansicht(heute time.Time) vertragAnsicht {
	a := vertragAnsicht{Vertrag: v}
	switch v.Turnus {
	case "monatlich":
		a.Jaehrlich = round2(v.Kosten * 12)
	case "vierteljährlich":
		a.Jaehrlich = round2(v.Kosten * 4)
	case "einmalig":
	default:
		a.Jaehrlich = v.Kosten
	}
	ende, err := parseDate(v.Ende)
	if err != nil {
		return a
	}
	h := time.Date(heute.Year(), heute.Month(), heute.Day(), 0, 0, 0, 0, time.UTC)
	for i := 0; v.Verlaengert > 0 && v.Gekuendigt == "" && ende.Before(h) && i < 1200; i++ {
		ende = ende.AddDate(0, v.Verlaengert, 0)
	}
	a.LaeuftBis = ende.Format("2006-01-02")
	a.KuendigenBis = monateZurueck(ende, v.FristMonate).Format("2006-01-02")
	return a
}

// monateZurueck geht n Monate zurück und bleibt am Monatsende (31.12. minus 3 Monate = 30.09., nicht 01.10.).
func monateZurueck(t time.Time, n int) time.Time {
	r := t.AddDate(0, -n, 0)
	if r.Day() != t.Day() {
		r = r.AddDate(0, 0, -r.Day())
	}
	return r
}

func (a *App) handleVertraege(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	heute := time.Now()
	out := []vertragAnsicht{}
	summe := 0.0
	for _, v := range a.st.d.Vertraege {
		x := v.ansicht(heute)
		out = append(out, x)
		summe += x.Jaehrlich
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Art != out[j].Art {
			return out[i].Art < out[j].Art
		}
		return out[i].Bezeichnung < out[j].Bezeichnung
	})
	writeJSON(w, 200, map[string]any{"vertraege": out, "arten": vertragsArten, "jaehrlich": round2(summe)})
}

func (a *App) handleVertragSpeichern(w http.ResponseWriter, r *http.Request) {
	var in Vertrag
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in.Art, in.Bezeichnung, in.Partner, in.Nummer = trim(in.Art, 40), trim(in.Bezeichnung, 120), trim(in.Partner, 120), trim(in.Nummer, 60)
	in.Notiz, in.Turnus = trim(in.Notiz, 1000), trim(in.Turnus, 20)
	if in.Bezeichnung == "" {
		writeErr(w, bad("Bitte den Vertrag bezeichnen"))
		return
	}
	if in.Art == "" {
		in.Art = "Sonstiges"
	}
	if !gueltigesDatum(in.Beginn, true) || !gueltigesDatum(in.Ende, true) || !gueltigesDatum(in.Gekuendigt, true) ||
		in.FristMonate < 0 || in.FristMonate > 60 || in.Verlaengert < 0 || in.Verlaengert > 120 || !validNum(in.Kosten) {
		writeErr(w, bad("Bitte Daten, Fristen und Kosten prüfen"))
		return
	}
	in.Kosten = round2(in.Kosten)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Vertraege {
		if in.ID != "" && a.st.d.Vertraege[i].ID == in.ID {
			a.st.d.Vertraege[i] = in
			a.st.audit("Vertrag geändert: %s", in.Bezeichnung)
			a.speichernUndAntworten(w, in)
			return
		}
	}
	if len(a.st.d.Vertraege) >= maxVerwaltung {
		writeErr(w, bad("Zu viele Verträge"))
		return
	}
	in.ID = newID()
	a.st.d.Vertraege = append(a.st.d.Vertraege, in)
	a.st.audit("Vertrag eingetragen: %s", in.Bezeichnung)
	a.speichernUndAntworten(w, in)
}

func (a *App) handleVertragLoeschen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i, v := range a.st.d.Vertraege {
		if v.ID == r.PathValue("id") {
			a.st.d.Vertraege = append(a.st.d.Vertraege[:i], a.st.d.Vertraege[i+1:]...)
			a.st.audit("Vertrag gelöscht: %s", v.Bezeichnung)
			a.speichernUndAntworten(w, map[string]bool{"ok": true})
			return
		}
	}
	writeErr(w, notFound("Vertrag nicht gefunden"))
}

// vertragAufgabenLocked: Kündigungstermin in den nächsten 60 Tagen, abgelaufene Verträge.
func (s *Store) vertragAufgabenLocked(heute time.Time, add func(stufe, titel, text, ziel, knopf string)) {
	for _, v := range s.d.Vertraege {
		x := v.ansicht(heute)
		if x.LaeuftBis == "" {
			continue
		}
		if v.Gekuendigt == "" {
			if n := tageBis(heute, x.KuendigenBis); n >= 0 && n <= 60 {
				stufe := "info"
				if n <= 14 {
					stufe = "warn"
				}
				add(stufe, fmt.Sprintf("%s: bis %s kündbar", v.Bezeichnung, germanDate(x.KuendigenBis)),
					fmt.Sprintf("Läuft bis %s und verlängert sich sonst. Jetzt prüfen, ob es ein besseres Angebot gibt.", germanDate(x.LaeuftBis)), "admin:verwaltung:vertraege", "Verträge")
			}
		}
		if (v.Verlaengert == 0 || v.Gekuendigt != "") && tageBis(heute, x.LaeuftBis) < 0 && tageBis(heute, x.LaeuftBis) >= -60 {
			add("info", fmt.Sprintf("%s ist am %s ausgelaufen", v.Bezeichnung, germanDate(x.LaeuftBis)), "Neuen Vertrag eintragen oder den alten löschen.", "admin:verwaltung:vertraege", "Verträge")
		}
	}
}

// ------------------------------------------------------------------ Haushaltsplan

// Haushaltsplan: geplante Einnahmen und Ausgaben eines Jahres je Posten (Soll), verglichen mit dem Ist aus
// dem Kassenbericht.
type Haushaltsplan struct {
	Einnahmen map[string]float64 `json:"einnahmen"`
	Ausgaben  map[string]float64 `json:"ausgaben"`
	Notiz     string             `json:"notiz,omitempty"`
}

var haushaltEinnahmen = []string{"Pacht", "Mitgliedsbeiträge und Umlage", "Wasser und Strom", "Spenden", "Sonstige Einnahmen"}

type haushaltZeile struct {
	Posten string   `json:"posten"`
	Soll   *float64 `json:"soll,omitempty"`
	Ist    float64  `json:"ist"`
}

type haushaltAnsicht struct {
	Jahr      int             `json:"jahr"`
	Einnahmen []haushaltZeile `json:"einnahmen"`
	Ausgaben  []haushaltZeile `json:"ausgaben"`
	Notiz     string          `json:"notiz"`
	Vorjahr   bool            `json:"vorjahr"` // es gibt Ist-Werte des Vorjahres zum Übernehmen
}

// haushaltIstLocked: Ist-Werte aus dem Kassenbericht (Rechnungsbeträge, Spenden, Ausgaben je Kategorie).
func (s *Store) haushaltIstLocked(jahr int) (map[string]float64, map[string]float64, bool) {
	v, ok := s.viewLocked(jahr)
	if !ok {
		return map[string]float64{}, map[string]float64{}, false
	}
	k := s.kassenberichtLocked(v)
	ein := map[string]float64{
		"Pacht":                        round2(k.Summen.PachtGarten + k.Summen.PachtVerein + k.Summen.PachtFrei),
		"Mitgliedsbeiträge und Umlage": round2(k.Summen.Mitgliedsbeitrag + k.Summen.Umlage),
		"Wasser und Strom":             round2(k.Summen.KostenWasser + k.Summen.KostenEnergie),
		"Spenden":                      round2(k.Spenden),
	}
	aus := map[string]float64{}
	for _, x := range k.AusgabenKategorie {
		aus[x.Kategorie] = x.Summe
	}
	return ein, aus, true
}

func (s *Store) haushaltAnsichtLocked(jahr int) haushaltAnsicht {
	plan := s.d.Haushalt[strconv.Itoa(jahr)]
	ein, aus, _ := s.haushaltIstLocked(jahr)
	_, _, vj := s.haushaltIstLocked(jahr - 1)
	a := haushaltAnsicht{Jahr: jahr, Notiz: plan.Notiz, Vorjahr: vj, Einnahmen: []haushaltZeile{}, Ausgaben: []haushaltZeile{}}
	zeile := func(posten string, soll map[string]float64, ist map[string]float64) haushaltZeile {
		z := haushaltZeile{Posten: posten, Ist: ist[posten]}
		if x, ok := soll[posten]; ok {
			z.Soll = &x
		}
		return z
	}
	for _, p := range haushaltEinnahmen {
		a.Einnahmen = append(a.Einnahmen, zeile(p, plan.Einnahmen, ein))
	}
	posten := append([]string(nil), AusgabenKategorien...)
	bekannt := map[string]bool{}
	for _, p := range posten {
		bekannt[p] = true
	}
	var extra []string
	for p := range aus {
		if !bekannt[p] {
			extra = append(extra, p)
			bekannt[p] = true
		}
	}
	for p := range plan.Ausgaben {
		if !bekannt[p] {
			extra = append(extra, p)
			bekannt[p] = true
		}
	}
	sort.Strings(extra)
	for _, p := range append(posten, extra...) {
		a.Ausgaben = append(a.Ausgaben, zeile(p, plan.Ausgaben, aus))
	}
	return a
}

func (a *App) handleHaushalt(w http.ResponseWriter, r *http.Request) {
	jahr := a.yearParam(r)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	writeJSON(w, 200, a.st.haushaltAnsichtLocked(jahr))
}

func (a *App) handleHaushaltSpeichern(w http.ResponseWriter, r *http.Request) {
	jahr := a.yearParam(r)
	var in Haushaltsplan
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	pruefen := func(m map[string]float64) (map[string]float64, error) {
		out := map[string]float64{}
		if len(m) > 100 {
			return nil, bad("Zu viele Posten")
		}
		for k, v := range m {
			k = trim(k, 60)
			if k == "" || !validNum(v) {
				return nil, bad("Bitte nur Beträge ab 0 eintragen")
			}
			out[k] = round2(v)
		}
		return out, nil
	}
	ein, err := pruefen(in.Einnahmen)
	if err != nil {
		writeErr(w, err)
		return
	}
	aus, err := pruefen(in.Ausgaben)
	if err != nil {
		writeErr(w, err)
		return
	}
	if jahr < 2000 || jahr > 2200 {
		writeErr(w, bad("Ungültiges Jahr"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if a.st.d.Haushalt == nil {
		a.st.d.Haushalt = map[string]Haushaltsplan{}
	}
	a.st.d.Haushalt[strconv.Itoa(jahr)] = Haushaltsplan{Einnahmen: ein, Ausgaben: aus, Notiz: trim(in.Notiz, 2000)}
	a.st.audit("Haushaltsplan %d gespeichert", jahr)
	a.speichernUndAntworten(w, a.st.haushaltAnsichtLocked(jahr))
}

// haushaltPDF: Plan und Ist eines Jahres für Mitgliederversammlung und Kassenprüfer.
func (a *App) haushaltPDF(r *http.Request) ([]byte, string, error) {
	jahr := a.yearParam(r)
	a.st.mu.Lock()
	h := a.st.haushaltAnsichtLocked(jahr)
	s := a.st.d.Settings
	a.st.mu.Unlock()
	pdf := neuesPDF(s, fmt.Sprintf("Haushaltsplan %d", jahr), false)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	y := mappeAbschnitt(pdf, s, 36, fmt.Sprintf("Haushaltsplan %d – Plan und Ist", jahr))
	tabelle := func(titel string, l []haushaltZeile) {
		pdf.SetFont("lib", "B", 10)
		pdf.SetFillColor(240, 243, 238)
		pdf.SetXY(leftX, y)
		for i, t := range []string{titel, "Plan", "Ist", "Abweichung"} {
			al := "R"
			w := 35.0
			if i == 0 {
				al, w = "L", rightX-leftX-105
			}
			pdf.CellFormat(w, 6.5, t, "", 0, al, true, 0, "")
		}
		y += 7
		var sp, si, sa float64 // Abweichung nur über geplante Posten
		for _, z := range l {
			soll, abw := "–", "–"
			if z.Soll != nil {
				soll, abw = fmtEUR(*z.Soll), fmtEUR(round2(z.Ist-*z.Soll))
				sp += *z.Soll
				sa += z.Ist - *z.Soll
			}
			if z.Soll == nil && z.Ist == 0 {
				continue
			}
			si += z.Ist
			pdf.SetFont("lib", "", 10)
			pdf.SetXY(leftX, y)
			pdf.CellFormat(rightX-leftX-105, 6, z.Posten, "", 0, "L", false, 0, "")
			pdf.CellFormat(35, 6, soll, "", 0, "R", false, 0, "")
			pdf.CellFormat(35, 6, fmtEUR(z.Ist), "", 0, "R", false, 0, "")
			pdf.CellFormat(35, 6, abw, "", 0, "R", false, 0, "")
			y += 6
		}
		pdf.SetFont("lib", "B", 10)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(rightX-leftX-105, 6.5, "Summe", "T", 0, "L", false, 0, "")
		pdf.CellFormat(35, 6.5, fmtEUR(round2(sp)), "T", 0, "R", false, 0, "")
		pdf.CellFormat(35, 6.5, fmtEUR(round2(si)), "T", 0, "R", false, 0, "")
		pdf.CellFormat(35, 6.5, fmtEUR(round2(sa)), "T", 0, "R", false, 0, "")
		y += 13
	}
	tabelle("Einnahmen", h.Einnahmen)
	tabelle("Ausgaben", h.Ausgaben)
	if h.Notiz != "" {
		y = mappeAbschnitt(pdf, s, y, "Erläuterungen")
		mappeText(pdf, y, h.Notiz, 10)
	}
	mappeFuss(pdf, s, fmt.Sprintf("%s · Haushaltsplan %d · Ist-Werte laut Kassenbericht, Stand %s", s.VereinName, jahr, time.Now().Format("02.01.2006")))
	out, err := pdfBytes(pdf)
	return out, fmt.Sprintf("Haushaltsplan_%d.pdf", jahr), err
}
