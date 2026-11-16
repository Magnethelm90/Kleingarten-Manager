package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Wartelisten-Briefe: Gartenangebot mit Besichtigung, Nachfrage nach dem Interesse und Eingangsbestätigung als
// PDF mit Briefkopf. Dazu der Nachrücker-Vorschlag: wer ist als Nächstes dran, wenn ein Garten frei wird. Wer
// schon ausdrücklich kein Interesse hatte oder nicht zur Besichtigung erschienen ist, rückt nicht vor.

var wartelistenBriefe = []struct{ Key, Titel string }{
	{"angebot", "Angebot eines Gartens"},
	{"nachfrage", "Nachfrage zum Interesse"},
	{"bestaetigung", "Bestätigung der Anmeldung"},
}

func (s *Store) bewerberNachIDLocked(id string) (Bewerber, bool) {
	for _, b := range s.d.Warteliste {
		if b.ID == id {
			return b, true
		}
	}
	return Bewerber{}, false
}

// nachrueckerLocked: die nächsten Bewerber in der Reihenfolge der Anmeldung, ohne die, die abgesagt haben.
func (s *Store) nachrueckerLocked(n int) []Bewerber {
	l := append([]Bewerber{}, s.d.Warteliste...)
	sortiereWarteliste(l)
	var out []Bewerber
	for _, b := range l {
		if b.BesichtigungErgebnis == "kein Interesse" || b.BesichtigungErgebnis == "nicht erschienen" {
			continue
		}
		out = append(out, b)
		if len(out) == n {
			break
		}
	}
	return out
}

func (a *App) handleNachruecker(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	out := a.st.nachrueckerLocked(3)
	if out == nil {
		out = []Bewerber{}
	}
	writeJSON(w, 200, out)
}

func buildWartelistenBrief(s Settings, b Bewerber, art, garten, datum string) ([]byte, error) {
	p := Paechter{Anrede: b.Anrede, Name: b.Name, Strasse: b.Strasse, PLZOrt: b.PLZOrt, Versand: "Post"}
	pdf := neuesPDF(s, "Warteliste – "+art, false)
	mr, mg, mb := markenfarbe(s.Farbe)
	y := briefAnfang(pdf, s, p, datum)
	titel, text := "", ""
	gruss := "Liebe Gartenfreundin, lieber Gartenfreund"
	if b.Anrede != "" {
		gruss = "Guten Tag"
	}
	switch art {
	case "angebot":
		titel = "Angebot eines Gartens"
		wo := "ein Garten in unserer Anlage"
		if garten != "" {
			wo = "der Garten " + garten + " in unserer Anlage"
		}
		text = fmt.Sprintf("Sie stehen seit dem %s auf unserer Warteliste. Nun wird %s frei, und wir möchten ihn Ihnen anbieten.\n\n", germanDate(b.Seit), wo)
		if b.BesichtigungAm != "" {
			zeit := germanDate(b.BesichtigungAm)
			if b.BesichtigungUm != "" {
				zeit += " um " + b.BesichtigungUm + " Uhr"
			}
			text += fmt.Sprintf("Gern zeigen wir Ihnen den Garten bei einer Besichtigung am %s. ", zeit)
		} else {
			text += "Gern zeigen wir Ihnen den Garten bei einer Besichtigung. Bitte melden Sie sich beim Vorstand, dann vereinbaren wir einen Termin. "
		}
		text += "Dabei besprechen wir auch die Übernahmekosten, die Pachtbedingungen und die Gartenordnung.\n\nBitte teilen Sie uns innerhalb von 14 Tagen mit, ob Sie Interesse haben. Andernfalls bieten wir den Garten dem Nächsten auf der Liste an."
	case "nachfrage":
		titel = "Besteht Ihr Interesse an einem Garten noch?"
		text = fmt.Sprintf("Sie stehen seit dem %s auf der Warteliste unseres Vereins. Damit die Liste aktuell bleibt und wir keine Angaben unnötig aufbewahren, fragen wir regelmäßig nach.\n\nBitte teilen Sie uns kurz mit, ob Sie weiterhin Interesse an einem Garten haben. Das geht formlos per Post, E-Mail oder im Gespräch mit dem Vorstand. "+
			"Hören wir bis zum %s nichts von Ihnen, nehmen wir Sie von der Liste und löschen Ihre Angaben.", germanDate(b.Seit), germanDate(time.Now().AddDate(0, 0, 28).Format("2006-01-02")))
	default:
		titel = "Bestätigung Ihrer Anmeldung"
		text = fmt.Sprintf("wir bestätigen, dass wir Sie am %s auf die Warteliste für einen Kleingarten aufgenommen haben.\n\nSobald ein Garten frei wird, melden wir uns in der Reihenfolge der Anmeldung bei Ihnen. Wie lange das dauert, lässt sich nicht vorhersagen. Bitte teilen Sie uns Änderungen Ihrer Anschrift oder Ihres Wunsches mit. "+
			"Ihre Angaben verwenden wir nur für die Vergabe der Gärten und löschen sie, wenn Sie kein Interesse mehr haben.", germanDate(b.Seit))
	}
	pdf.SetFont("lib", "B", 13)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 7, titel, "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	y += 11
	pdf.SetFont("lib", "", 10)
	pdf.SetXY(leftX, y)
	pdf.MultiCell(rightX-leftX, 5.2, gruss+",\n\n"+text, "", "L", false)
	y = pdf.GetY() + 8
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 6, "Mit freundlichen Gartengrüßen", "", 0, "L", false, 0, "")
	pdf.SetXY(leftX, y+6)
	pdf.CellFormat(rightX-leftX, 6, "Der Vorstand", "", 0, "L", false, 0, "")
	return pdfBytes(pdf)
}

func (a *App) wartelistenBriefPDF(r *http.Request) ([]byte, string, error) {
	art := r.URL.Query().Get("art")
	ok := false
	for _, x := range wartelistenBriefe {
		ok = ok || x.Key == art
	}
	if !ok {
		return nil, "", bad("Unbekannte Briefart")
	}
	a.st.mu.Lock()
	b, gefunden := a.st.bewerberNachIDLocked(r.PathValue("id"))
	s := a.st.d.Settings
	a.st.mu.Unlock()
	if !gefunden {
		return nil, "", notFound("Eintrag nicht gefunden")
	}
	garten := trim(strings.TrimSpace(r.URL.Query().Get("garten")), 20)
	if garten == "" {
		garten = b.BesichtigungGarten
	}
	pdf, err := buildWartelistenBrief(s, b, art, garten, time.Now().Format("2006-01-02"))
	return pdf, safeName("Warteliste_"+art+"_"+b.Name, 80) + ".pdf", err
}

// nachrueckerAufgabeLocked: Wird ein Garten frei (laufende Kündigung ohne Nachfolger-Anfrage), nennt das Cockpit, wer
// als Nächstes dran wäre.
func (s *Store) nachrueckerAufgabeLocked(add func(stufe, titel, text, ziel, knopf string)) {
	offen := 0
	for _, k := range s.d.Kuendigungen {
		if k.Abgeschlossen == "" && k.Erledigt["warteliste"] == "" {
			offen++
		}
	}
	if offen == 0 {
		return
	}
	n := s.nachrueckerLocked(1)
	if len(n) == 0 {
		add("info", "Garten wird frei – die Warteliste ist leer", "Der Garten muss bekannt gemacht werden, z. B. mit dem Exposé für freie Gärten.", "admin:warteliste", "Warteliste")
		return
	}
	add("info", "Garten wird frei: "+n[0].Name+" ist auf der Warteliste als Nächste/r dran", "Gartenangebot als Brief schreiben und zur Besichtigung einladen.", "admin:warteliste", "Warteliste")
}
