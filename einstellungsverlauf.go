package main

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Änderungsverlauf der Preise und Einstellungen: die letzten Stände lassen sich zurücknehmen.
// Gespeichert werden nur Änderungen über die Seite »Preise & Einstellungen«; Jahr und zweiter
// Sicherungsordner werden nie zurückgesetzt.

const maxEinstellungsStaende = 10

type EinstellungsStand struct {
	Datum    string   `json:"datum"` // JJJJ-MM-TT HH:MM
	Settings Settings `json:"settings"`
}

var einstellungsNamen = map[string]string{
	"VereinName": "Vereinsname", "Ort": "Ort", "Absenderzeile": "Absenderzeile", "Rechnungsdatum": "Rechnungsdatum",
	"Zahlungsziel": "Zahlungsziel", "RechnungsnrPraefix": "Rechnungsnummern-Anfang", "EinspruchTage": "Einspruchsfrist (Tage)",
	"BankName": "Bank", "IBAN": "IBAN", "BIC": "BIC", "WasserGrundpreis": "Wasser-Grundpreis", "WasserPreis": "Wasserpreis",
	"EnergieGrundpreis": "Strom-Grundpreis", "EnergiePreis": "Strompreis", "Pflichtstunden": "Pflichtstunden",
	"StundenObergrenze": "Stunden-Obergrenze", "VerguetungJeStd": "Vergütung je Stunde", "NachzahlungJeStd": "Nachzahlung je Stunde",
	"PachtJeQm": "Pacht je m²", "VereinsflaecheQm": "Vereinsfläche (m²)", "FreieGaertenQm": "Freie Gärten (m²)",
	"Vereinsbeitrag": "Vereinsbeitrag", "Territorialverband": "Verbandsbeitrag", "UmlageStandard": "Umlage", "Farbe": "Vereinsfarbe", "Logo": "Logo",
}

// einstellungsAenderungen beschreibt in einfachen Worten, was sich von alt nach neu geändert hat.
func einstellungsAenderungen(alt, neu Settings) []string {
	var out []string
	va, vn := reflect.ValueOf(alt), reflect.ValueOf(neu)
	t := va.Type()
	for i := 0; i < t.NumField(); i++ {
		name := t.Field(i).Name
		label, ok := einstellungsNamen[name]
		if !ok || reflect.DeepEqual(va.Field(i).Interface(), vn.Field(i).Interface()) {
			continue
		}
		fmtWert := func(v reflect.Value) string {
			switch v.Kind() {
			case reflect.Float64:
				return strings.Replace(strconv.FormatFloat(v.Float(), 'f', -1, 64), ".", ",", 1)
			case reflect.String:
				if v.String() == "" {
					return "(leer)"
				}
				return v.String()
			}
			return fmt.Sprint(v.Interface())
		}
		out = append(out, fmt.Sprintf("%s: %s → %s", label, fmtWert(va.Field(i)), fmtWert(vn.Field(i))))
	}
	return out
}

// einstellungenMerkenLocked legt den bisherigen Stand vorn in den Verlauf. Der Aufrufer hält s.mu.
func (s *Store) einstellungenMerkenLocked(alt Settings) {
	s.d.EinstellungsVerlauf = append([]EinstellungsStand{{Datum: time.Now().Format("2006-01-02 15:04"), Settings: alt}}, s.d.EinstellungsVerlauf...)
	if len(s.d.EinstellungsVerlauf) > maxEinstellungsStaende {
		s.d.EinstellungsVerlauf = s.d.EinstellungsVerlauf[:maxEinstellungsStaende]
	}
}

type einstellungsZeile struct {
	Index        int      `json:"index"`
	Datum        string   `json:"datum"`
	Aenderungen  []string `json:"aenderungen"`
	Zuruecknehmb bool     `json:"zuruecknehmbar"`
}

func (a *App) handleEinstellungsVerlauf(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	out := []einstellungsZeile{}
	for i, st := range a.st.d.EinstellungsVerlauf {
		// Spätere Stand dieser Änderung: der nächstneuere Eintrag oder der aktuelle Stand.
		neuer := a.st.d.Settings
		if i > 0 {
			neuer = a.st.d.EinstellungsVerlauf[i-1].Settings
		}
		ae := einstellungsAenderungen(st.Settings, neuer)
		if len(ae) == 0 || st.Settings.Jahr != a.st.d.Settings.Jahr {
			continue
		}
		out = append(out, einstellungsZeile{Index: i, Datum: st.Datum, Aenderungen: ae, Zuruecknehmb: true})
	}
	writeJSON(w, 200, out)
}

func (a *App) handleEinstellungenZurueck(w http.ResponseWriter, r *http.Request) {
	i, err := strconv.Atoi(r.PathValue("index"))
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if err != nil || i < 0 || i >= len(a.st.d.EinstellungsVerlauf) {
		writeErr(w, notFound("Dieser Stand ist nicht mehr da"))
		return
	}
	alt := a.st.d.EinstellungsVerlauf[i].Settings
	akt := a.st.d.Settings
	if alt.Jahr != akt.Jahr {
		writeErr(w, bad("Dieser Stand gehört zu einem anderen Abrechnungsjahr."))
		return
	}
	alt.Jahr, alt.ZweiteSicherung = akt.Jahr, akt.ZweiteSicherung
	if alt == akt {
		writeErr(w, bad("Das ist schon der aktuelle Stand."))
		return
	}
	a.st.einstellungenMerkenLocked(akt)
	a.st.d.Settings = alt
	a.st.audit("Einstellungen auf den Stand vom %s zurückgesetzt", a.st.d.EinstellungsVerlauf[i+1].Datum)
	a.speichernUndAntworten(w, alt)
}
