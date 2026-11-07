package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Kassenprüfung: geführte Prüfung eines Jahres durch die Kassenprüfer – Kontostand abgleichen, Belege und
// eine Stichprobe prüfen, offene Forderungen nachvollziehen – und am Ende ein fertiger Prüfbericht für die
// Mitgliederversammlung. Kassenprüfer können sie über ihren eigenen Zugang bearbeiten.

type Kassenpruefung struct {
	Datum          string          `json:"datum"`
	Pruefer        string          `json:"pruefer"`              // Namen, durch Komma oder Zeilenumbruch getrennt
	Kontostand     *float64        `json:"kontostand,omitempty"` // laut Kontoauszug zum Jahresende
	Barkasse       *float64        `json:"barkasse,omitempty"`
	Punkte         map[string]bool `json:"punkte"`
	Stichprobe     []string        `json:"stichprobe,omitempty"` // IDs der gezogenen Ausgaben
	Feststellungen string          `json:"feststellungen"`
	Ergebnis       string          `json:"ergebnis"` // "" offen, "ohne" bzw. "mit" Beanstandungen
	Entlastung     bool            `json:"entlastung"`
}

var pruefPunkte = []struct{ Key, Text string }{
	{"kontostand", "Kontostand laut Kontoauszug mit dem Kassenbestand abgeglichen"},
	{"belege", "Zu allen Ausgaben liegen Belege vor"},
	{"stichprobe", "Stichprobe der Ausgaben mit den Belegen verglichen"},
	{"forderungen", "Offene Forderungen und ausgezahlte Guthaben nachvollzogen"},
	{"beschluesse", "Größere Ausgaben sind durch Beschluss oder Haushaltsplan gedeckt"},
	{"spenden", "Spenden und Zuwendungsbestätigungen vollständig"},
}

type pruefPunkt struct {
	Key     string `json:"key"`
	Text    string `json:"text"`
	Hinweis string `json:"hinweis"`
	Warnung bool   `json:"warnung,omitempty"` // die Daten deuten auf ein Problem hin
}

type pruefAnsicht struct {
	Jahr           int            `json:"jahr"`
	Pruefung       Kassenpruefung `json:"pruefung"`
	Punkte         []pruefPunkt   `json:"punkte"`
	Anfangsbestand float64        `json:"anfangsbestand"`
	Saldo          float64        `json:"saldo"`
	Kassenbestand  float64        `json:"kassenbestand"`
	Differenz      *float64       `json:"differenz,omitempty"`
	Ausgaben       int            `json:"ausgaben"`
	OhneBeleg      []Ausgabe      `json:"ohneBeleg"`
	Ungeprueft     int            `json:"ungeprueft"`
	Stichprobe     []Ausgabe      `json:"stichprobe"`
	Offen          int            `json:"offen"`
	OffenSumme     float64        `json:"offenSumme"`
}

func (s *Store) pruefungLocked(jahr int) (pruefAnsicht, bool) {
	v, ok := s.viewLocked(jahr)
	if !ok {
		return pruefAnsicht{}, false
	}
	k := s.kassenberichtLocked(v)
	p := s.d.Kassenpruefungen[yearKey(jahr)]
	if p.Punkte == nil {
		p.Punkte = map[string]bool{}
	}
	a := pruefAnsicht{Jahr: jahr, Pruefung: p, Anfangsbestand: k.Anfangsbestand, Saldo: k.Saldo, Kassenbestand: k.Kassenbestand,
		Ausgaben: len(k.Ausgaben), OhneBeleg: []Ausgabe{}, Stichprobe: []Ausgabe{}}
	if p.Kontostand != nil {
		d := round2(*p.Kontostand - k.Kassenbestand)
		if p.Barkasse != nil {
			d = round2(d + *p.Barkasse)
		}
		a.Differenz = &d
	}
	gezogen := map[string]bool{}
	for _, id := range p.Stichprobe {
		gezogen[id] = true
	}
	for _, x := range k.Ausgaben {
		if len(x.Belege) == 0 {
			a.OhneBeleg = append(a.OhneBeleg, x)
		}
		if !x.Geprueft {
			a.Ungeprueft++
		}
		if gezogen[x.ID] {
			a.Stichprobe = append(a.Stichprobe, x)
		}
	}
	for _, r := range s.d.Rechnungen {
		if r.Jahr == jahr && r.Status == statusGueltig && r.Result.Gesamt > 0 && !r.Bereinigt {
			if o := openAmount(r); o > 0.004 {
				a.Offen++
				a.OffenSumme = round2(a.OffenSumme + o)
			}
		}
	}
	spenden, ohne := 0, 0
	for _, sp := range s.d.Spenden {
		if sp.Jahr == jahr {
			spenden++
			if sp.BestaetigtAm == "" {
				ohne++
			}
		}
	}
	beschluesse := 0
	for _, b := range s.d.Beschluesse {
		if strings.HasPrefix(b.Datum, fmt.Sprintf("%d-", jahr)) {
			beschluesse++
		}
	}
	_, plan := s.d.Haushalt[yearKey(jahr)]
	for _, pp := range pruefPunkte {
		pu := pruefPunkt{Key: pp.Key, Text: pp.Text}
		switch pp.Key {
		case "kontostand":
			switch {
			case a.Differenz == nil:
				pu.Hinweis = "Kontostand laut Auszug oben eintragen – das Programm rechnet die Differenz aus."
			case math.Abs(*a.Differenz) < 0.005:
				pu.Hinweis = "Stimmt überein mit dem Kassenbestand von " + fmtEUR(k.Kassenbestand) + "."
			default:
				pu.Hinweis, pu.Warnung = fmt.Sprintf("Differenz von %s zum Kassenbestand (%s).", fmtEUR(*a.Differenz), fmtEUR(k.Kassenbestand)), true
			}
		case "belege":
			if len(a.OhneBeleg) == 0 {
				pu.Hinweis = fmt.Sprintf("Alle %d Ausgaben haben einen Beleg.", a.Ausgaben)
			} else {
				pu.Hinweis, pu.Warnung = fmt.Sprintf("%d von %d Ausgaben ohne Beleg.", len(a.OhneBeleg), a.Ausgaben), true
			}
		case "stichprobe":
			if len(a.Stichprobe) == 0 {
				pu.Hinweis = "Noch keine Stichprobe gezogen."
			} else {
				ok := 0
				for _, x := range a.Stichprobe {
					if x.Geprueft {
						ok++
					}
				}
				pu.Hinweis = fmt.Sprintf("%d Ausgaben gezogen, %d davon abgehakt.", len(a.Stichprobe), ok)
				pu.Warnung = ok < len(a.Stichprobe)
			}
		case "forderungen":
			if a.Offen == 0 {
				pu.Hinweis = "Keine offenen Rechnungen."
			} else {
				pu.Hinweis = fmt.Sprintf("%d Rechnung(en) offen, zusammen %s.", a.Offen, fmtEUR(a.OffenSumme))
			}
		case "beschluesse":
			pu.Hinweis = fmt.Sprintf("%d Beschlüsse im Beschlussbuch", beschluesse)
			if plan {
				pu.Hinweis += ", Haushaltsplan vorhanden."
			} else {
				pu.Hinweis += ", kein Haushaltsplan."
			}
		case "spenden":
			if spenden == 0 {
				pu.Hinweis = "Keine Spenden erfasst."
			} else {
				pu.Hinweis = fmt.Sprintf("%d Spende(n), %d noch ohne Zuwendungsbestätigung.", spenden, ohne)
				pu.Warnung = ohne > 0
			}
		}
		a.Punkte = append(a.Punkte, pu)
	}
	return a, true
}

func (a *App) handlePruefung(w http.ResponseWriter, r *http.Request) {
	jahr := a.yearParam(r)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	x, ok := a.st.pruefungLocked(jahr)
	if !ok {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	writeJSON(w, 200, x)
}

func (a *App) handlePruefungSpeichern(w http.ResponseWriter, r *http.Request) {
	jahr := a.yearParam(r)
	var in Kassenpruefung
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in.Pruefer, in.Feststellungen = trim(in.Pruefer, 200), trim(in.Feststellungen, 3000)
	if !gueltigesDatum(in.Datum, true) || (in.Kontostand != nil && (math.IsNaN(*in.Kontostand) || math.IsInf(*in.Kontostand, 0))) || (in.Barkasse != nil && !validNum(*in.Barkasse)) {
		writeErr(w, bad("Bitte Datum und Beträge prüfen"))
		return
	}
	if in.Ergebnis != "" && in.Ergebnis != "ohne" && in.Ergebnis != "mit" {
		in.Ergebnis = ""
	}
	punkte := map[string]bool{}
	for _, pp := range pruefPunkte {
		if in.Punkte[pp.Key] {
			punkte[pp.Key] = true
		}
	}
	in.Punkte = punkte
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if _, ok := a.st.viewLocked(jahr); !ok {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	if a.st.d.Kassenpruefungen == nil {
		a.st.d.Kassenpruefungen = map[string]Kassenpruefung{}
	}
	in.Stichprobe = a.st.d.Kassenpruefungen[yearKey(jahr)].Stichprobe // nur über »Stichprobe ziehen«
	if in.Kontostand != nil {
		k := round2(*in.Kontostand)
		in.Kontostand = &k
	}
	a.st.d.Kassenpruefungen[yearKey(jahr)] = in
	a.st.audit("Kassenprüfung %d gespeichert", jahr)
	x, _ := a.st.pruefungLocked(jahr)
	a.speichernUndAntworten(w, x)
}

// handleStichprobe zieht neu: die drei größten Ausgaben und zufällig weitere, zusammen etwa 10 %, mindestens 5.
func (a *App) handleStichprobe(w http.ResponseWriter, r *http.Request) {
	jahr := a.yearParam(r)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	v, ok := a.st.viewLocked(jahr)
	if !ok {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	aus := append([]Ausgabe(nil), a.st.kassenberichtLocked(v).Ausgaben...)
	if len(aus) == 0 {
		writeErr(w, bad("In diesem Jahr gibt es keine Ausgaben"))
		return
	}
	sort.SliceStable(aus, func(i, j int) bool { return math.Abs(aus[i].Betrag) > math.Abs(aus[j].Betrag) })
	n := max(5, int(math.Ceil(float64(len(aus))/10)))
	n = min(n, len(aus))
	gross := min(3, n)
	ids := []string{}
	for _, x := range aus[:gross] {
		ids = append(ids, x.ID)
	}
	rest := aus[gross:]
	rand.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
	for _, x := range rest[:n-gross] {
		ids = append(ids, x.ID)
	}
	if a.st.d.Kassenpruefungen == nil {
		a.st.d.Kassenpruefungen = map[string]Kassenpruefung{}
	}
	p := a.st.d.Kassenpruefungen[yearKey(jahr)]
	p.Stichprobe = ids
	a.st.d.Kassenpruefungen[yearKey(jahr)] = p
	a.st.audit("Kassenprüfung %d: Stichprobe mit %d Ausgaben gezogen", jahr, len(ids))
	x, _ := a.st.pruefungLocked(jahr)
	a.speichernUndAntworten(w, x)
}

func pruefNamen(s string) []string {
	var out []string
	for _, n := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' || r == ';' }) {
		if n = strings.TrimSpace(n); n != "" && len(out) < 3 {
			out = append(out, n)
		}
	}
	return out
}

func (a *App) pruefberichtPDF(r *http.Request) ([]byte, string, error) {
	jahr := a.yearParam(r)
	a.st.mu.Lock()
	s := a.st.d.Settings
	x, ok := a.st.pruefungLocked(jahr)
	a.st.mu.Unlock()
	if !ok {
		return nil, "", notFound("Jahr nicht gefunden")
	}
	p := x.Pruefung
	titel := fmt.Sprintf("Bericht der Kassenprüfung %d", jahr)
	pdf := neuesPDF(s, titel, false)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	y := mappeAbschnitt(pdf, s, 36, titel)
	datum := "–"
	if p.Datum != "" {
		datum = germanDate(p.Datum)
	}
	y = mappeTabelle(pdf, y, [][2]string{
		{"Prüfungszeitraum", fmt.Sprintf("01.01.%d bis 31.12.%d", jahr, jahr)},
		{"Tag der Prüfung", datum},
		{"Kassenprüfer", strings.Join(pruefNamen(p.Pruefer), ", ")},
	})
	y = mappeAbschnitt(pdf, s, y, "Kassenstand")
	zeilen := [][2]string{
		{"Anfangsbestand", fmtEUR(x.Anfangsbestand)},
		{"Kassenbewegung im Jahr", fmtEUR(x.Saldo)},
		{"Kassenbestand laut Programm", fmtEUR(x.Kassenbestand)},
	}
	if p.Kontostand != nil {
		zeilen = append(zeilen, [2]string{"Kontostand laut Auszug", fmtEUR(*p.Kontostand)})
	}
	if p.Barkasse != nil {
		zeilen = append(zeilen, [2]string{"Barkasse", fmtEUR(*p.Barkasse)})
	}
	if x.Differenz != nil {
		zeilen = append(zeilen, [2]string{"Differenz", fmtEUR(*x.Differenz)})
	}
	y = mappeTabelle(pdf, y, zeilen)
	y = mappeAbschnitt(pdf, s, y, "Geprüft wurde")
	for _, pu := range x.Punkte {
		mark := "offen"
		if p.Punkte[pu.Key] {
			mark = "erledigt"
		}
		pdf.SetFont("lib", "B", 9.5)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(20, 5, mark, "", 0, "L", false, 0, "")
		pdf.SetFont("lib", "", 9.5)
		pdf.SetXY(leftX+20, y)
		pdf.MultiCell(rightX-leftX-20, 5, pu.Text+" – "+pu.Hinweis, "", "L", false)
		y = pdf.GetY() + 1
	}
	if len(x.Stichprobe) > 0 {
		t := []string{}
		for _, a := range x.Stichprobe {
			t = append(t, fmt.Sprintf("%s %s (%s)", germanDate(a.Datum), a.Beschreibung, fmtEUR(a.Betrag)))
		}
		pdf.SetFont("lib", "", 8.5)
		pdf.SetTextColor(90, 90, 90)
		pdf.SetXY(leftX+20, y)
		pdf.MultiCell(rightX-leftX-20, 4.3, "Stichprobe: "+strings.Join(t, "; "), "", "L", false)
		pdf.SetTextColor(0, 0, 0)
		y = pdf.GetY() + 2
	}
	y += 2
	if p.Feststellungen != "" {
		y = mappeAbschnitt(pdf, s, y, "Feststellungen")
		y = mappeText(pdf, y, p.Feststellungen, 10)
	}
	y = mappeAbschnitt(pdf, s, y, "Ergebnis")
	erg := "Die Prüfung ist noch nicht abgeschlossen."
	switch p.Ergebnis {
	case "ohne":
		erg = "Die Kasse wurde ordnungsgemäß geführt. Die Belege waren vollständig und sachlich richtig; Beanstandungen ergaben sich nicht."
	case "mit":
		erg = "Die Kasse wurde mit den oben genannten Beanstandungen geführt."
	}
	if p.Ergebnis != "" && p.Entlastung {
		erg += " Die Kassenprüfer empfehlen der Mitgliederversammlung, den Vorstand für das Geschäftsjahr " + fmt.Sprint(jahr) + " zu entlasten."
	}
	y = mappeText(pdf, y, erg, 10)
	y = max(y+14, 240.0)
	namen := pruefNamen(p.Pruefer)
	if len(namen) == 0 {
		namen = []string{"Kassenprüfer/in", "Kassenprüfer/in"}
	}
	breite := (rightX - leftX) / float64(len(namen))
	for i, n := range namen {
		x0 := leftX + float64(i)*breite
		pdf.Line(x0, y, x0+breite-8, y)
		pdf.SetFont("lib", "", 8.5)
		pdf.SetXY(x0, y+1)
		pdf.CellFormat(breite-8, 4.5, "Ort, Datum, Unterschrift – "+n, "", 0, "L", false, 0, "")
	}
	mappeFuss(pdf, s, s.VereinName+" · "+titel)
	out, err := pdfBytes(pdf)
	return out, fmt.Sprintf("Kassenpruefung-%d.pdf", jahr), err
}

// pruefungAufgabenLocked: Erinnerung an die Kassenprüfung des Vorjahres (bis Ende April) und an fehlende Belege.
func (s *Store) pruefungAufgabenLocked(heute time.Time, add func(stufe, titel, text, ziel, knopf string)) {
	vj := heute.Year() - 1
	if heute.Month() <= time.April {
		if _, ok := s.viewLocked(vj); ok && s.d.Kassenpruefungen[yearKey(vj)].Ergebnis == "" {
			add("info", fmt.Sprintf("Kassenprüfung %d steht an", vj),
				"Vor der Mitgliederversammlung prüfen die Kassenprüfer das Vorjahr. Die geführte Prüfung erstellt den Bericht gleich mit.", "admin:kassenbericht", "Zur Prüfung")
		}
	}
	if v, ok := s.viewLocked(s.d.Settings.Jahr); ok {
		n := 0
		for _, x := range s.kassenberichtLocked(v).Ausgaben {
			if len(x.Belege) == 0 && x.Betrag != 0 {
				n++
			}
		}
		if n > 0 {
			add("info", fmt.Sprintf("%d Ausgabe(n) ohne Beleg", n), "Quittung einscannen oder fotografieren und an die Ausgabe hängen – so geht die Kassenprüfung schneller.", "admin:kassenbericht:ohnebeleg", "Belege ergänzen")
		}
	}
}
