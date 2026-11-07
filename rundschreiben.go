package main

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// Rundschreiben: ein Brief des Vorstands an alle Pächter oder an eine Auswahl. Wer per E-Mail erreichbar ist
// (Versand »Emailsendung« und gültige Adresse), bekommt ihn über das eigene E-Mail-Programm (alle Adressen im
// Bcc), alle anderen als Serienbrief zum Drucken – je Pächter eine Seite mit Anschrift im Fensterumschlag.
// Zusätzlich gibt es den Text als Aushang für den Schaukasten. Verschickt wird nichts automatisch.

type Rundschreiben struct {
	ID      string `json:"id"`
	Datum   string `json:"datum"`
	Betreff string `json:"betreff"`
	Text    string `json:"text"`
	// An: »alle« aktiven Pächter oder nur die »auswahl« (IDs in Auswahl)
	An      string   `json:"an"`
	Auswahl []string `json:"auswahl,omitempty"`
	// Verschickt: wann zuletzt Briefe gedruckt oder die E-Mail geöffnet wurden (leer = Entwurf)
	Verschickt string `json:"verschickt,omitempty"`
}

const (
	maxRundschreiben    = 500
	maxRundschreibenLen = 12000
)

type rundschreibenAnsicht struct {
	Rundschreiben
	Empfaenger int `json:"empfaenger"`
	PerPost    int `json:"perPost"`
	PerMail    int `json:"perMail"`
}

// rundschreibenEmpfaengerLocked: aktive Pächter (ohne ausgeschiedene) laut Auswahl, getrennt nach Post und E-Mail.
func (s *Store) rundschreibenEmpfaengerLocked(rs Rundschreiben) (post, mail []Paechter) {
	gewaehlt := map[string]bool{}
	for _, id := range rs.Auswahl {
		gewaehlt[id] = true
	}
	for _, p := range s.d.Paechter {
		if p.Geloescht || p.AusgeschiedenAm != "" || (rs.An == "auswahl" && !gewaehlt[p.ID]) {
			continue
		}
		if p.Versand == "Emailsendung" && emailGueltig(p.Email) {
			mail = append(mail, p)
		} else {
			post = append(post, p)
		}
	}
	nachGarten := func(l []Paechter) {
		sort.SliceStable(l, func(i, j int) bool {
			return naturalLess(strings.TrimSpace(l[i].Gartennr+" "+l[i].Mitgliedsnr), strings.TrimSpace(l[j].Gartennr+" "+l[j].Mitgliedsnr))
		})
	}
	nachGarten(post)
	nachGarten(mail)
	return post, mail
}

func (a *App) handleRundschreibenListe(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	out := []rundschreibenAnsicht{}
	for _, rs := range a.st.d.Rundschreiben {
		post, mail := a.st.rundschreibenEmpfaengerLocked(rs)
		out = append(out, rundschreibenAnsicht{rs, len(post) + len(mail), len(post), len(mail)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Datum > out[j].Datum })
	writeJSON(w, 200, out)
}

func (a *App) handleRundschreibenSpeichern(w http.ResponseWriter, r *http.Request) {
	var in Rundschreiben
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in.Betreff = trim(strings.Join(strings.Fields(in.Betreff), " "), 150)
	in.Text = strings.TrimSpace(strings.ReplaceAll(in.Text, "\r\n", "\n"))
	if in.Betreff == "" || in.Text == "" {
		writeErr(w, bad("Bitte Betreff und Text eintragen"))
		return
	}
	if len([]rune(in.Text)) > maxRundschreibenLen {
		writeErr(w, bad("Der Text ist zu lang (höchstens 12.000 Zeichen)"))
		return
	}
	if in.Datum == "" {
		in.Datum = time.Now().Format("2006-01-02")
	}
	if _, err := parseDate(in.Datum); err != nil {
		writeErr(w, bad("Das Datum ist ungültig"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	bekannt := map[string]bool{}
	for _, p := range a.st.d.Paechter {
		if !p.Geloescht && p.AusgeschiedenAm == "" {
			bekannt[p.ID] = true
		}
	}
	var auswahl []string
	doppelt := map[string]bool{}
	for _, id := range in.Auswahl {
		if bekannt[id] && !doppelt[id] {
			doppelt[id] = true
			auswahl = append(auswahl, id)
		}
	}
	if in.An != "auswahl" {
		in.An, auswahl = "alle", nil
	} else if len(auswahl) == 0 {
		writeErr(w, bad("Bitte mindestens einen Empfänger wählen"))
		return
	}
	in.Auswahl = auswahl
	for i := range a.st.d.Rundschreiben {
		if in.ID != "" && a.st.d.Rundschreiben[i].ID == in.ID {
			in.Verschickt = a.st.d.Rundschreiben[i].Verschickt
			a.st.d.Rundschreiben[i] = in
			a.st.audit("Rundschreiben geändert: %s", in.Betreff)
			a.speichernUndAntworten(w, in)
			return
		}
	}
	if len(a.st.d.Rundschreiben) >= maxRundschreiben {
		writeErr(w, bad("Es sind schon sehr viele Rundschreiben gespeichert. Bitte alte löschen"))
		return
	}
	in.ID, in.Verschickt = newID(), ""
	a.st.d.Rundschreiben = append(a.st.d.Rundschreiben, in)
	a.st.audit("Rundschreiben angelegt: %s", in.Betreff)
	a.speichernUndAntworten(w, in)
}

// speichernUndAntworten speichert (der Aufrufer hält s.mu) und schickt v zurück.
func (a *App) speichernUndAntworten(w http.ResponseWriter, v any) {
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, v)
}

func (a *App) handleRundschreibenLoeschen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i, rs := range a.st.d.Rundschreiben {
		if rs.ID == id {
			a.st.d.Rundschreiben = append(a.st.d.Rundschreiben[:i], a.st.d.Rundschreiben[i+1:]...)
			a.st.audit("Rundschreiben gelöscht: %s", rs.Betreff)
			a.speichernUndAntworten(w, map[string]bool{"ok": true})
			return
		}
	}
	writeErr(w, notFound("Rundschreiben nicht gefunden"))
}

// rundschreibenLocked sucht ein Rundschreiben (Zeiger in den Datenbestand). Der Aufrufer hält s.mu.
func (s *Store) rundschreibenLocked(id string) *Rundschreiben {
	for i := range s.d.Rundschreiben {
		if s.d.Rundschreiben[i].ID == id {
			return &s.d.Rundschreiben[i]
		}
	}
	return nil
}

// briefText schreibt Betreff, Text und Gruß ab Höhe y; lange Texte laufen auf Folgeseiten weiter.
func briefText(pdf *fpdf.Fpdf, s Settings, y float64, betreff, text string) {
	mr, mg, mb := markenfarbe(s.Farbe)
	pdf.SetFont("lib", "B", 12.5)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(leftX, y)
	pdf.MultiCell(rightX-leftX, 6.5, betreff, "", "L", false)
	pdf.SetTextColor(0, 0, 0)
	y = pdf.GetY() + 5
	pdf.SetFont("lib", "", 10.5)
	for _, absatz := range strings.Split(text, "\n") {
		if y > 268 {
			pdf.AddPage()
			y = 20
		}
		pdf.SetXY(leftX, y)
		if strings.TrimSpace(absatz) == "" {
			y += 3.5
			continue
		}
		pdf.MultiCell(rightX-leftX, 5.4, absatz, "", "L", false)
		y = pdf.GetY()
	}
	if y > 262 {
		pdf.AddPage()
		y = 20
	}
	y += 8
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 6, "Mit freundlichen Gartengrüßen", "", 0, "L", false, 0, "")
	pdf.SetXY(leftX, y+6)
	pdf.CellFormat(rightX-leftX, 6, "Der Vorstand", "", 0, "L", false, 0, "")
}

// rundschreibenBriefePDF: Serienbrief für alle ohne E-Mail (mit ?alle=1 auch für die E-Mail-Empfänger).
func (a *App) rundschreibenBriefePDF(r *http.Request) ([]byte, string, error) {
	alle := r.URL.Query().Get("alle") == "1"
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	rs := a.st.rundschreibenLocked(r.PathValue("id"))
	if rs == nil {
		return nil, "", notFound("Rundschreiben nicht gefunden")
	}
	post, mail := a.st.rundschreibenEmpfaengerLocked(*rs)
	if alle {
		post = append(post, mail...)
	}
	if len(post) == 0 {
		return nil, "", bad("Alle Empfänger bekommen das Rundschreiben per E-Mail. Für Briefe an alle bitte »Briefe an alle« wählen")
	}
	s := a.st.d.Settings
	pdf := neuesPDF(s, rs.Betreff, false)
	for _, p := range post {
		y := briefAnfang(pdf, s, p, rs.Datum)
		briefText(pdf, s, y, rs.Betreff, rs.Text)
	}
	out, err := pdfBytes(pdf)
	if err != nil {
		return nil, "", err
	}
	a.rundschreibenVerschicktLocked(rs)
	return out, "Rundschreiben-Briefe.pdf", nil
}

// rundschreibenAushangPDF: der Text ohne Anschrift, für Schaukasten oder Vereinsheim.
func (a *App) rundschreibenAushangPDF(r *http.Request) ([]byte, string, error) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	rs := a.st.rundschreibenLocked(r.PathValue("id"))
	if rs == nil {
		return nil, "", notFound("Rundschreiben nicht gefunden")
	}
	s := a.st.d.Settings
	pdf := neuesPDF(s, rs.Betreff, false)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	pdf.SetFont("lib", "", 10)
	pdf.SetXY(leftX, 34)
	pdf.CellFormat(rightX-leftX, 6, "Aushang · "+germanDate(rs.Datum), "", 0, "R", false, 0, "")
	briefText(pdf, s, 44, rs.Betreff, rs.Text)
	out, err := pdfBytes(pdf)
	return out, "Rundschreiben-Aushang.pdf", err
}

// rundschreibenVerschicktLocked merkt sich, dass das Rundschreiben hinausging (ohne Fehler, falls das
// Speichern scheitert: der Ausdruck ist trotzdem da). Der Aufrufer hält s.mu.
func (a *App) rundschreibenVerschicktLocked(rs *Rundschreiben) {
	heute := time.Now().Format("2006-01-02")
	if rs.Verschickt == heute {
		return
	}
	rs.Verschickt = heute
	a.st.audit("Rundschreiben verschickt: %s", rs.Betreff)
	_ = a.st.saveLocked()
}

// mailPakete verteilt die Adressen so auf mailto-Links, dass jeder unter der Längengrenze bleibt. Passt der
// Text mit genug Platz für Adressen hinein, steht er im Link, sonst nur der Betreff (der Text kommt dann aus
// der Zwischenablage).
func mailPakete(betreff, text string, adressen []string) (pakete []string, mitText bool) {
	basis := "mailto:?subject=" + mailtoEscape(betreff)
	mitBody := basis + "&body=" + mailtoEscape(text)
	if len(mitBody)+400 <= maxMailtoLaenge {
		basis, mitText = mitBody, true
	}
	basis += "&bcc="
	var aktuell []string
	laenge := len(basis)
	for _, adr := range adressen {
		e := url.QueryEscape(adr)
		if len(aktuell) > 0 && laenge+len(e)+3 > maxMailtoLaenge {
			pakete = append(pakete, basis+strings.Join(aktuell, ","))
			aktuell, laenge = nil, len(basis)
		}
		aktuell = append(aktuell, e)
		laenge += len(e) + 3
	}
	if len(aktuell) > 0 {
		pakete = append(pakete, basis+strings.Join(aktuell, ","))
	}
	return pakete, mitText
}

// handleRundschreibenMail öffnet das E-Mail-Programm für Paket n (ab 1) und meldet, wie viele Pakete es gibt.
func (a *App) handleRundschreibenMail(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(r.URL.Query().Get("paket"))
	if n < 1 {
		n = 1
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	rs := a.st.rundschreibenLocked(r.PathValue("id"))
	if rs == nil {
		writeErr(w, notFound("Rundschreiben nicht gefunden"))
		return
	}
	_, mail := a.st.rundschreibenEmpfaengerLocked(*rs)
	if len(mail) == 0 {
		writeErr(w, bad("Keiner der Empfänger bekommt Post per E-Mail"))
		return
	}
	adressen := make([]string, len(mail))
	for i, p := range mail {
		adressen[i] = p.Email
	}
	text := rs.Text + "\n\nMit freundlichen Gartengrüßen\nDer Vorstand\n" + a.st.d.Settings.VereinName
	pakete, mitText := mailPakete(rs.Betreff, text, adressen)
	if n > len(pakete) {
		writeErr(w, bad("Dieses Paket gibt es nicht"))
		return
	}
	openMailFn(pakete[n-1])
	a.rundschreibenVerschicktLocked(rs)
	writeJSON(w, 200, map[string]any{"paket": n, "pakete": len(pakete), "empfaenger": len(mail), "mitText": mitText, "text": text})
}

// rundschreibenPersonEntfernen nimmt einen Pächter aus allen Empfängerlisten.
func (d *Data) rundschreibenPersonEntfernen(id string) bool {
	changed := false
	for i := range d.Rundschreiben {
		rs := &d.Rundschreiben[i]
		for j := 0; j < len(rs.Auswahl); j++ {
			if rs.Auswahl[j] == id {
				rs.Auswahl = append(rs.Auswahl[:j], rs.Auswahl[j+1:]...)
				j--
				changed = true
			}
		}
	}
	return changed
}

// rundschreibenFuerLocked: Betreffe der Rundschreiben, in deren Auswahl die Person ausdrücklich steht (Auskunft).
func (s *Store) rundschreibenFuerLocked(id string) []string {
	var out []string
	for _, rs := range s.d.Rundschreiben {
		for _, x := range rs.Auswahl {
			if x == id {
				out = append(out, fmt.Sprintf("%s (%s)", rs.Betreff, germanDate(rs.Datum)))
			}
		}
	}
	return out
}
