package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Rechnung per E-Mail vorbereiten: Das Programm verschickt selbst nichts (kein Zugang zum
// Mailkonto, keine Zugangsdaten im Programm). Es öffnet das E-Mail-Programm des Rechners mit
// Empfänger, Betreff und Text und zeigt die PDF-Datei im Ordner an, damit sie nur noch
// angehängt werden muss. Abschicken bleibt eine bewusste Handlung des Kassenwarts.

// openMailFn und revealPathFn sind in Tests ersetzbar, damit kein echtes Programm startet.
var (
	openMailFn   = openBrowser
	revealPathFn = revealPath
)

// mailtoEscape kodiert Betreff und Text für einen mailto-Link (Leerzeichen als %20, Zeilenumbruch als %0D%0A).
func mailtoEscape(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// rechnungsMail liefert Betreff und Text der E-Mail zu einer ausgestellten Rechnung.
func rechnungsMail(r *Rechnung) (betreff, text string) {
	s, p := r.Settings, r.Paechter
	verein := strings.TrimSpace(s.VereinName)
	betreff = fmt.Sprintf("Jahresabrechnung %d – Rechnungs-Nr. %s", r.Jahr, r.Nummer)
	if verein != "" {
		betreff += " – " + verein
	}
	var b strings.Builder
	switch strings.TrimSpace(p.Anrede) {
	case "Herr":
		fmt.Fprintf(&b, "Sehr geehrter Herr %s,\n\n", p.Name)
	case "Frau":
		fmt.Fprintf(&b, "Sehr geehrte Frau %s,\n\n", p.Name)
	default:
		fmt.Fprintf(&b, "Guten Tag %s,\n\n", p.Name)
	}
	fmt.Fprintf(&b, "anbei erhalten Sie Ihre Jahresabrechnung %d (Rechnungs-Nr. %s) als PDF-Datei.\n\n", r.Jahr, r.Nummer)
	switch g := r.Result.Gesamt; {
	case g > 0:
		fmt.Fprintf(&b, "Bitte überweisen Sie den Betrag von %s bis zum %s auf folgendes Konto:\n", fmtEUR(g), germanDate(s.Zahlungsziel))
		if s.BankName != "" {
			fmt.Fprintf(&b, "%s\n", s.BankName)
		}
		fmt.Fprintf(&b, "IBAN: %s\n", s.IBAN)
		if s.BIC != "" {
			fmt.Fprintf(&b, "BIC: %s\n", s.BIC)
		}
		fmt.Fprintf(&b, "Verwendungszweck: %s\n\n", r.Nummer)
	case g < 0:
		fmt.Fprintf(&b, "Es ergibt sich ein Guthaben von %s zu Ihren Gunsten.\n\n", fmtEUR(-g))
	}
	if s.EinspruchTage > 0 {
		fmt.Fprintf(&b, "Bei Fragen oder Unstimmigkeiten melden Sie sich bitte innerhalb von %d Tagen.\n\n", s.EinspruchTage)
	}
	b.WriteString("Mit freundlichen Grüßen\n")
	b.WriteString(verein)
	return betreff, strings.TrimRight(b.String(), "\n")
}

// rechnungsPDFBereit stellt sicher, dass die PDF-Datei einer ausgestellten Rechnung vorhanden ist
// (fehlt sie, wird sie aus den archivierten Werten neu erzeugt), und gibt ihren Pfad zurück.
func (a *App) rechnungsPDFBereit(rec Rechnung) (string, error) {
	if rec.Bereinigt {
		return "", apiError{http.StatusGone, "Die Aufbewahrungsfrist dieser Rechnung ist abgelaufen, der Personenbezug wurde entfernt"}
	}
	full, err := a.rechnungsDatei(rec.Datei)
	if err != nil {
		return "", bad(err.Error())
	}
	fi, statErr := os.Lstat(full)
	switch {
	case statErr == nil && !fi.Mode().IsRegular():
		// z. B. ein Verweis (Symlink): nie hindurchschreiben oder öffnen
		return "", bad("Die gespeicherte Rechnungsdatei ist keine gewöhnliche Datei")
	case statErr != nil && !os.IsNotExist(statErr):
		return "", statErr
	case statErr != nil:
		pdf, err := buildInvoice(rec.Settings, rec.Paechter, rec.Ablesung, rec.Result)
		if err != nil {
			return "", bad(err.Error())
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			return "", err
		}
		if err := os.WriteFile(full, pdf, 0o600); err != nil {
			return "", err
		}
	}
	return full, nil
}

// handleRechnungEmail öffnet das E-Mail-Programm mit einer fertigen Nachricht zur Rechnung und zeigt
// die PDF-Datei im Ordner an. Empfänger ist die aktuelle E-Mail-Adresse aus den Stammdaten (sie kann
// sich seit der Ausstellung geändert haben), ersatzweise die auf der Rechnung gespeicherte.
func (a *App) handleRechnungEmail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	var rec *Rechnung
	for _, x := range a.st.d.Rechnungen {
		if x.ID == id {
			rec = x
			break
		}
	}
	if rec == nil {
		a.st.mu.Unlock()
		writeErr(w, notFound("Rechnung nicht im Archiv gefunden"))
		return
	}
	if rec.Status != statusGueltig {
		a.st.mu.Unlock()
		writeErr(w, bad("Diese Rechnung wurde durch eine neuere ersetzt und sollte nicht mehr verschickt werden"))
		return
	}
	email := rec.Paechter.Email
	for _, p := range a.st.d.Paechter {
		if p.ID == rec.PaechterID && !p.Geloescht {
			email = p.Email
		}
	}
	kopie := *rec
	a.st.mu.Unlock()

	if !emailGueltig(email) {
		writeErr(w, bad("Für "+kopie.Paechter.Name+" ist keine E-Mail-Adresse eingetragen. Bitte unter Admin → Pächter ergänzen"))
		return
	}
	full, err := a.rechnungsPDFBereit(kopie)
	if err != nil {
		writeErr(w, err)
		return
	}
	betreff, text := rechnungsMail(&kopie)
	openMailFn("mailto:" + email + "?subject=" + mailtoEscape(betreff) + "&body=" + mailtoEscape(text))
	revealPathFn(full)

	jetzt := time.Now().Format(time.RFC3339)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for _, x := range a.st.d.Rechnungen {
		if x.ID == id {
			x.EmailAm = jetzt
		}
	}
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"email": email, "datei": filepath.Base(full), "emailAm": jetzt})
}
