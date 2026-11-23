package main

import (
	"fmt"
	"net/http"
	"time"
)

// Ratenplan: Der Vorstand vereinbart mit einem Pächter, eine Rechnung in Raten zu bezahlen.
// Die Raten ändern weder den Betrag noch die Rechnung selbst. Gezahlt wird wie immer über den Zahlungs-Dialog
// (Teilbetrag); der Plan sagt nur, bis wann wie viel da sein soll. Danach richten sich Fälligkeit,
// Überfällig-Hinweis im Cockpit und die Mahnung.

const ratenMax = 24

// Rate ist ein Teilbetrag mit Fälligkeitsdatum.
type Rate struct {
	Faellig string  `json:"faellig"` // JJJJ-MM-TT
	Betrag  float64 `json:"betrag"`
}

// faelligAm ist das Datum, bis zu dem die Rechnung (laut Plan) spätestens zu einem Teil bezahlt sein muss:
// ohne Plan das Zahlungsziel, mit Plan das Datum der ersten Rate, die noch nicht durch Zahlungen gedeckt ist.
func faelligAm(r *Rechnung) string {
	if len(r.Ratenplan) == 0 {
		return r.Settings.Zahlungsziel
	}
	gezahlt := paidAmount(r)
	summe := 0.0
	for _, ra := range r.Ratenplan {
		summe += ra.Betrag
		if summe > gezahlt+0.005 {
			return ra.Faellig
		}
	}
	return r.Ratenplan[len(r.Ratenplan)-1].Faellig
}

// ratenGeprueft prüft einen Plan gegen den Rechnungsbetrag und liefert die bereinigte Liste.
func ratenGeprueft(raten []Rate, soll float64) ([]Rate, error) {
	if len(raten) > ratenMax {
		return nil, bad(fmt.Sprintf("Höchstens %d Raten", ratenMax))
	}
	out := make([]Rate, 0, len(raten))
	summe, letzte := 0.0, time.Time{}
	for i, ra := range raten {
		d, err := parseDate(ra.Faellig)
		if err != nil {
			return nil, bad(fmt.Sprintf("Rate %d: Bitte ein gültiges Datum angeben", i+1))
		}
		if i > 0 && d.Before(letzte) {
			return nil, bad(fmt.Sprintf("Rate %d: Das Datum darf nicht vor der vorigen Rate liegen", i+1))
		}
		if !validNum(ra.Betrag) || ra.Betrag <= 0 {
			return nil, bad(fmt.Sprintf("Rate %d: Bitte einen Betrag über 0 angeben", i+1))
		}
		letzte = d
		b := round2(ra.Betrag)
		summe += b
		out = append(out, Rate{Faellig: ra.Faellig, Betrag: b})
	}
	if len(out) > 0 && round2(summe-soll) != 0 {
		return nil, bad(fmt.Sprintf("Die Raten ergeben %s, die Rechnung lautet aber auf %s", fmtEUR(round2(summe)), fmtEUR(soll)))
	}
	return out, nil
}

// handleRatenplan setzt oder löscht (leere Liste) den Ratenplan einer Rechnung.
func (a *App) handleRatenplan(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Raten []Rate `json:"raten"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	x := a.st.rechnungLocked(r.PathValue("id"))
	if x == nil {
		writeErr(w, notFound("Rechnung nicht gefunden"))
		return
	}
	if x.Status != statusGueltig || x.Bereinigt {
		writeErr(w, bad("Zu dieser Rechnung ist kein Ratenplan mehr möglich (ersetzt oder bereinigt)"))
		return
	}
	if x.Result.Gesamt <= 0 {
		writeErr(w, bad("Zu einem Guthaben gibt es keinen Ratenplan"))
		return
	}
	raten, err := ratenGeprueft(in.Raten, zuZahlen(x))
	if err != nil {
		writeErr(w, err)
		return
	}
	if len(raten) == 0 {
		x.Ratenplan = nil
		a.st.audit("Ratenplan zu Rechnung %s gelöscht", x.Nummer)
	} else {
		x.Ratenplan = raten
		a.st.audit("Ratenplan zu Rechnung %s: %d Raten", x.Nummer, len(raten))
	}
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "faellig": faelligAm(x)})
}

// ratePasst: der Betrag deckt genau die nächste fällige Rate (nach schon Gezahltem und schon vorgeschlagenen Beträgen).
func ratePasst(r *Rechnung, schonVergeben, betrag float64) bool {
	if len(r.Ratenplan) == 0 {
		return false
	}
	gezahlt := paidAmount(r) + schonVergeben
	summe := 0.0
	for _, ra := range r.Ratenplan {
		summe += ra.Betrag
		if summe > gezahlt+0.005 {
			return absf(round2(summe-gezahlt)-betrag) < 0.005
		}
	}
	return false
}

// betragPasst: voller offener Betrag oder genau die nächste Rate.
func betragPasst(r *Rechnung, schonVergeben, betrag float64) bool {
	return absf(openAmount(r)-schonVergeben-betrag) < 0.005 || ratePasst(r, schonVergeben, betrag)
}
