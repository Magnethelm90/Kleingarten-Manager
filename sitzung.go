package main

import (
	"fmt"
	"net/http"
	"strings"
)

// Protokoll einer Versammlung oder Vorstandssitzung zu einem Termin aus dem Jahresplaner. Die Tagesordnung kommt
// aus der Einladung; Abstimmungen landen automatisch im Beschlussbuch (und werden dort mit dem Protokoll
// aktuell gehalten). Am Ende steht ein Protokoll-PDF mit Unterschriftszeilen.

type Abstimmung struct {
	Titel      string `json:"titel"`
	Ja         int    `json:"ja"`
	Nein       int    `json:"nein"`
	Enthaltung int    `json:"enthaltung"`
	Ergebnis   string `json:"ergebnis"`
}

type ProtokollPunkt struct {
	Titel       string      `json:"titel"`
	Text        string      `json:"text,omitempty"`
	Beschluss   *Abstimmung `json:"beschluss,omitempty"`
	BeschlussID string      `json:"beschlussId,omitempty"`
}

type Sitzungsprotokoll struct {
	TerminID         string           `json:"terminId"`
	Datum            string           `json:"datum"`
	Gremium          string           `json:"gremium"` // Mitgliederversammlung | Vorstand
	Ort              string           `json:"ort,omitempty"`
	Beginn           string           `json:"beginn,omitempty"`
	Ende             string           `json:"ende,omitempty"`
	Leitung          string           `json:"leitung,omitempty"`
	Protokollfuehrer string           `json:"protokollfuehrer,omitempty"`
	Anwesend         int              `json:"anwesend,omitempty"`
	Stimmberechtigt  int              `json:"stimmberechtigt,omitempty"`
	Beschlussfaehig  bool             `json:"beschlussfaehig"`
	Punkte           []ProtokollPunkt `json:"punkte"`
}

type sitzungAntwort struct {
	Protokoll  Sitzungsprotokoll `json:"protokoll"`
	Neu        bool              `json:"neu"`
	Termin     Termin            `json:"termin"`
	Mitglieder int               `json:"mitglieder"` // aktive Pächter, als Vorschlag für »stimmberechtigt«
	Nummern    map[string]string `json:"nummern"`    // BeschlussID -> Nummer im Beschlussbuch
}

func (s *Store) sitzungAntwortLocked(t Termin) sitzungAntwort {
	p, ok := s.d.Sitzungen[t.ID]
	if !ok {
		p = Sitzungsprotokoll{TerminID: t.ID, Datum: t.Datum, Gremium: "Mitgliederversammlung", Ort: t.Ort, Beginn: t.Von, Beschlussfaehig: true}
		if t.Art == "Vorstand" {
			p.Gremium = "Vorstand"
		}
		for _, top := range tagesordnung(t) {
			if top = strings.TrimSpace(top); top != "" {
				p.Punkte = append(p.Punkte, ProtokollPunkt{Titel: top})
			}
		}
	}
	if p.Punkte == nil {
		p.Punkte = []ProtokollPunkt{}
	}
	a := sitzungAntwort{Protokoll: p, Neu: !ok, Termin: t, Nummern: map[string]string{}}
	for _, x := range s.d.Paechter {
		if !x.Geloescht && x.AusgeschiedenAm == "" {
			a.Mitglieder++
		}
	}
	for _, b := range s.d.Beschluesse {
		a.Nummern[b.ID] = b.Nummer
	}
	return a
}

func (s *Store) terminLocked(id string) (Termin, bool) {
	for _, t := range s.d.Termine {
		if t.ID == id {
			return t, true
		}
	}
	return Termin{}, false
}

func (a *App) handleSitzung(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	t, ok := a.st.terminLocked(r.PathValue("id"))
	if !ok {
		writeErr(w, notFound("Termin nicht gefunden"))
		return
	}
	writeJSON(w, 200, a.st.sitzungAntwortLocked(t))
}

func cleanSitzung(in Sitzungsprotokoll) (Sitzungsprotokoll, error) {
	in.Ort, in.Leitung, in.Protokollfuehrer = trim(in.Ort, 120), trim(in.Leitung, 100), trim(in.Protokollfuehrer, 100)
	in.Beginn, in.Ende = trim(in.Beginn, 5), trim(in.Ende, 5)
	if (in.Beginn != "" && !uhrzeitGueltig(in.Beginn)) || (in.Ende != "" && !uhrzeitGueltig(in.Ende)) {
		return in, bad("Bitte Beginn und Ende als Uhrzeit eintragen, z. B. 18:00")
	}
	if in.Anwesend < 0 || in.Stimmberechtigt < 0 || in.Anwesend > 100000 || in.Stimmberechtigt > 100000 {
		return in, bad("Bitte die Zahl der Anwesenden prüfen")
	}
	if len(in.Punkte) > 60 {
		return in, bad("Zu viele Tagesordnungspunkte")
	}
	punkte := []ProtokollPunkt{}
	for _, p := range in.Punkte {
		p.Titel, p.Text = trim(p.Titel, 200), trim(p.Text, 10000)
		if p.Titel == "" && p.Text == "" && p.Beschluss == nil {
			continue
		}
		if p.Beschluss != nil {
			b := p.Beschluss
			b.Titel = trim(b.Titel, 200)
			if b.Ergebnis != "abgelehnt" && b.Ergebnis != "zur Kenntnis" {
				b.Ergebnis = "angenommen"
			}
			if b.Ja < 0 || b.Nein < 0 || b.Enthaltung < 0 || b.Ja > 100000 || b.Nein > 100000 || b.Enthaltung > 100000 {
				return in, bad("Bitte die Stimmen prüfen")
			}
			if b.Titel == "" {
				p.Beschluss = nil
			}
		}
		punkte = append(punkte, p)
	}
	in.Punkte = punkte
	return in, nil
}

// handleSitzungSpeichern speichert das Protokoll und gleicht das Beschlussbuch ab: neue Abstimmungen werden
// eingetragen, geänderte aktualisiert (die Nummer bleibt), entfernte gelöscht.
func (a *App) handleSitzungSpeichern(w http.ResponseWriter, r *http.Request) {
	var in Sitzungsprotokoll
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in, err := cleanSitzung(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	t, ok := a.st.terminLocked(r.PathValue("id"))
	if !ok {
		writeErr(w, notFound("Termin nicht gefunden"))
		return
	}
	in.TerminID, in.Datum = t.ID, t.Datum
	if in.Gremium != "Vorstand" {
		in.Gremium = "Mitgliederversammlung"
	}
	if len(a.st.d.Beschluesse)+len(in.Punkte) > maxVerwaltung {
		writeErr(w, bad("Zu viele Beschlüsse"))
		return
	}
	alt := map[string]bool{}
	for _, p := range a.st.d.Sitzungen[t.ID].Punkte {
		if p.BeschlussID != "" {
			alt[p.BeschlussID] = true
		}
	}
	for i := range in.Punkte {
		p := &in.Punkte[i]
		if !alt[p.BeschlussID] {
			p.BeschlussID = "" // nur eigene Beschlüsse verknüpfen, nie fremde aus dem Beschlussbuch
		}
		if p.Beschluss == nil {
			continue
		}
		b := Beschluss{ID: p.BeschlussID, Datum: t.Datum, Gremium: in.Gremium, Titel: p.Beschluss.Titel, Ja: p.Beschluss.Ja, Nein: p.Beschluss.Nein,
			Enthaltung: p.Beschluss.Enthaltung, Ergebnis: p.Beschluss.Ergebnis, Text: "Protokoll, TOP " + fmt.Sprint(i+1) + ": " + p.Titel}
		if j := a.st.beschlussIndexLocked(b.ID); j >= 0 {
			b.Nummer, b.Stichworte = a.st.d.Beschluesse[j].Nummer, a.st.d.Beschluesse[j].Stichworte
			if b.Nummer == "" || !strings.Contains(b.Nummer, t.Datum[:4]) || a.st.d.Beschluesse[j].Gremium != b.Gremium {
				b.Nummer = a.st.beschlussNummerLocked(b)
			}
			a.st.d.Beschluesse[j] = b
		} else {
			b.ID = newID()
			b.Nummer = a.st.beschlussNummerLocked(b)
			a.st.d.Beschluesse = append(a.st.d.Beschluesse, b)
			p.BeschlussID = b.ID
		}
		delete(alt, b.ID)
	}
	for i := range in.Punkte {
		if in.Punkte[i].Beschluss == nil {
			in.Punkte[i].BeschlussID = ""
		}
	}
	for id := range alt { // Abstimmung aus dem Protokoll entfernt: auch aus dem Beschlussbuch
		if j := a.st.beschlussIndexLocked(id); j >= 0 {
			a.st.d.Beschluesse = append(a.st.d.Beschluesse[:j], a.st.d.Beschluesse[j+1:]...)
		}
	}
	if a.st.d.Sitzungen == nil {
		a.st.d.Sitzungen = map[string]Sitzungsprotokoll{}
	}
	a.st.d.Sitzungen[t.ID] = in
	a.st.audit("Protokoll gespeichert: %s am %s", t.Titel, germanDate(t.Datum))
	a.speichernUndAntworten(w, a.st.sitzungAntwortLocked(t))
}

func (s *Store) beschlussIndexLocked(id string) int {
	for i, b := range s.d.Beschluesse {
		if id != "" && b.ID == id {
			return i
		}
	}
	return -1
}

func (a *App) sitzungPDF(r *http.Request) ([]byte, string, error) {
	a.st.mu.Lock()
	s := a.st.d.Settings
	t, ok := a.st.terminLocked(r.PathValue("id"))
	var x sitzungAntwort
	if ok {
		x = a.st.sitzungAntwortLocked(t)
	}
	a.st.mu.Unlock()
	if !ok {
		return nil, "", notFound("Termin nicht gefunden")
	}
	p := x.Protokoll
	titel := "Protokoll der " + p.Gremium
	if p.Gremium == "Vorstand" {
		titel = "Protokoll der Vorstandssitzung"
	}
	pdf := neuesPDF(s, titel, false)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	y := mappeAbschnitt(pdf, s, 36, titel+" am "+germanDate(p.Datum))
	zeit := p.Beginn
	if zeit != "" && p.Ende != "" {
		zeit += " bis " + p.Ende
	}
	if zeit != "" {
		zeit += " Uhr"
	}
	anwesend := ""
	if p.Anwesend > 0 {
		anwesend = fmt.Sprint(p.Anwesend)
		if p.Stimmberechtigt > 0 {
			anwesend += fmt.Sprintf(" von %d stimmberechtigten Mitgliedern", p.Stimmberechtigt)
		}
	}
	faehig := "ja"
	if !p.Beschlussfaehig {
		faehig = "nein"
	}
	y = mappeTabelle(pdf, y, [][2]string{{"Ort", p.Ort}, {"Zeit", zeit}, {"Versammlungsleitung", p.Leitung}, {"Protokoll", p.Protokollfuehrer},
		{"Anwesend", anwesend}, {"Beschlussfähig", faehig}})
	seite := func(h float64) {
		if y+h > 262 {
			mappeFuss(pdf, s, s.VereinName+" · "+titel+" am "+germanDate(p.Datum))
			pdf.AddPage()
			y = 20
		}
	}
	for i, pu := range p.Punkte {
		seite(16)
		pdf.SetFont("lib", "B", 11)
		pdf.SetXY(leftX, y)
		pdf.MultiCell(rightX-leftX, 5.5, fmt.Sprintf("TOP %d: %s", i+1, pu.Titel), "", "L", false)
		y = pdf.GetY() + 1
		if pu.Text != "" {
			for _, absatz := range strings.Split(pu.Text, "\n") {
				seite(6)
				pdf.SetFont("lib", "", 10)
				pdf.SetXY(leftX, y)
				pdf.MultiCell(rightX-leftX, 5, absatz, "", "L", false)
				y = pdf.GetY()
			}
			y++
		}
		if b := pu.Beschluss; b != nil {
			seite(14)
			nr := x.Nummern[pu.BeschlussID]
			kopf := "Beschluss"
			if nr != "" {
				kopf += " " + nr
			}
			stimmen := ""
			if b.Ja+b.Nein+b.Enthaltung > 0 {
				stimmen = fmt.Sprintf(" (Ja %d, Nein %d, Enthaltung %d)", b.Ja, b.Nein, b.Enthaltung)
			}
			pdf.SetFillColor(240, 243, 238)
			pdf.SetFont("lib", "B", 10)
			pdf.SetXY(leftX, y)
			pdf.MultiCell(rightX-leftX, 5.2, kopf+": "+b.Titel+" – "+b.Ergebnis+stimmen, "", "L", true)
			y = pdf.GetY() + 1
		}
		y += 3
	}
	seite(30)
	y = max(y+16, 250.0)
	for i, rolle := range []string{"Versammlungsleitung", "Protokoll"} {
		if p.Gremium == "Vorstand" && i == 0 {
			rolle = "Sitzungsleitung"
		}
		x0 := leftX + float64(i)*(rightX-leftX)/2
		pdf.Line(x0, y, x0+(rightX-leftX)/2-10, y)
		pdf.SetFont("lib", "", 8.5)
		pdf.SetXY(x0, y+1)
		pdf.CellFormat((rightX-leftX)/2-10, 4.5, "Datum, Unterschrift – "+rolle, "", 0, "L", false, 0, "")
	}
	mappeFuss(pdf, s, s.VereinName+" · "+titel+" am "+germanDate(p.Datum))
	out, err := pdfBytes(pdf)
	return out, safeName("Protokoll_"+p.Datum, 60) + ".pdf", err
}
