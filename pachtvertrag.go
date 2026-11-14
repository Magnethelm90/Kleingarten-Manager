package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Pachtvertrag: aus den Pächterdaten, den Preisen des Jahres und den Vertragsbestimmungen des Vereins (als
// Vorlage änderbar) entsteht ein Unterpachtvertrag zum Unterschreiben. Auf Wunsch wird er als PDF in die
// Gartenakte gelegt (gehört zur Person und wird mit ihr gelöscht).

const standardVertragsbestimmungen = `§ Nutzung
Der Garten dient der kleingärtnerischen Nutzung nach dem Bundeskleingartengesetz: Gewinnung von Obst, Gemüse und anderen Früchten für den Eigenbedarf und Erholung. Mindestens ein Drittel der Fläche wird für den Anbau genutzt. Ein dauerhaftes Wohnen ist nicht erlaubt.

§ Laube und Bauten
Eine Gartenlaube in einfacher Ausführung mit höchstens 24 m² Grundfläche einschließlich überdachtem Freisitz ist zulässig. Neue Bauten und Veränderungen bedürfen vorher der schriftlichen Zustimmung des Vorstands.

§ Gemeinschaft
Satzung, Gartenordnung und Beschlüsse der Mitgliederversammlung sind Bestandteil dieses Vertrags. Der Pächter leistet die beschlossene Gemeinschaftsarbeit oder zahlt den beschlossenen Ersatzbetrag.

§ Ende des Pachtverhältnisses
Bei Beendigung wird der Garten in ordnungsgemäßem Zustand übergeben. Für Laube, Anpflanzungen und Einrichtungen findet eine Wertermittlung nach den Richtlinien des Verbands statt. Eine Entschädigung zahlt der Nachfolger, nicht der Verein.

§ Datenschutz
Der Verein speichert die für das Pachtverhältnis nötigen Daten (Name, Anschrift, Kontakt, Zählerstände, Zahlungen) und nutzt sie nur für Verwaltung und Abrechnung.`

func (s *Store) vertragsbestimmungenLocked() string {
	if strings.TrimSpace(s.d.Vertragsbestimmungen) != "" {
		return s.d.Vertragsbestimmungen
	}
	return standardVertragsbestimmungen
}

type pachtvertragDaten struct {
	S      Settings
	P      Paechter
	Beginn string
	Regel  KuendigungsRegel
	Text   string
	Plan   lagePlan
}

func (s *Store) pachtvertragDatenLocked(id, beginn string) (pachtvertragDaten, error) {
	var p *Paechter
	for i := range s.d.Paechter {
		if x := &s.d.Paechter[i]; x.ID == id && !x.Geloescht {
			p = x
		}
	}
	if p == nil {
		return pachtvertragDaten{}, notFound("Pächter nicht gefunden")
	}
	if strings.TrimSpace(p.Gartennr) == "" || p.Gartengroesse <= 0 {
		return pachtvertragDaten{}, bad("Für den Pachtvertrag braucht der Pächter eine Gartennummer und die Gartengröße")
	}
	if beginn == "" {
		// Beginn aus der Gartenhistorie, sonst heute
		for _, e := range s.d.GartenHistorie {
			if e.PaechterID == p.ID && e.Gartennr == p.Gartennr && e.Bis == "" {
				beginn = e.Seit
			}
		}
		if beginn == "" {
			beginn = time.Now().Format("2006-01-02")
		}
	}
	if _, err := parseDate(beginn); err != nil {
		return pachtvertragDaten{}, bad("Das Datum des Pachtbeginns ist ungültig")
	}
	return pachtvertragDaten{S: s.d.Settings, P: *p, Beginn: beginn, Regel: s.d.KuendigungsRegel.mitStandard(), Text: s.vertragsbestimmungenLocked(), Plan: s.lagePlanLocked(false)}, nil
}

func monatTagText(mt string) string {
	t, err := time.Parse("2006-01-02", "2000-"+mt)
	if err != nil {
		return mt
	}
	return fmt.Sprintf("%d. %s", t.Day(), monatsnamen[t.Month()-1])
}

func buildPachtvertrag(d pachtvertragDaten) ([]byte, error) {
	s, p := d.S, d.P
	pdf := neuesPDF(s, "Pachtvertrag Garten "+p.Gartennr, false)
	pdf.SetAutoPageBreak(true, 20)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	mr, mg, mb := markenfarbe(s.Farbe)
	y := 36.0
	titel := func(t string) {
		if y > 255 {
			pdf.AddPage()
			y = 20
		}
		pdf.SetFont("lib", "B", 11)
		pdf.SetTextColor(mr, mg, mb)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(rightX-leftX, 6, t, "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		y += 7
	}
	absatz := func(t string) {
		pdf.SetFont("lib", "", 10)
		pdf.SetXY(leftX, y)
		pdf.MultiCell(rightX-leftX, 5.1, t, "", "L", false)
		y = pdf.GetY() + 2.5
	}
	pdf.SetFont("lib", "B", 16)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 8, "Unterpachtvertrag über einen Kleingarten", "", 0, "C", false, 0, "")
	y += 13
	absatz(fmt.Sprintf("Zwischen dem %s, %s, vertreten durch den Vorstand – im Folgenden »Verein« –\n\nund\n\n%s %s, %s, %s (Mitgliedsnummer %s) – im Folgenden »Pächter« –\n\nwird folgender Vertrag geschlossen:",
		s.VereinName, s.Ort, p.Anrede, p.Name, p.Strasse, p.PLZOrt, p.Mitgliedsnr))
	n := 0
	para := func(t string) string { n++; return fmt.Sprintf("§ %d %s", n, t) }
	titel(para("Pachtgegenstand"))
	absatz(fmt.Sprintf("Der Verein verpachtet dem Pächter den Kleingarten Nr. %s in der Anlage des Vereins mit einer Fläche von %s m². Die Lage ergibt sich aus dem Lageplan in der Anlage.",
		p.Gartennr, fmtFlex(p.Gartengroesse)))
	titel(para("Pachtzeit und Kündigung"))
	absatz(fmt.Sprintf("Das Pachtverhältnis beginnt am %s und läuft auf unbestimmte Zeit. Der Pächter kann zum %s eines Jahres kündigen; die Kündigung muss dem Vorstand bis zum %s schriftlich vorliegen. Für die Kündigung durch den Verein gilt das Bundeskleingartengesetz.",
		germanDate(d.Beginn), monatTagText(d.Regel.Zum), monatTagText(d.Regel.Bis)))
	titel(para("Pacht und weitere Kosten"))
	kosten := fmt.Sprintf("Die Pacht beträgt %s je m² und Jahr, für diesen Garten also %s jährlich. Hinzu kommen der Mitgliedsbeitrag (%s), der Beitrag an den Verband (%s) und die Umlage (%s). "+
		"Wasser und Strom werden nach Zählerstand zu den jeweils beschlossenen Preisen abgerechnet (derzeit %s je m³ Wasser und %s je kWh Strom zuzüglich Grundpreis).",
		fmtEUR(s.PachtJeQm), fmtEUR(round2(s.PachtJeQm*p.Gartengroesse)), fmtEUR(s.Vereinsbeitrag), fmtEUR(s.Territorialverband), fmtEUR(s.UmlageStandard), fmtEUR(s.WasserPreis), fmtEUR(s.EnergiePreis))
	if s.Pflichtstunden > 0 {
		kosten += fmt.Sprintf(" Der Pächter leistet %s Stunden Gemeinschaftsarbeit im Jahr; für jede fehlende Stunde werden %s berechnet.", fmtFlex(s.Pflichtstunden), fmtEUR(s.NachzahlungJeStd))
	}
	kosten += fmt.Sprintf(" Der Verein stellt jährlich eine Rechnung; der Betrag ist bis zum genannten Termin auf das Konto %s (%s) zu zahlen. Änderungen der Beträge beschließt die Mitgliederversammlung.", s.IBAN, s.BankName)
	absatz(kosten)
	// Vereinsbestimmungen: Zeilen mit »§« sind Überschriften
	var block []string
	flush := func() {
		if len(block) > 0 {
			absatz(strings.TrimSpace(strings.Join(block, "\n")))
			block = nil
		}
	}
	for _, z := range strings.Split(strings.ReplaceAll(d.Text, "\r\n", "\n"), "\n") {
		if t := strings.TrimSpace(z); strings.HasPrefix(t, "§") {
			flush()
			titel(para(strings.TrimSpace(strings.TrimLeft(t, "§ 0123456789."))))
			continue
		}
		block = append(block, z)
	}
	flush()
	titel(para("Schlussbestimmungen"))
	absatz("Änderungen und Ergänzungen dieses Vertrags bedürfen der Schriftform. Sollte eine Bestimmung unwirksam sein, bleibt der Vertrag im Übrigen wirksam. Jede Vertragspartei erhält eine Ausfertigung.")

	// Unterschriften
	if y > 235 {
		pdf.AddPage()
		y = 20
	}
	y += 10
	unterschrift := func(x float64, wer string) {
		pdf.SetDrawColor(0, 0, 0)
		pdf.Line(x, y+12, x+75, y+12)
		pdf.SetFont("lib", "", 8.5)
		pdf.SetTextColor(90, 90, 90)
		pdf.SetXY(x, y+13)
		pdf.CellFormat(75, 4, "Ort, Datum, Unterschrift "+wer, "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}
	unterschrift(leftX, "Verein (Vorstand)")
	unterschrift(rightX-75, "Pächter")
	y += 26
	pdf.SetFont("lib", "", 7.5)
	pdf.SetTextColor(110, 110, 110)
	pdf.SetXY(leftX, y)
	pdf.MultiCell(rightX-leftX, 3.8, "Muster aus dem Kleingarten-Manager. Bitte vor der ersten Verwendung mit dem Verband oder einer Rechtsberatung abstimmen, ob es zu Satzung und Zwischenpachtvertrag passt.", "", "L", false)
	pdf.SetTextColor(0, 0, 0)

	// Anlage: Lageplan mit markiertem Garten
	if len(d.Plan.Gaerten) > 0 {
		pdf.AddPage()
		pdf.SetFont("lib", "B", 11)
		pdf.SetTextColor(mr, mg, mb)
		pdf.SetXY(leftX, 20)
		pdf.CellFormat(rightX-leftX, 6, "Anlage: Lageplan – Garten "+p.Gartennr+" ist hervorgehoben", "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetAutoPageBreak(false, 0)
		zeichneLageplan(pdf, s, d.Plan, leftX, 30, rightX, 150, p.Gartennr)
	}
	return pdfBytes(pdf)
}

func (a *App) pachtvertragPDF(r *http.Request) ([]byte, string, error) {
	a.st.mu.Lock()
	d, err := a.st.pachtvertragDatenLocked(r.URL.Query().Get("paechter"), strings.TrimSpace(r.URL.Query().Get("beginn")))
	a.st.mu.Unlock()
	if err != nil {
		return nil, "", err
	}
	out, err := buildPachtvertrag(d)
	return out, "Pachtvertrag_Garten_" + safeName(d.P.Gartennr, 20) + ".pdf", err
}

// handlePachtvertragAblegen legt den Vertrag als Dokument in die Gartenakte (gehört zur Person).
func (a *App) handlePachtvertragAblegen(w http.ResponseWriter, r *http.Request) {
	pdf, _, err := a.pachtvertragPDF(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	d, err := a.st.pachtvertragDatenLocked(r.URL.Query().Get("paechter"), strings.TrimSpace(r.URL.Query().Get("beginn")))
	if err != nil {
		writeErr(w, err)
		return
	}
	doc, err := a.st.dokumentAblegenLocked(d.P.Gartennr, "Pachtvertrag "+d.P.Name, "Pachtvertrag", d.Beginn, d.P.ID, pdf, ".pdf")
	if err != nil {
		writeErr(w, err)
		return
	}
	a.speichernUndAntworten(w, doc)
}

// dokumentAblegenLocked speichert eine vom Programm erzeugte Datei in der Gartenakte. Der Aufrufer hält s.mu
// und speichert danach.
func (s *Store) dokumentAblegenLocked(gartennr, titel, art, datum, paechterID string, data []byte, ext string) (GartenDokument, error) {
	n := 0
	for _, d := range s.d.Dokumente {
		if d.Gartennr == gartennr {
			n++
		}
	}
	if n >= maxDokumenteJeAkte {
		return GartenDokument{}, bad(fmt.Sprintf("Höchstens %d Dokumente je Garten", maxDokumenteJeAkte))
	}
	doc := GartenDokument{ID: newID(), Gartennr: gartennr, Titel: trim(titel, 120), Art: art, Datum: datum, Groesse: len(data), PaechterID: paechterID, Hochgeladen: time.Now().Format(time.RFC3339)}
	dir := filepath.Join(s.dir, gartenakteOrdner)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return GartenDokument{}, err
	}
	name := doc.ID + ext
	if err := schreibeAtomar(filepath.Join(dir, name), data, 0o600); err != nil {
		return GartenDokument{}, err
	}
	doc.Datei = gartenakteOrdner + "/" + name
	s.d.Dokumente = append(s.d.Dokumente, doc)
	s.audit("Gartenakte %s: %s abgelegt", gartennr, art)
	return doc, nil
}

func (a *App) handleVertragsbestimmungen(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.st.mu.Lock()
		defer a.st.mu.Unlock()
		writeJSON(w, 200, map[string]any{"text": a.st.vertragsbestimmungenLocked(), "standard": standardVertragsbestimmungen})
		return
	}
	var in struct {
		Text string `json:"text"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if len([]rune(in.Text)) > 20000 {
		writeErr(w, bad("Der Text ist zu lang"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	a.st.d.Vertragsbestimmungen = strings.TrimSpace(in.Text)
	if a.st.d.Vertragsbestimmungen == strings.TrimSpace(standardVertragsbestimmungen) {
		a.st.d.Vertragsbestimmungen = ""
	}
	a.st.audit("Vertragsbestimmungen für den Pachtvertrag geändert")
	a.speichernUndAntworten(w, map[string]bool{"ok": true})
}
