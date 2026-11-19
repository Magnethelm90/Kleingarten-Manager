package main

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// TestAllePDFsMitSchwierigenZeichen: Jede Lese-Route (auch mit {id}) wird mit Beispieldaten aufgerufen, deren Namen
// und Texte Umlaute, Sonderzeichen, sehr lange Wörter und Zeichen über U+FFFF enthalten. Kein PDF darf abstürzen.
func TestAllePDFsMitSchwierigenZeichen(t *testing.T) {
	src, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`HandleFunc\("GET (/api/[^"]+)",`)
	treffer := re.FindAllStringSubmatch(string(src), -1)
	app, h := newTestApp(t)
	if rec := do(h, "POST", "/api/admin/beispieldaten", map[string]any{}); rec.Code >= 300 {
		t.Fatalf("Beispieldaten: %d", rec.Code)
	}
	schwierig := "Ärger-Öhlschläger-Müßiggang \"<&>\" 🌱 " + strings.Repeat("Langeswortohneumbruch", 6) + " ẞ € ½"
	app.st.mu.Lock()
	d := app.st.d
	d.Settings.VereinName = "KGV »Grüne Aue« 🌻 e. V. " + strings.Repeat("Sehr langer Vereinsname ", 4)
	for i := range d.Paechter {
		d.Paechter[i].Name = schwierig
		d.Paechter[i].Strasse = schwierig
	}
	for i := range d.Maengel {
		d.Maengel[i].Beschreibung = schwierig + " " + schwierig
	}
	for i := range d.Termine {
		d.Termine[i].Titel = schwierig
		d.Termine[i].Notiz = schwierig
	}
	// wie beim Speichern: Zeichen, die die PDF-Schrift nicht kann (über U+FFFF), fallen weg
	texteBereinigen(reflect.ValueOf(d))
	// alle IDs im Datenbestand als Kandidaten für {id}
	roh, _ := json.Marshal(d)
	app.st.mu.Unlock()
	ids := regexp.MustCompile(`"id":"([^"]+)"`).FindAllStringSubmatch(string(roh), -1)
	var mangelIDs []string
	mangelGarten := ""
	app.st.mu.Lock()
	for _, m := range app.st.d.Maengel {
		mangelIDs = append(mangelIDs, m.ID)
		if m.ErledigtAm == "" {
			mangelGarten = m.Gartennr
		}
	}
	nrs := []string{}
	for _, p := range app.st.d.Paechter {
		nrs = append(nrs, p.Gartennr)
	}
	app.st.mu.Unlock()
	extra := map[string]string{
		"/api/admin/saisonliste":           "?art=herbst",
		"/api/admin/maengel-liste":         "?ids=" + strings.Join(mangelIDs, ","),
		"/api/admin/maengelschreiben":      "?gartennr=" + mangelGarten,
		"/api/admin/expose":                "?gartennr=" + nrs[0],
		"/api/admin/gartenakte":            "?gartennr=" + nrs[0],
		"/api/admin/wahl/stimmzettel":      "?amt=" + url.QueryEscape(schwierig) + "&kandidat=" + url.QueryEscape(schwierig) + "&kandidat=B&anzahl=8",
		"/api/admin/warteliste/{id}/brief": "?art=aufnahmeantrag&garten=" + nrs[0],
		"/api/admin/befragungen/{id}/pdf":  "?art=auswertung",
	}
	param := regexp.MustCompile(`\{(\w+)\}`)
	pdfs := 0
	for _, m := range treffer {
		route := m[1]
		kandidaten := []string{route}
		if p := param.FindStringSubmatch(route); p != nil {
			kandidaten = nil
			werte := ids
			if p[1] == "nr" {
				werte = nil
				for _, n := range nrs {
					werte = append(werte, []string{"", n})
				}
			}
			for _, w := range werte {
				kandidaten = append(kandidaten, param.ReplaceAllString(route, w[1]))
			}
		}
		for _, u := range kandidaten {
			rec := do(h, "GET", u+extra[route], nil)
			if rec.Code >= 400 && rec.Code < 500 && len(kandidaten) > 1 {
				continue // ID gehört zu einer anderen Art von Eintrag
			}
			if testing.Verbose() {
				t.Logf("%s %d %s", u, rec.Code, rec.Header().Get("Content-Type"))
			}
			if rec.Code >= 500 {
				t.Errorf("GET %s -> %d %s", u, rec.Code, rec.Body.String())
			}
			if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/pdf") {
				pdfs++
				if !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
					t.Errorf("GET %s: kein PDF", u)
				}
			}
			break // eine passende ID je Route genügt
		}
	}
	if pdfs < 25 {
		t.Errorf("nur %d PDFs erzeugt – Routen-Suche prüfen", pdfs)
	}
	t.Logf("%d PDFs geprüft", pdfs)
}
