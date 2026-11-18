package main

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

// Eigene Brieftexte: Vereine haben ihre eigene Tonlage. Anpassbar sind die festen Sätze der Briefe; was sich je
// Brief ändert (Name, Garten, Daten, Beträge), kommt über Platzhalter hinein. Leer = Standardtext des Programms.
// Die Texte stehen in den Einstellungen: Ein abgeschlossenes Jahr behält die Texte, die damals galten, und der
// Einstellungs-Verlauf kann eine Änderung zurücknehmen. Geändert werden sie nur über /api/admin/brieftexte.

const maxBrieftext = 3000

// Brieftexte: ein Feld je Vorlage, der JSON-Name ist der Schlüssel (siehe brieftextVorlagen).
type Brieftexte struct {
	Anrede                 string `json:"anrede,omitempty"`
	Gruss                  string `json:"gruss,omitempty"`
	Unterschrift           string `json:"unterschrift,omitempty"`
	Mahnung1               string `json:"mahnung1,omitempty"`
	Mahnung2               string `json:"mahnung2,omitempty"`
	Mahnung3               string `json:"mahnung3,omitempty"`
	MahnungSchluss         string `json:"mahnungSchluss,omitempty"`
	MaengelErst            string `json:"maengelErst,omitempty"`
	MaengelZweit           string `json:"maengelZweit,omitempty"`
	WartelisteAngebot      string `json:"wartelisteAngebot,omitempty"`
	WartelisteNachfrage    string `json:"wartelisteNachfrage,omitempty"`
	WartelisteBestaetigung string `json:"wartelisteBestaetigung,omitempty"`
	Kuendigung             string `json:"kuendigung,omitempty"`
}

type brieftextVorlage struct {
	Key         string   `json:"key"`
	Gruppe      string   `json:"gruppe"`
	Titel       string   `json:"titel"`
	Hilfe       string   `json:"hilfe"`
	Platzhalter []string `json:"platzhalter"`
	Standard    string   `json:"standard"`
}

var brieftextVorlagen = []brieftextVorlage{
	{"anrede", "Für alle Briefe", "Anrede", "Steht über jedem Brief an einen Pächter.", nil,
		"Liebe Gartenfreundin, lieber Gartenfreund,"},
	{"gruss", "Für alle Briefe", "Grußformel", "Steht vor der Unterschrift.", nil,
		"Mit freundlichen Gartengrüßen"},
	{"unterschrift", "Für alle Briefe", "Unterschrift", "Zum Beispiel »Der Vorstand« oder ein Name mit Amt.", nil,
		"Der Vorstand"},
	{"mahnung1", "Mahnung", "Zahlungserinnerung (1. Stufe): Zusatz", "Wird an den Satz mit dem offenen Betrag angehängt. Leer lassen, wenn nichts dazu soll.", nil,
		""},
	{"mahnung2", "Mahnung", "Mahnung (2. Stufe): Zusatz", "Wird an den Satz mit dem offenen Betrag angehängt.", nil,
		"Wir hatten Sie bereits an die Zahlung erinnert."},
	{"mahnung3", "Mahnung", "Letzte Mahnung (3. Stufe): Zusatz", "Wird an den Satz mit dem offenen Betrag angehängt.", nil,
		"Trotz Erinnerung und Mahnung ist die Zahlung nicht eingegangen. Dies ist unsere letzte Mahnung; danach müssen wir weitere Schritte nach Pachtvertrag und Satzung prüfen."},
	{"mahnungSchluss", "Mahnung", "Schlusssatz unter der Bankverbindung", "", nil,
		"Sollte die Zahlung zwischenzeitlich bereits erfolgt sein, betrachten Sie dieses Schreiben als gegenstandslos."},
	{"maengelErst", "Mängel", "Aufforderung und Erinnerung: Schlusstext", "Steht unter der Liste der Mängel.", []string{"Frist"},
		"Wir bitten Sie, diese Mängel bis zum {Frist} zu beseitigen. Sind sie bereits behoben oder gibt es Gründe, warum das nicht möglich ist, melden Sie sich bitte beim Vorstand. " +
			"Wir bitten um Verständnis, dass wir nach Fristablauf weitere Schritte nach Pachtvertrag und Satzung prüfen müssen."},
	{"maengelZweit", "Mängel", "Letzte Aufforderung: Schlusstext", "Steht unter der Liste der Mängel.", []string{"Frist"},
		"Wir fordern Sie auf, diese Mängel bis zum {Frist} zu beseitigen. Das ist die letzte Frist. Danach wird der Vorstand über eine Abmahnung und weitere Schritte nach Pachtvertrag, Satzung und Bundeskleingartengesetz entscheiden. " +
			"Melden Sie sich bitte vorher, wenn es Gründe gibt, warum die Beseitigung nicht möglich ist."},
	{"wartelisteAngebot", "Warteliste", "Garten anbieten", "{Garten} wird zu »der Garten 12« oder »ein Garten«; {Besichtigung} zum Satz über den Besichtigungstermin.", []string{"Seit", "Garten", "Besichtigung"},
		"Sie stehen seit dem {Seit} auf unserer Warteliste. Nun wird {Garten} in unserer Anlage frei, und wir möchten ihn Ihnen anbieten.\n\n" +
			"{Besichtigung} Dabei besprechen wir auch die Übernahmekosten, die Pachtbedingungen und die Gartenordnung.\n\n" +
			"Bitte teilen Sie uns innerhalb von 14 Tagen mit, ob Sie Interesse haben. Andernfalls bieten wir den Garten dem Nächsten auf der Liste an."},
	{"wartelisteNachfrage", "Warteliste", "Interesse nachfragen", "{Frist} ist vier Wochen nach dem Briefdatum.", []string{"Seit", "Frist"},
		"Sie stehen seit dem {Seit} auf der Warteliste unseres Vereins. Damit die Liste aktuell bleibt und wir keine Angaben unnötig aufbewahren, fragen wir regelmäßig nach.\n\n" +
			"Bitte teilen Sie uns kurz mit, ob Sie weiterhin Interesse an einem Garten haben. Das geht formlos per Post, E-Mail oder im Gespräch mit dem Vorstand. " +
			"Hören wir bis zum {Frist} nichts von Ihnen, nehmen wir Sie von der Liste und löschen Ihre Angaben."},
	{"wartelisteBestaetigung", "Warteliste", "Anmeldung bestätigen", "", []string{"Seit"},
		"wir bestätigen, dass wir Sie am {Seit} auf die Warteliste für einen Kleingarten aufgenommen haben.\n\n" +
			"Sobald ein Garten frei wird, melden wir uns in der Reihenfolge der Anmeldung bei Ihnen. Wie lange das dauert, lässt sich nicht vorhersagen. Bitte teilen Sie uns Änderungen Ihrer Anschrift oder Ihres Wunsches mit. " +
			"Ihre Angaben verwenden wir nur für die Vergabe der Gärten und löschen sie, wenn Sie kein Interesse mehr haben."},
	{"kuendigung", "Kündigung", "Bestätigung: Ablauf bis zur Übergabe", "Steht unter dem ersten Satz (wer zu wann gekündigt hat). {Schluessel} wird zur Zeile mit den zurückzugebenden Schlüsseln (oder bleibt leer).", []string{"Garten", "Zum", "Schluessel"},
		"Bis zur Übergabe gilt Folgendes:\n" +
			"•  Der Garten wird bis dahin weiter gepflegt; Pacht, Beiträge und Verbrauch werden bis zur Übergabe abgerechnet.\n" +
			"•  Für Laube, Anpflanzungen und Einrichtungen findet eine Wertermittlung statt. Den Termin stimmen wir mit Ihnen ab.\n" +
			"•  Bei der Übergabe lesen wir gemeinsam die Zähler ab; danach erhalten Sie die Schlussrechnung.{Schluessel}\n\n" +
			"Gern helfen wir dabei, eine Nachfolgerin oder einen Nachfolger für den Garten zu finden. Bei Fragen melden Sie sich bitte beim Vorstand."},
}

// grenze: höchste Zeichenzahl je Text. Kurz für Anrede und Gruß; die Mahnung muss auf eine Seite passen.
func (v brieftextVorlage) grenze() int {
	switch v.Key {
	case "anrede", "gruss", "unterschrift":
		return 120
	case "mahnung1", "mahnung2", "mahnung3":
		return 400
	case "mahnungSchluss":
		return 300
	}
	return maxBrieftext
}

func brieftextVorlageZu(key string) (brieftextVorlage, bool) {
	for _, v := range brieftextVorlagen {
		if v.Key == key {
			return v, true
		}
	}
	return brieftextVorlage{}, false
}

// brieftextFeld liefert das Feld zu einem Schlüssel (JSON-Name).
func brieftextFeld(b *Brieftexte, key string) (reflect.Value, bool) {
	v := reflect.ValueOf(b).Elem()
	for i := 0; i < v.NumField(); i++ {
		if strings.Split(v.Type().Field(i).Tag.Get("json"), ",")[0] == key {
			return v.Field(i), true
		}
	}
	return reflect.Value{}, false
}

// text liefert den Brieftext zu key – eigener Text oder Standard – mit eingesetzten Platzhaltern {Name}.
func (b Brieftexte) text(key string, werte map[string]string) string {
	t := ""
	if f, ok := brieftextFeld(&b, key); ok {
		t = f.String()
	}
	if strings.TrimSpace(t) == "" {
		v, _ := brieftextVorlageZu(key)
		t = v.Standard
	}
	for k, w := range werte {
		t = strings.ReplaceAll(t, "{"+k+"}", w)
	}
	return t
}

type brieftextZeile struct {
	brieftextVorlage
	Text      string `json:"text"`
	Angepasst bool   `json:"angepasst"`
	Max       int    `json:"max"` // höchstens so viele Zeichen (die Mahnung muss auf eine Seite passen)
}

func (a *App) handleBrieftexte(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	b := a.st.d.Settings.Brieftexte
	out := make([]brieftextZeile, 0, len(brieftextVorlagen))
	for _, v := range brieftextVorlagen {
		z := brieftextZeile{brieftextVorlage: v, Text: v.Standard}
		z.Max = v.grenze()
		if v.Platzhalter == nil {
			z.Platzhalter = []string{}
		}
		if f, ok := brieftextFeld(&b, v.Key); ok && strings.TrimSpace(f.String()) != "" {
			z.Text, z.Angepasst = f.String(), true
		}
		out = append(out, z)
	}
	writeJSON(w, 200, out)
}

// handleBrieftextSpeichern setzt einen Brieftext. Leer oder gleich dem Standard = zurück zum Standardtext.
func (a *App) handleBrieftextSpeichern(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Text string `json:"text"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	key := r.PathValue("key")
	v, ok := brieftextVorlageZu(key)
	if !ok {
		writeErr(w, notFound("Unbekannter Brieftext"))
		return
	}
	text := strings.TrimSpace(strings.ReplaceAll(in.Text, "\r\n", "\n"))
	if n := len([]rune(text)); n > v.grenze() {
		writeErr(w, bad(fmt.Sprintf("Der Text ist zu lang: %d Zeichen, erlaubt sind höchstens %d.", n, v.grenze())))
		return
	}
	if text == strings.TrimSpace(v.Standard) {
		text = ""
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	neu := a.st.d.Settings
	f, _ := brieftextFeld(&neu.Brieftexte, key)
	if f.String() == text {
		writeJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	a.st.einstellungenMerkenLocked(a.st.d.Settings)
	f.SetString(text)
	a.st.d.Settings = neu
	if text == "" {
		a.st.audit("Brieftext »%s« auf den Standard zurückgesetzt", v.Titel)
	} else {
		a.st.audit("Brieftext »%s« geändert", v.Titel)
	}
	a.speichernUndAntworten(w, map[string]bool{"ok": true})
}
