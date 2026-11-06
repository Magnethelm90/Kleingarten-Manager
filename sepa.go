package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SEPA-Lastschrift: monatliche Vorauszahlungen und der Einzug der Jahresrechnung. Je Pächter gibt es ein Mandat
// (IBAN, Mandatsreferenz, Datum der Unterschrift, Monatsbetrag). Ein Lastschrift-Lauf erzeugt die Datei für das
// Online-Banking (pain.008.001.08, SEPA-Basislastschrift) und bucht gleich: Vorauszahlungen als Abschlag im
// Abrechnungsjahr (die Jahresrechnung zieht sie ab), Rechnungen als bezahlt. Eine Rücklastschrift nimmt die
// Buchung zurück. Der Verein braucht eine Gläubiger-Identifikationsnummer der Bundesbank.

type SepaEinstellungen struct {
	Aktiv           bool   `json:"aktiv"`           // Bereich »Lastschrift« anzeigen (nur Vereine, die Lastschrift nutzen)
	GlaeubigerID    string `json:"glaeubigerId"`    // z. B. DE98ZZZ09999999999
	Faelligkeitstag int    `json:"faelligkeitstag"` // Tag im Monat für Vorauszahlungen (1–28)
}

// Mandat ist ein SEPA-Lastschriftmandat (wiederkehrend, Basislastschrift).
type Mandat struct {
	ID             string  `json:"id"`
	PaechterID     string  `json:"paechterId"`
	Referenz       string  `json:"referenz"`
	Kontoinhaber   string  `json:"kontoinhaber"`
	IBAN           string  `json:"iban"`
	BIC            string  `json:"bic,omitempty"`
	Unterschrieben string  `json:"unterschrieben"`           // JJJJ-MM-TT
	Monatsbetrag   float64 `json:"monatsbetrag"`             // Vorauszahlung je Monat, 0 = keine
	Rechnungen     bool    `json:"rechnungen"`               // Jahresrechnung (Restbetrag) einziehen
	Widerrufen     string  `json:"widerrufen,omitempty"`     // JJJJ-MM-TT
	ZuletztGenutzt string  `json:"zuletztGenutzt,omitempty"` // Fälligkeit des letzten Einzugs
}

type LastschriftPosten struct {
	MandatID       string  `json:"mandatId"`
	PaechterID     string  `json:"paechterId"`
	Referenz       string  `json:"referenz"`
	Unterschrieben string  `json:"unterschrieben"`
	Name           string  `json:"name"`
	IBAN           string  `json:"iban"`
	BIC            string  `json:"bic,omitempty"`
	Betrag         float64 `json:"betrag"`
	Zweck          string  `json:"zweck"`
	EndToEnd       string  `json:"endToEnd"`
	RechnungID     string  `json:"rechnungId,omitempty"`
	Zurueck        string  `json:"zurueck,omitempty"` // Datum der Rücklastschrift
}

type LastschriftLauf struct {
	ID        string              `json:"id"`
	Art       string              `json:"art"` // vorauszahlung | rechnungen
	Jahr      int                 `json:"jahr"`
	Faellig   string              `json:"faellig"`
	Erstellt  string              `json:"erstellt"`
	MsgID     string              `json:"msgId"`
	Posten    []LastschriftPosten `json:"posten"`
	Bereinigt bool                `json:"bereinigt,omitempty"`
}

const (
	mandatVerfallMonate = 36 // ungenutzte Mandate verfallen nach 36 Monaten
	maxMandate          = 3000
	maxLaeufe           = 2000
	lastschriftOrdner   = "Lastschriften"
)

// glaeubigerIDGueltig prüft Aufbau und Prüfziffer (ISO 7064 Mod 97-10, die Geschäftsbereichskennung zählt nicht mit).
func glaeubigerIDGueltig(id string) bool {
	id = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(id), " ", ""))
	if len(id) < 9 || len(id) > 35 || id[0] < 'A' || id[0] > 'Z' || id[1] < 'A' || id[1] > 'Z' || id[2] < '0' || id[2] > '9' || id[3] < '0' || id[3] > '9' {
		return false
	}
	if id[:2] == "DE" && len(id) != 18 {
		return false
	}
	rest := 0
	for _, c := range id[7:] + id[:4] {
		switch {
		case c >= '0' && c <= '9':
			rest = (rest*10 + int(c-'0')) % 97
		case c >= 'A' && c <= 'Z':
			rest = (rest*100 + int(c-'A') + 10) % 97
		default:
			return false
		}
	}
	return rest == 1
}

// sepaText macht Text SEPA-tauglich: Umlaute umschreiben, nur erlaubte Zeichen, höchstens max Zeichen.
func sepaText(s string, max int) string {
	s = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "Ä", "Ae", "Ö", "Oe", "Ü", "Ue", "ß", "ss", "&", "+").Replace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', strings.ContainsRune("/-?:().,'+ ", r):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	out := strings.Join(strings.Fields(b.String()), " ")
	if len(out) > max {
		out = strings.TrimSpace(out[:max])
	}
	return out
}

// sepaID: für Referenzen und IDs ohne Leerzeichen (Mandatsreferenz, EndToEndId, MsgId).
func sepaID(s string, max int) string {
	t := strings.ReplaceAll(sepaText(s, 1000), " ", "")
	if len(t) > max {
		t = t[:max]
	}
	return t
}

func normIBAN(s string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
}

func bicGueltig(s string) bool {
	if s == "" {
		return true
	}
	if len(s) != 8 && len(s) != 11 {
		return false
	}
	for i, c := range s {
		ok := (c >= 'A' && c <= 'Z') || (i >= 6 && c >= '0' && c <= '9')
		if !ok {
			return false
		}
	}
	return true
}

func (m Mandat) verfallen(heute time.Time) bool {
	basis := m.ZuletztGenutzt
	if basis == "" {
		basis = m.Unterschrieben
	}
	d, err := parseDate(basis)
	return err == nil && d.AddDate(0, mandatVerfallMonate, 0).Before(heute)
}

func (m Mandat) aktiv(heute time.Time) bool { return m.Widerrufen == "" && !m.verfallen(heute) }

func cleanMandat(m Mandat) (Mandat, error) {
	m.IBAN, m.BIC = normIBAN(m.IBAN), strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(m.BIC), " ", ""))
	m.Kontoinhaber = sepaText(m.Kontoinhaber, 70)
	m.Referenz = sepaID(m.Referenz, 35)
	if !ibanGueltig(m.IBAN) {
		return m, bad("Die IBAN ist ungültig – bitte genau abschreiben (die Prüfziffer passt nicht)")
	}
	if !bicGueltig(m.BIC) {
		return m, bad("Die BIC ist ungültig (8 oder 11 Zeichen) – sie kann bei deutschen Konten auch leer bleiben")
	}
	if m.Kontoinhaber == "" {
		return m, bad("Bitte den Kontoinhaber eintragen")
	}
	if d, err := parseDate(m.Unterschrieben); err != nil || d.Year() < 2000 || d.After(time.Now().AddDate(0, 0, 1)) {
		return m, bad("Bitte das Datum der Unterschrift eintragen (nicht in der Zukunft)")
	}
	if !validNum(m.Monatsbetrag) || m.Monatsbetrag < 0 || m.Monatsbetrag > 10000 {
		return m, bad("Der Monatsbetrag muss zwischen 0 und 10.000 € liegen")
	}
	m.Monatsbetrag = round2(m.Monatsbetrag)
	if m.Widerrufen != "" {
		if _, err := parseDate(m.Widerrufen); err != nil {
			m.Widerrufen = ""
		}
	}
	return m, nil
}

// ------------------------------------------------------------------ Ansicht und Mandate

type mandatAnsicht struct {
	Mandat
	Name      string `json:"name"`
	Gartennr  string `json:"gartennr"`
	Verfallen bool   `json:"verfallen"`
}

type laufAnsicht struct {
	LastschriftLauf
	Summe   float64 `json:"summe"`
	Zurueck int     `json:"zurueckAnzahl"`
}

type sepaAnsicht struct {
	Einstellungen SepaEinstellungen `json:"einstellungen"`
	Mandate       []mandatAnsicht   `json:"mandate"`
	Laeufe        []laufAnsicht     `json:"laeufe"`
	Hinweis       string            `json:"hinweis,omitempty"`
}

func (s *Store) sepaHinweisLocked() string {
	switch {
	case !glaeubigerIDGueltig(s.d.Sepa.GlaeubigerID):
		return "Bitte zuerst die Gläubiger-Identifikationsnummer des Vereins eintragen (beantragt bei der Deutschen Bundesbank)."
	case !ibanGueltig(s.d.Settings.IBAN):
		return "Unter »Preise & Einstellungen« fehlt eine gültige IBAN des Vereinskontos."
	}
	return ""
}

func (a *App) handleSepaAnsicht(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	heute := time.Now()
	p := map[string]Paechter{}
	for _, x := range a.st.d.Paechter {
		p[x.ID] = x
	}
	out := sepaAnsicht{Einstellungen: a.st.d.Sepa, Mandate: []mandatAnsicht{}, Laeufe: []laufAnsicht{}, Hinweis: a.st.sepaHinweisLocked()}
	for _, m := range a.st.d.Mandate {
		out.Mandate = append(out.Mandate, mandatAnsicht{Mandat: m, Name: p[m.PaechterID].Name, Gartennr: p[m.PaechterID].Gartennr, Verfallen: m.verfallen(heute)})
	}
	sort.SliceStable(out.Mandate, func(i, j int) bool { return naturalLess(out.Mandate[i].Gartennr, out.Mandate[j].Gartennr) })
	for _, l := range a.st.d.Lastschriften {
		v := laufAnsicht{LastschriftLauf: l}
		for _, x := range l.Posten {
			if x.Zurueck == "" {
				v.Summe += x.Betrag
			} else {
				v.Zurueck++
			}
		}
		v.Summe = round2(v.Summe)
		out.Laeufe = append(out.Laeufe, v)
	}
	sort.SliceStable(out.Laeufe, func(i, j int) bool { return out.Laeufe[i].Faellig > out.Laeufe[j].Faellig })
	writeJSON(w, 200, out)
}

func (a *App) handleSepaEinstellungen(w http.ResponseWriter, r *http.Request) {
	var in SepaEinstellungen
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in.GlaeubigerID = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(in.GlaeubigerID), " ", ""))
	if in.GlaeubigerID != "" && !glaeubigerIDGueltig(in.GlaeubigerID) {
		writeErr(w, bad("Die Gläubiger-Identifikationsnummer ist ungültig (Aufbau wie DE98ZZZ09999999999)"))
		return
	}
	if in.Faelligkeitstag < 1 || in.Faelligkeitstag > 28 {
		in.Faelligkeitstag = 1
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	a.st.d.Sepa = in
	a.st.audit("Lastschrift-Einstellungen geändert")
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, in)
}

func (s *Store) mandatIndexLocked(id string) int {
	for i, m := range s.d.Mandate {
		if m.ID == id {
			return i
		}
	}
	return -1
}

// neueMandatsreferenz: »M« + Mitgliedsnummer + laufende Nummer, eindeutig.
func (s *Store) neueMandatsreferenzLocked(p Paechter) string {
	vergeben := map[string]bool{}
	for _, m := range s.d.Mandate {
		vergeben[strings.ToUpper(m.Referenz)] = true
	}
	basis := sepaID("M"+p.Mitgliedsnr, 30)
	for n := 1; ; n++ {
		ref := fmt.Sprintf("%s-%02d", basis, n)
		if !vergeben[strings.ToUpper(ref)] {
			return ref
		}
	}
}

func (a *App) handleMandatSpeichern(neu bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in Mandat
		if err := readJSON(w, r, &in); err != nil {
			writeErr(w, err)
			return
		}
		a.st.mu.Lock()
		defer a.st.mu.Unlock()
		var p *Paechter
		for i := range a.st.d.Paechter {
			if a.st.d.Paechter[i].ID == in.PaechterID && !a.st.d.Paechter[i].Geloescht {
				p = &a.st.d.Paechter[i]
			}
		}
		if p == nil {
			writeErr(w, bad("Bitte einen Pächter auswählen"))
			return
		}
		if in.Kontoinhaber == "" {
			in.Kontoinhaber = p.Name
		}
		if strings.TrimSpace(in.Referenz) == "" {
			in.Referenz = a.st.neueMandatsreferenzLocked(*p)
		}
		in, err := cleanMandat(in)
		if err != nil {
			writeErr(w, err)
			return
		}
		idx := -1
		if !neu {
			if idx = a.st.mandatIndexLocked(r.PathValue("id")); idx < 0 {
				writeErr(w, notFound("Mandat nicht gefunden"))
				return
			}
		}
		for i, m := range a.st.d.Mandate {
			if i != idx && strings.EqualFold(m.Referenz, in.Referenz) {
				writeErr(w, bad("Diese Mandatsreferenz ist schon vergeben – jede Referenz darf nur einmal vorkommen"))
				return
			}
			if i != idx && m.PaechterID == in.PaechterID && m.Widerrufen == "" && in.Widerrufen == "" {
				writeErr(w, bad("Für diesen Pächter gibt es schon ein gültiges Mandat. Bitte das alte erst widerrufen"))
				return
			}
		}
		if neu {
			if len(a.st.d.Mandate) >= maxMandate {
				writeErr(w, bad("Es sind sehr viele Mandate gespeichert"))
				return
			}
			in.ID, in.ZuletztGenutzt = newID(), ""
			a.st.d.Mandate = append(a.st.d.Mandate, in)
			a.st.audit("Lastschriftmandat angelegt: %s", in.Referenz)
		} else {
			alt := a.st.d.Mandate[idx]
			if alt.ZuletztGenutzt != "" && (alt.Referenz != in.Referenz || alt.Unterschrieben != in.Unterschrieben) {
				writeErr(w, bad("Mandatsreferenz und Unterschriftsdatum eines schon genutzten Mandats lassen sich nicht ändern – bei neuem Konto bitte ein neues Mandat anlegen"))
				return
			}
			in.ID, in.ZuletztGenutzt = alt.ID, alt.ZuletztGenutzt
			a.st.d.Mandate[idx] = in
			a.st.audit("Lastschriftmandat geändert: %s", in.Referenz)
		}
		if err := a.st.saveLocked(); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 200, in)
	}
}

func (a *App) handleMandatLoeschen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	i := a.st.mandatIndexLocked(r.PathValue("id"))
	if i < 0 {
		writeErr(w, notFound("Mandat nicht gefunden"))
		return
	}
	m := a.st.d.Mandate[i]
	if m.ZuletztGenutzt != "" {
		writeErr(w, bad("Mit diesem Mandat wurde schon eingezogen. Bitte statt Löschen »Widerrufen« verwenden"))
		return
	}
	a.st.d.Mandate = append(a.st.d.Mandate[:i], a.st.d.Mandate[i+1:]...)
	a.st.audit("Lastschriftmandat gelöscht: %s", m.Referenz)
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// ------------------------------------------------------------------ Lastschrift-Lauf

type laufVorschau struct {
	Art      string              `json:"art"`
	Jahr     int                 `json:"jahr"`
	Faellig  string              `json:"faellig"`
	Posten   []LastschriftPosten `json:"posten"`
	Summe    float64             `json:"summe"`
	Hinweise []string            `json:"hinweise"`
}

var monatsnamenKurz = []string{"Jan", "Feb", "Mär", "Apr", "Mai", "Jun", "Jul", "Aug", "Sep", "Okt", "Nov", "Dez"}

// laufPlanenLocked stellt die Posten eines Laufs zusammen, ohne etwas zu ändern. Der Aufrufer hält s.mu.
func (s *Store) laufPlanenLocked(art, faellig string, heute time.Time) (laufVorschau, error) {
	v := laufVorschau{Art: art, Faellig: faellig, Jahr: s.d.Settings.Jahr, Posten: []LastschriftPosten{}, Hinweise: []string{}}
	if h := s.sepaHinweisLocked(); h != "" {
		return v, bad(h)
	}
	f, err := parseDate(faellig)
	if err != nil {
		return v, bad("Bitte das Fälligkeitsdatum eintragen")
	}
	morgen := time.Date(heute.Year(), heute.Month(), heute.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	if f.Before(morgen) || f.After(morgen.AddDate(1, 0, 0)) {
		return v, bad("Das Fälligkeitsdatum muss in der Zukunft liegen (frühestens morgen; die Bank braucht die Datei rechtzeitig vorher)")
	}
	if wd := f.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return v, bad("Das Fälligkeitsdatum fällt auf ein Wochenende – bitte einen Bankarbeitstag wählen")
	}
	j := s.d.Jahre[yearKey(v.Jahr)]
	if j == nil || j.Abgeschlossen {
		return v, bad("Das Abrechnungsjahr ist abgeschlossen")
	}
	personen := map[string]Paechter{}
	for _, p := range s.d.Paechter {
		if !p.Geloescht {
			personen[p.ID] = p
		}
	}
	mandat := map[string]Mandat{}
	for _, m := range s.d.Mandate {
		if m.aktiv(heute) {
			mandat[m.PaechterID] = m
		}
	}
	posten := func(m Mandat, betrag float64, zweck, rechnung string) LastschriftPosten {
		return LastschriftPosten{MandatID: m.ID, PaechterID: m.PaechterID, Referenz: m.Referenz, Unterschrieben: m.Unterschrieben, Name: m.Kontoinhaber,
			IBAN: m.IBAN, BIC: m.BIC, Betrag: round2(betrag), Zweck: sepaText(zweck, 140), RechnungID: rechnung}
	}
	switch art {
	case "vorauszahlung":
		monat := faellig[:7]
		for _, l := range s.d.Lastschriften {
			if l.Art == "vorauszahlung" && strings.HasPrefix(l.Faellig, monat) {
				return v, bad(fmt.Sprintf("Für %s %d gibt es schon einen Lauf mit Vorauszahlungen", monatsnamen[f.Month()-1], f.Year()))
			}
		}
		ausgestellt := map[string]bool{}
		for _, r := range s.d.Rechnungen {
			if r.Jahr == v.Jahr && r.Status == statusGueltig {
				ausgestellt[r.PaechterID] = true
			}
		}
		ids := make([]string, 0, len(mandat))
		for id := range mandat {
			ids = append(ids, id)
		}
		sort.SliceStable(ids, func(i, k int) bool { return naturalLess(personen[ids[i]].Gartennr, personen[ids[k]].Gartennr) })
		for _, id := range ids {
			m, p := mandat[id], personen[id]
			if m.Monatsbetrag <= 0 {
				continue
			}
			switch {
			case p.ID == "" || p.AusgeschiedenAm != "":
				v.Hinweise = append(v.Hinweise, fmt.Sprintf("%s: Pächter ausgeschieden – Mandat bitte widerrufen", m.Kontoinhaber))
			case ausgestellt[id]:
				v.Hinweise = append(v.Hinweise, fmt.Sprintf("Garten %s (%s): Jahresrechnung %d ist schon ausgestellt – Vorauszahlung erst wieder nach dem Jahreswechsel", p.Gartennr, p.Name, v.Jahr))
			default:
				v.Posten = append(v.Posten, posten(m, m.Monatsbetrag, fmt.Sprintf("Vorauszahlung %s %d Garten %s Mitglied %s", monatsnamenKurz[f.Month()-1], f.Year(), p.Gartennr, p.Mitgliedsnr), ""))
			}
		}
	case "rechnungen":
		offenImLauf := map[string]bool{}
		for _, l := range s.d.Lastschriften {
			for _, x := range l.Posten {
				if x.RechnungID != "" && x.Zurueck == "" {
					offenImLauf[x.RechnungID] = true
				}
			}
		}
		var rs []*Rechnung
		for _, r := range s.d.Rechnungen {
			if r.Status == statusGueltig && !r.Bereinigt && r.Result.Gesamt > 0 && openAmount(r) > 0.005 {
				rs = append(rs, r)
			}
		}
		sort.SliceStable(rs, func(i, k int) bool { return natLess(rs[i].Nummer, rs[k].Nummer) })
		for _, r := range rs {
			m, ok := mandat[r.PaechterID]
			switch {
			case !ok || !m.Rechnungen:
				continue
			case offenImLauf[r.ID]:
				v.Hinweise = append(v.Hinweise, fmt.Sprintf("Rechnung %s ist schon in einem Lastschrift-Lauf", r.Nummer))
			default:
				v.Posten = append(v.Posten, posten(m, openAmount(r), fmt.Sprintf("Rechnung %s Jahresabrechnung %d", r.Nummer, r.Jahr), r.ID))
			}
		}
	default:
		return v, bad("Unbekannte Art des Laufs")
	}
	for _, x := range v.Posten {
		v.Summe += x.Betrag
	}
	v.Summe = round2(v.Summe)
	return v, nil
}

func (a *App) handleLaufVorschau(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	v, err := a.st.laufPlanenLocked(r.URL.Query().Get("art"), r.URL.Query().Get("faellig"), time.Now())
	a.st.mu.Unlock()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, v)
}

func zufallsID(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return strings.ToUpper(hex.EncodeToString(b))
}

// handleLaufErstellen legt den Lauf an und bucht: Vorauszahlungen als Abschlag, Rechnungen als bezahlt.
func (a *App) handleLaufErstellen(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Art     string `json:"art"`
		Faellig string `json:"faellig"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	heute := time.Now()
	v, err := a.st.laufPlanenLocked(in.Art, in.Faellig, heute)
	if err != nil {
		writeErr(w, err)
		return
	}
	if len(v.Posten) == 0 {
		writeErr(w, bad("Es gibt nichts einzuziehen"))
		return
	}
	if len(a.st.d.Lastschriften) >= maxLaeufe {
		writeErr(w, bad("Es sind sehr viele Lastschrift-Läufe gespeichert"))
		return
	}
	a.st.snapshotBackup("vor-Lastschrift")
	lauf := LastschriftLauf{ID: newID(), Art: v.Art, Jahr: v.Jahr, Faellig: v.Faellig, Erstellt: heute.Format(time.RFC3339), MsgID: "KGM-" + heute.Format("20060102150405") + "-" + zufallsID(4)}
	kurz := zufallsID(4)
	for i, x := range v.Posten {
		x.EndToEnd = fmt.Sprintf("KGM-%s-%03d", kurz, i+1)
		lauf.Posten = append(lauf.Posten, x)
		a.st.bucheLastschriftLocked(lauf, x, +1)
		if k := a.st.mandatIndexLocked(x.MandatID); k >= 0 {
			a.st.d.Mandate[k].ZuletztGenutzt = v.Faellig
		}
	}
	a.st.d.Lastschriften = append(a.st.d.Lastschriften, lauf)
	a.st.audit("Lastschrift-Lauf (%s) zum %s: %d Posten über %s", lauf.Art, germanDate(lauf.Faellig), len(lauf.Posten), fmtEUR(v.Summe))
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, lauf)
}

// bucheLastschriftLocked bucht einen Posten (richtung +1) oder nimmt ihn zurück (−1).
func (s *Store) bucheLastschriftLocked(l LastschriftLauf, x LastschriftPosten, richtung float64) {
	betrag := round2(x.Betrag * richtung)
	if x.RechnungID != "" {
		for _, r := range s.d.Rechnungen {
			if r.ID != x.RechnungID {
				continue
			}
			bezahlt := round2(paidAmount(r) + betrag)
			r.BezahltBetrag = nil
			switch {
			case bezahlt <= 0.005:
				r.BezahltAm = ""
			default:
				if r.BezahltAm == "" || (richtung > 0 && l.Faellig > r.BezahltAm) {
					r.BezahltAm = l.Faellig
				}
				if bezahlt != round2(absf(r.Result.Gesamt)) {
					r.BezahltBetrag = &bezahlt
				}
			}
		}
		return
	}
	if j := s.d.Jahre[yearKey(l.Jahr)]; j != nil && !j.Abgeschlossen {
		if j.Ablesungen == nil {
			j.Ablesungen = map[string]Ablesung{}
		}
		ab := j.Ablesungen[x.PaechterID]
		ab.Abschlag = round2(ab.Abschlag + betrag)
		if ab.Abschlag < 0 {
			ab.Abschlag = 0
		}
		j.Ablesungen[x.PaechterID] = ab
	}
}

func (s *Store) laufIndexLocked(id string) int {
	for i, l := range s.d.Lastschriften {
		if l.ID == id {
			return i
		}
	}
	return -1
}

// handleRuecklastschrift nimmt einen Posten zurück (Rücklastschrift oder Widerspruch des Pächters).
func (a *App) handleRuecklastschrift(w http.ResponseWriter, r *http.Request) {
	var in struct {
		EndToEnd string `json:"endToEnd"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	i := a.st.laufIndexLocked(r.PathValue("id"))
	if i < 0 {
		writeErr(w, notFound("Lastschrift-Lauf nicht gefunden"))
		return
	}
	l := &a.st.d.Lastschriften[i]
	if j := a.st.d.Jahre[yearKey(l.Jahr)]; l.Art == "vorauszahlung" && (j == nil || j.Abgeschlossen) {
		writeErr(w, bad("Das Abrechnungsjahr dieses Laufs ist abgeschlossen – die Rücklastschrift bitte von Hand verbuchen"))
		return
	}
	for k := range l.Posten {
		x := &l.Posten[k]
		if x.EndToEnd != in.EndToEnd {
			continue
		}
		if x.Zurueck != "" {
			writeErr(w, bad("Diese Lastschrift ist schon als zurückgegangen vermerkt"))
			return
		}
		a.st.bucheLastschriftLocked(*l, *x, -1)
		x.Zurueck = time.Now().Format("2006-01-02")
		a.st.audit("Rücklastschrift: %s über %s", x.Referenz, fmtEUR(x.Betrag))
		if err := a.st.saveLocked(); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 200, x)
		return
	}
	writeErr(w, notFound("Posten nicht gefunden"))
}

// ------------------------------------------------------------------ SEPA-XML (pain.008.001.08)

func buildPain008(s Settings, glaeubiger string, l LastschriftLauf, jetzt time.Time) ([]byte, error) {
	var aktiv []LastschriftPosten
	summe := 0.0
	for _, x := range l.Posten {
		if x.Zurueck == "" {
			aktiv = append(aktiv, x)
			summe += x.Betrag
		}
	}
	if len(aktiv) == 0 {
		return nil, bad("In diesem Lauf ist nichts mehr einzuziehen")
	}
	betrag := func(v float64) string { return strconv.FormatFloat(round2(v), 'f', 2, 64) }
	agent := func(bic string) string {
		if bic == "" {
			return "<FinInstnId><Othr><Id>NOTPROVIDED</Id></Othr></FinInstnId>"
		}
		return "<FinInstnId><BICFI>" + xmlText(bic) + "</BICFI></FinInstnId>"
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pain.008.001.08" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` + "\n")
	b.WriteString("<CstmrDrctDbtInitn>\n<GrpHdr>")
	fmt.Fprintf(&b, "<MsgId>%s</MsgId><CreDtTm>%s</CreDtTm><NbOfTxs>%d</NbOfTxs><CtrlSum>%s</CtrlSum><InitgPty><Nm>%s</Nm></InitgPty>",
		xmlText(sepaID(l.MsgID, 35)), jetzt.Format("2006-01-02T15:04:05"), len(aktiv), betrag(summe), xmlText(sepaText(s.VereinName, 70)))
	b.WriteString("</GrpHdr>\n<PmtInf>")
	fmt.Fprintf(&b, "<PmtInfId>%s</PmtInfId><PmtMtd>DD</PmtMtd><BtchBookg>true</BtchBookg><NbOfTxs>%d</NbOfTxs><CtrlSum>%s</CtrlSum>",
		xmlText(sepaID(l.MsgID+"-1", 35)), len(aktiv), betrag(summe))
	b.WriteString("<PmtTpInf><SvcLvl><Cd>SEPA</Cd></SvcLvl><LclInstrm><Cd>CORE</Cd></LclInstrm><SeqTp>RCUR</SeqTp></PmtTpInf>")
	fmt.Fprintf(&b, "<ReqdColltnDt>%s</ReqdColltnDt><Cdtr><Nm>%s</Nm></Cdtr><CdtrAcct><Id><IBAN>%s</IBAN></Id></CdtrAcct><CdtrAgt>%s</CdtrAgt><ChrgBr>SLEV</ChrgBr>",
		xmlText(l.Faellig), xmlText(sepaText(s.VereinName, 70)), xmlText(normIBAN(s.IBAN)), agent(strings.ToUpper(strings.ReplaceAll(s.BIC, " ", ""))))
	fmt.Fprintf(&b, "<CdtrSchmeId><Id><PrvtId><Othr><Id>%s</Id><SchmeNm><Prtry>SEPA</Prtry></SchmeNm></Othr></PrvtId></Id></CdtrSchmeId>\n", xmlText(glaeubiger))
	for _, x := range aktiv {
		b.WriteString("<DrctDbtTxInf>")
		fmt.Fprintf(&b, "<PmtId><EndToEndId>%s</EndToEndId></PmtId><InstdAmt Ccy=\"EUR\">%s</InstdAmt>", xmlText(x.EndToEnd), betrag(x.Betrag))
		fmt.Fprintf(&b, "<DrctDbtTx><MndtRltdInf><MndtId>%s</MndtId><DtOfSgntr>%s</DtOfSgntr></MndtRltdInf></DrctDbtTx>", xmlText(x.Referenz), xmlText(x.Unterschrieben))
		fmt.Fprintf(&b, "<DbtrAgt>%s</DbtrAgt><Dbtr><Nm>%s</Nm></Dbtr><DbtrAcct><Id><IBAN>%s</IBAN></Id></DbtrAcct><RmtInf><Ustrd>%s</Ustrd></RmtInf>",
			agent(x.BIC), xmlText(x.Name), xmlText(x.IBAN), xmlText(x.Zweck))
		b.WriteString("</DrctDbtTxInf>\n")
	}
	b.WriteString("</PmtInf>\n</CstmrDrctDbtInitn>\n</Document>\n")
	return []byte(b.String()), nil
}

func (a *App) laufXML(r *http.Request) ([]byte, string, error) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	i := a.st.laufIndexLocked(r.PathValue("id"))
	if i < 0 {
		return nil, "", notFound("Lastschrift-Lauf nicht gefunden")
	}
	l := a.st.d.Lastschriften[i]
	if l.Bereinigt {
		return nil, "", bad("Dieser Lauf ist wegen abgelaufener Aufbewahrungsfrist bereinigt")
	}
	raw, err := buildPain008(a.st.d.Settings, a.st.d.Sepa.GlaeubigerID, l, time.Now())
	art := "Vorauszahlung"
	if l.Art == "rechnungen" {
		art = "Rechnungen"
	}
	return raw, safeName(fmt.Sprintf("Lastschrift_%s_%s", art, l.Faellig), 80) + ".xml", err
}

func (a *App) handleLaufXML(w http.ResponseWriter, r *http.Request) {
	raw, name, err := a.laufXML(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	_, _ = w.Write(raw)
}

// handleLaufDatei legt die XML-Datei im Datenordner ab und zeigt sie im Dateimanager (für das Programmfenster).
func (a *App) handleLaufDatei(w http.ResponseWriter, r *http.Request) {
	raw, name, err := a.laufXML(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	dir := filepath.Join(a.st.dir, lastschriftOrdner)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		writeErr(w, err)
		return
	}
	full := filepath.Join(dir, name)
	if err := schreibeAtomar(full, raw, 0o600); err != nil {
		writeErr(w, err)
		return
	}
	revealPathFn(full)
	writeJSON(w, 200, map[string]string{"datei": name})
}

// ------------------------------------------------------------------ Kasse, Datenschutz

// vorauszahlungenLocked: Vorauszahlungen eines Abrechnungsjahres – per Lastschrift eingezogen (ohne
// Rücklastschriften) und per Überweisung eingegangen (auf den Abschlag gebucht).
func (s *Store) vorauszahlungenLocked(jahr int) float64 {
	sum := 0.0
	for _, v := range s.d.Vorauszahlungen {
		if v.Jahr == jahr && v.Gebucht {
			sum += v.Betrag
		}
	}
	for _, l := range s.d.Lastschriften {
		if l.Art != "vorauszahlung" || l.Jahr != jahr {
			continue
		}
		for _, x := range l.Posten {
			if x.Zurueck == "" {
				sum += x.Betrag
			}
		}
	}
	return round2(sum)
}

// sepaPersonEntfernen: Mandate der Person löschen; die Läufe sind Buchungsbelege und bleiben bis zum Ende der
// Aufbewahrungsfrist (danach bereinigt sepaBereinigen sie).
func (d *Data) sepaPersonEntfernen(id string) bool {
	changed := false
	rest := d.Mandate[:0]
	for _, m := range d.Mandate {
		if m.PaechterID == id {
			changed = true
			continue
		}
		rest = append(rest, m)
	}
	d.Mandate = rest
	for i := range d.Lastschriften {
		for k := range d.Lastschriften[i].Posten {
			if x := &d.Lastschriften[i].Posten[k]; x.PaechterID == id {
				x.PaechterID, x.MandatID, changed = "", "", true
			}
		}
	}
	return changed
}

func (d *Data) sepaBereinigen(aktuell int) int {
	n := 0
	for i := range d.Lastschriften {
		l := &d.Lastschriften[i]
		y, _ := strconv.Atoi(jahrVon(l.Faellig))
		if l.Bereinigt || !fristAbgelaufen(y, aktuell) {
			continue
		}
		for k := range l.Posten {
			x := &l.Posten[k]
			x.Name, x.IBAN, x.BIC, x.PaechterID, x.MandatID, x.Zweck = fristAbgelaufenText, "", "", "", "", ""
		}
		l.Bereinigt = true
		n++
	}
	return n
}

// sepaAktivLocked: Lastschrift ist eingeschaltet oder es gibt schon Mandate (dann bleibt der Bereich sichtbar).
func (s *Store) sepaAktivLocked() bool { return s.d.Sepa.Aktiv || len(s.d.Mandate) > 0 }
