package main

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"
)

// Bankabgleich lernt: Ordnet der Vorstand eine Zahlung von Hand einem Pächter zu, merkt sich das Programm den
// Absendernamen laut Bank – nur als Prüfsumme, nie im Klartext – und schlägt beim nächsten Kontoauszug dieselbe
// Zuordnung vor. Dazu kommen die Mitgliedsnummer im Verwendungszweck und Namen in anderer Reihenfolge
// (»Müller, Hans« statt »Hans Müller«). Vorschläge werden immer zur Prüfung angezeigt, nie automatisch gebucht.

const maxGelernt = 2000

func lernSchluessel(name string) string {
	k := kompakt(name)
	if len([]rune(k)) < 4 {
		return ""
	}
	sum := sha256.Sum256([]byte("absender|" + k))
	return hex.EncodeToString(sum[:8])
}

// kontoLernenLocked merkt: Zahlungen mit diesem Absender gehören zu diesem Pächter.
func (s *Store) kontoLernenLocked(absender, paechterID string) {
	k := lernSchluessel(absender)
	if k == "" || paechterID == "" {
		return
	}
	if s.d.KontoGelernt == nil {
		s.d.KontoGelernt = map[string]string{}
	}
	if _, da := s.d.KontoGelernt[k]; !da && len(s.d.KontoGelernt) >= maxGelernt {
		return
	}
	s.d.KontoGelernt[k] = paechterID
}

// kontoGelerntPersonEntfernen vergisst alles, was zu dieser Person gelernt wurde (Datenschutz).
func (d *Data) kontoGelerntPersonEntfernen(id string) bool {
	geaendert := false
	for k, v := range d.KontoGelernt {
		if v == id {
			delete(d.KontoGelernt, k)
			geaendert = true
		}
	}
	return geaendert
}

// nameTokensEnthalten: Alle Namensbestandteile (mindestens zwei, je mindestens drei Buchstaben) stehen im Text,
// in beliebiger Reihenfolge. text ist bereits mit kompakt() aufbereitet.
func nameTokensEnthalten(text, name string) bool {
	teile := strings.FieldsFunc(name, func(r rune) bool { return !unicode.IsLetter(r) })
	n := 0
	for _, t := range teile {
		if len([]rune(t)) < 3 {
			continue
		}
		if !strings.Contains(text, strings.ToUpper(t)) {
			return false
		}
		n++
	}
	return n >= 2
}

var mitgliedsnrImText = regexp.MustCompile(`(?i)mitglied\w*[\s.\-]*(?:nr|nummer)?[\s.:#\-]*(\d{1,10})`)

// mitgliedsnrAusZweck liefert die Mitgliedsnummer, wenn der Verwendungszweck eine nennt.
func mitgliedsnrAusZweck(zweck string) string {
	if m := mitgliedsnrImText.FindStringSubmatch(zweck); m != nil {
		return strings.TrimLeft(m[1], "0")
	}
	return ""
}
