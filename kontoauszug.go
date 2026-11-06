package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Kontoauszug einlesen: die CSV-Datei aus dem Online-Banking (Sparkasse, Volksbank, DKB, ING, Postbank, …) wird
// nur im Speicher gelesen und nie gespeichert. Eingänge werden offenen Rechnungen zugeordnet – sicher über die
// Rechnungsnummer im Verwendungszweck, sonst als Vorschlag über Name und Betrag. Übernommen wird erst, was der
// Vorstand in der Vorschau bestätigt. Von jeder übernommenen Buchung bleibt nur ein Fingerabdruck (Hash), damit
// derselbe Auszug nicht zweimal gebucht werden kann.

type kontoBuchung struct {
	Nr          int     `json:"nr"`
	Datum       string  `json:"datum"`
	Name        string  `json:"name"`
	Zweck       string  `json:"zweck"`
	Betrag      float64 `json:"betrag"`
	Fingerprint string  `json:"fingerprint"`
	RechnungID  string  `json:"rechnungId,omitempty"` // Vorschlag
	Status      string  `json:"status"`               // passt | teilbetrag | vorschlag | bezahlt | doppelt | offen
	Hinweis     string  `json:"hinweis,omitempty"`
}

type offeneRechnung struct {
	ID     string  `json:"id"`
	Nummer string  `json:"nummer"`
	Name   string  `json:"name"`
	Jahr   int     `json:"jahr"`
	Offen  float64 `json:"offen"`
}

type kontoVorschau struct {
	Buchungen []kontoBuchung   `json:"buchungen"`
	Offene    []offeneRechnung `json:"offene"`
	Ausgaben  int              `json:"ausgaben"` // Abbuchungen, die übersprungen wurden
}

const (
	maxKontoauszug  = 5 << 20
	maxFingerprints = 20000
)

// parseKontoCSV liest die Buchungen aus einer Bank-CSV. Kopfzeilen vor der Tabelle (Kontonummer, Zeitraum)
// werden übersprungen; Trennzeichen und Spalten werden erkannt.
func parseKontoCSV(data []byte) ([]kontoBuchung, int, error) {
	text := decodeText(data)
	zeilen := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	probe := strings.Join(zeilen[:min(len(zeilen), 40)], "\n")
	delim := ';'
	if strings.Count(probe, "\t") > strings.Count(probe, string(delim)) {
		delim = '\t'
	}
	if strings.Count(probe, ",") > 2*strings.Count(probe, string(delim)) {
		delim = ','
	}
	r := csv.NewReader(strings.NewReader(text))
	r.Comma, r.FieldsPerRecord, r.LazyQuotes = delim, -1, true
	var rows [][]string
	for n := 0; n < 200000; n++ {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // kaputte Zeile überspringen, der Rest zählt
		}
		rows = append(rows, rec)
	}
	norm := func(s string) string {
		return strings.ToLower(strings.Trim(strings.TrimSpace(s), "\"'"))
	}
	kopf, iDatum, iBetrag, iHaben, iName := -1, -1, -1, -1, -1
	var iZweck []int
	for n, row := range rows {
		if n > 40 {
			break
		}
		d, b, h, nm := -1, -1, -1, -1
		dRang := 0 // Buchungstag vor »Datum« vor Valuta/Wertstellung
		var z []int
		for i, c := range row {
			c = norm(c)
			rang := map[string]int{"buchungstag": 3, "buchungsdatum": 3, "buchung": 3, "datum": 2, "valutadatum": 1, "valuta": 1, "wertstellung": 1}[c]
			switch {
			case rang > 0:
				if rang > dRang {
					d, dRang = i, rang
				}
			case strings.HasPrefix(c, "betrag") || c == "umsatz" || strings.HasPrefix(c, "umsatz ") || strings.HasPrefix(c, "umsatz("):
				if !strings.Contains(c, "fremdw") && b < 0 { // »Umsatzart«/»Umsatztyp« sind keine Beträge
					b = i
				}
			case c == "haben" || strings.HasPrefix(c, "haben "):
				h = i
			case strings.Contains(c, "verwendungszweck") || c == "zweck" || c == "buchungsdetails":
				z = append(z, i)
			case strings.Contains(c, "zahlungspflichtig") || strings.Contains(c, "begünstigter") || strings.Contains(c, "beguenstigter") ||
				strings.Contains(c, "auftraggeber") || strings.Contains(c, "zahlungsbeteiligter") || strings.Contains(c, "gegenkonto name") ||
				c == "name" || c == "empfänger" || c == "absender":
				if nm < 0 {
					nm = i
				}
			}
		}
		if d >= 0 && (b >= 0 || h >= 0) {
			kopf, iDatum, iBetrag, iHaben, iName, iZweck = n, d, b, h, nm, z
			break
		}
	}
	if kopf < 0 {
		return nil, 0, bad("In der Datei wurde keine Umsatztabelle gefunden. Bitte im Online-Banking die Umsätze als CSV exportieren (Spalten wie Buchungstag, Betrag, Verwendungszweck)")
	}
	if len(iZweck) == 0 { // ersatzweise Buchungstext
		for i, c := range rows[kopf] {
			if c = norm(c); c == "buchungstext" || c == "beschreibung" || c == "text" {
				iZweck = append(iZweck, i)
			}
		}
	}
	feld := func(row []string, i int) string {
		if i < 0 || i >= len(row) {
			return ""
		}
		return strings.Join(strings.Fields(row[i]), " ")
	}
	var out []kontoBuchung
	ausgaben := 0
	for n, row := range rows[kopf+1:] {
		datum, ok := bankDatum(feld(row, iDatum))
		if !ok {
			continue // Leer-, Summen- oder Saldozeile
		}
		betrag, ok := parseNumber(feld(row, iBetrag), false)
		if iBetrag < 0 || !ok {
			betrag, ok = parseNumber(feld(row, iHaben), false)
		}
		if !ok {
			continue
		}
		if betrag <= 0 {
			ausgaben++
			continue
		}
		var zweck []string
		for _, i := range iZweck {
			if z := feld(row, i); z != "" {
				zweck = append(zweck, z)
			}
		}
		b := kontoBuchung{Nr: n + 1, Datum: datum, Name: trim(feld(row, iName), 120), Zweck: trim(strings.Join(zweck, " "), 400), Betrag: round2(betrag)}
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%.2f|%s|%s", b.Datum, b.Betrag, strings.ToUpper(b.Name), strings.ToUpper(strings.Join(strings.Fields(b.Zweck), "")))))
		b.Fingerprint = hex.EncodeToString(sum[:12])
		out = append(out, b)
		if len(out) >= 5000 {
			break
		}
	}
	return out, ausgaben, nil
}

// bankDatum versteht 02.01.2026, 02.01.26 und 2026-01-02.
func bankDatum(s string) (string, bool) {
	for _, f := range []string{"02.01.2006", "02.01.06", "2006-01-02", "2.1.2006"} {
		if t, err := time.Parse(f, s); err == nil && t.Year() >= 2000 && t.Year() <= 2100 {
			return t.Format("2006-01-02"), true
		}
	}
	return "", false
}

// kompakt macht Text vergleichbar: Großbuchstaben, ohne Leerzeichen (Banken brechen Verwendungszwecke mitten im Wort um).
func kompakt(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// enthaeltNummer prüft, ob die Rechnungsnummer im Text steht und nicht nur Teil einer längeren Nummer ist
// (»2025-1-03« passt nicht in »2025-1-031« oder »12025-1-03«).
func enthaeltNummer(text, nummer string) bool {
	n := kompakt(nummer)
	if len(n) < 3 {
		return false
	}
	for start := 0; ; {
		i := strings.Index(text[start:], n)
		if i < 0 {
			return false
		}
		i += start
		vor, nach := byte(' '), byte(' ')
		if i > 0 {
			vor = text[i-1]
		}
		if e := i + len(n); e < len(text) {
			nach = text[e]
		}
		if !(vor >= '0' && vor <= '9') && vor != '-' && !(nach >= '0' && nach <= '9') {
			return true
		}
		start = i + 1
	}
}

// zuordnenLocked schlägt zu jeder Buchung eine offene Rechnung vor. Der Aufrufer hält s.mu.
func (s *Store) zuordnenLocked(bs []kontoBuchung) []offeneRechnung {
	gesehen := map[string]bool{}
	for _, f := range s.d.KontoFingerprints {
		gesehen[f] = true
	}
	var offen, alle []*Rechnung
	for _, x := range s.d.Rechnungen {
		if x.Status != statusGueltig || x.Bereinigt || x.Result.Gesamt <= 0 {
			continue
		}
		alle = append(alle, x)
		if openAmount(x) > 0.005 {
			offen = append(offen, x)
		}
	}
	sort.SliceStable(offen, func(i, j int) bool {
		if offen[i].Jahr != offen[j].Jahr {
			return offen[i].Jahr < offen[j].Jahr
		}
		return natLess(offen[i].Nummer, offen[j].Nummer)
	})
	vergeben := map[string]float64{} // in dieser Datei schon vorgeschlagene Beträge je Rechnung
	for i := range bs {
		b := &bs[i]
		if gesehen[b.Fingerprint] {
			b.Status, b.Hinweis = "doppelt", "schon aus einem früheren Kontoauszug übernommen"
			continue
		}
		text := kompakt(b.Zweck + " " + b.Name)
		var treffer []*Rechnung
		for _, x := range alle {
			if enthaeltNummer(text, x.Nummer) {
				treffer = append(treffer, x)
			}
		}
		var wahl *Rechnung
		for _, x := range treffer {
			rest := round2(openAmount(x) - vergeben[x.ID])
			if rest > 0.005 && (wahl == nil || absf(rest-b.Betrag) < 0.005) {
				wahl = x
			}
		}
		switch {
		case wahl != nil:
			rest := round2(openAmount(wahl) - vergeben[wahl.ID])
			b.RechnungID = wahl.ID
			if absf(rest-b.Betrag) < 0.005 {
				b.Status = "passt"
			} else {
				b.Status = "teilbetrag"
				b.Hinweis = fmt.Sprintf("Rechnung %s: offen %s, gezahlt %s", wahl.Nummer, fmtEUR(rest), fmtEUR(b.Betrag))
			}
			vergeben[wahl.ID] += b.Betrag
		case len(treffer) > 0:
			b.Status, b.Hinweis = "bezahlt", "Rechnung "+treffer[0].Nummer+" ist schon als bezahlt vermerkt"
		default:
			// ohne Nummer: Name des Pächters im Text und Betrag gleich dem offenen Betrag, eindeutig
			var kandidat *Rechnung
			mehrere := false
			for _, x := range offen {
				name := kompakt(x.Paechter.Name)
				if len([]rune(name)) < 4 || !strings.Contains(text, name) || absf(openAmount(x)-vergeben[x.ID]-b.Betrag) >= 0.005 {
					continue
				}
				if kandidat != nil {
					mehrere = true
				}
				kandidat = x
			}
			if kandidat != nil && !mehrere {
				b.RechnungID, b.Status = kandidat.ID, "vorschlag"
				b.Hinweis = "über Name und Betrag gefunden – bitte prüfen"
				vergeben[kandidat.ID] += b.Betrag
			} else {
				b.Status = "offen"
			}
		}
	}
	out := make([]offeneRechnung, 0, len(offen))
	for _, x := range offen {
		out = append(out, offeneRechnung{ID: x.ID, Nummer: x.Nummer, Name: x.Paechter.Name, Jahr: x.Jahr, Offen: openAmount(x)})
	}
	return out
}

func (a *App) handleKontoauszugVorschau(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxKontoauszug+1<<16)
	if err := r.ParseMultipartForm(maxKontoauszug); err != nil {
		writeErr(w, bad("Die Datei ist zu groß oder konnte nicht gelesen werden (höchstens 5 MB)"))
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, bad("Bitte die CSV-Datei mit den Umsätzen auswählen"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxKontoauszug))
	if err != nil {
		writeErr(w, bad("Die Datei konnte nicht gelesen werden"))
		return
	}
	bs, ausgaben, err := parseKontoCSV(data)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	offene := a.st.zuordnenLocked(bs)
	a.st.mu.Unlock()
	if bs == nil {
		bs = []kontoBuchung{}
	}
	writeJSON(w, 200, kontoVorschau{Buchungen: bs, Offene: offene, Ausgaben: ausgaben})
}

// handleKontoauszugBuchen vermerkt die bestätigten Zahlungen. Erst wird alles geprüft, dann gebucht.
func (a *App) handleKontoauszugBuchen(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Zahlungen []struct {
			RechnungID  string  `json:"rechnungId"`
			Datum       string  `json:"datum"`
			Betrag      float64 `json:"betrag"`
			Fingerprint string  `json:"fingerprint"`
		} `json:"zahlungen"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if len(in.Zahlungen) == 0 {
		writeErr(w, bad("Bitte mindestens eine Zahlung auswählen"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	gesehen := map[string]bool{}
	for _, f := range a.st.d.KontoFingerprints {
		gesehen[f] = true
	}
	rech := map[string]*Rechnung{}
	for _, x := range a.st.d.Rechnungen {
		rech[x.ID] = x
	}
	for _, z := range in.Zahlungen {
		x := rech[z.RechnungID]
		if x == nil || x.Status != statusGueltig || x.Bereinigt {
			writeErr(w, bad("Eine ausgewählte Rechnung gibt es nicht mehr – bitte den Auszug neu einlesen"))
			return
		}
		if _, err := parseDate(z.Datum); err != nil {
			writeErr(w, bad("Ungültiges Buchungsdatum"))
			return
		}
		if !validNum(z.Betrag) || z.Betrag <= 0 || z.Betrag > 1e6 {
			writeErr(w, bad("Ungültiger Betrag"))
			return
		}
		if len(z.Fingerprint) != 24 || strings.Trim(z.Fingerprint, "0123456789abcdef") != "" {
			writeErr(w, bad("Bitte den Auszug neu einlesen"))
			return
		}
		if gesehen[z.Fingerprint] {
			writeErr(w, bad(fmt.Sprintf("Die Zahlung vom %s über %s wurde schon übernommen", germanDate(z.Datum), fmtEUR(z.Betrag))))
			return
		}
		gesehen[z.Fingerprint] = true
	}
	a.st.snapshotBackup("vor-Kontoauszug")
	summe := 0.0
	for _, z := range in.Zahlungen {
		x := rech[z.RechnungID]
		bezahlt := round2(paidAmount(x) + z.Betrag)
		if x.BezahltAm == "" || z.Datum > x.BezahltAm {
			x.BezahltAm = z.Datum
		}
		x.BezahltBetrag = nil
		if bezahlt != round2(absf(x.Result.Gesamt)) {
			x.BezahltBetrag = &bezahlt // nur bei Teil- oder Mehrzahlung speichern, wie bei der Eingabe von Hand
		}
		a.st.d.KontoFingerprints = append(a.st.d.KontoFingerprints, z.Fingerprint)
		summe += z.Betrag
	}
	if n := len(a.st.d.KontoFingerprints); n > maxFingerprints {
		a.st.d.KontoFingerprints = append([]string(nil), a.st.d.KontoFingerprints[n-maxFingerprints:]...)
	}
	a.st.audit("Kontoauszug: %d Zahlung(en) über %s übernommen", len(in.Zahlungen), fmtEUR(summe))
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"gebucht": len(in.Zahlungen), "summe": round2(summe)})
}
