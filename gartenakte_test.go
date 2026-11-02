package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// hochladen schickt eine Datei an /api/admin/dokumente.
func hochladen(t *testing.T, app *App, felder map[string]string, name string, inhalt []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range felder {
		_ = mw.WriteField(k, v)
	}
	fw, _ := mw.CreateFormFile("file", name)
	_, _ = fw.Write(inhalt)
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/admin/dokumente", &buf)
	req.Host = "127.0.0.1:8765"
	req.Header.Set("X-GA-Request", "1")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	app.routes(http.NotFoundHandler()).ServeHTTP(rec, req)
	return rec
}

func TestGartenakteDokumente(t *testing.T) {
	app, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "gartennr": "5", "name": "Anna Akte", "gartengroesse": 300}))
	pdf := []byte("%PDF-1.4\n%test\n")

	// gefälschte Dateien und Pfadtricks werden abgelehnt
	if rec := hochladen(t, app, map[string]string{"gartennr": "5"}, "boese.pdf", []byte("<html><script>alert(1)</script>")); rec.Code != 400 {
		t.Errorf("HTML als PDF: %d", rec.Code)
	}
	if rec := hochladen(t, app, map[string]string{"gartennr": "5"}, "x.exe", []byte("MZ")); rec.Code != 400 {
		t.Errorf("Programmdatei: %d", rec.Code)
	}
	if rec := hochladen(t, app, map[string]string{"gartennr": "5", "zurPerson": "1"}, "../../vertrag.pdf", pdf); rec.Code != 200 {
		t.Fatalf("Upload: %d %s", rec.Code, rec.Body.String())
	}
	foto := hochladen(t, app, map[string]string{"gartennr": "5", "titel": "Laube", "art": "Foto"}, "laube.jpg", jpegKopf("bild"))
	if foto.Code != 200 {
		t.Fatalf("Foto: %d %s", foto.Code, foto.Body.String())
	}
	ak := decode[gartenakte](t, do(h, "GET", "/api/admin/gartenakte?gartennr=5", nil))
	if len(ak.Dokumente) != 2 || ak.Paechter == nil || ak.Paechter.ID != p.ID {
		t.Fatalf("Akte: %+v", ak)
	}
	var vertrag, bild GartenDokument
	for _, d := range ak.Dokumente {
		if d.PaechterID != "" {
			vertrag = d
		} else {
			bild = d
		}
		if !strings.HasPrefix(d.Datei, "Gartenakte/") || strings.Contains(d.Datei, "..") {
			t.Errorf("Dateiname aus dem Upload übernommen: %q", d.Datei)
		}
	}
	if vertrag.PaechterID != p.ID || vertrag.Titel != "vertrag" {
		t.Errorf("Personendokument: %+v", vertrag)
	}
	// Bilder laufen in einer Sandbox, PDFs nicht (sonst blockiert der Browser den PDF-Betrachter)
	if rec := do(h, "GET", "/api/admin/dokumente/"+bild.ID, nil); rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "sandbox") || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("Bild ausliefern: %d %v", rec.Code, rec.Header())
	}
	if rec := do(h, "GET", "/api/admin/dokumente/"+vertrag.ID, nil); rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Errorf("PDF ausliefern: %d", rec.Code)
	}
	// ein manipulierter Pfad in der Datendatei öffnet keine fremde Datei
	app.st.mu.Lock()
	for i := range app.st.d.Dokumente {
		if app.st.d.Dokumente[i].ID == bild.ID {
			app.st.d.Dokumente[i].Datei = "Gartenakte/../kleingarten-manager-daten.json"
		}
	}
	app.st.mu.Unlock()
	if rec := do(h, "GET", "/api/admin/dokumente/"+bild.ID, nil); rec.Code == 200 {
		t.Error("Pfad mit .. wurde ausgeliefert")
	}
	if rec := do(h, "DELETE", "/api/admin/dokumente/"+bild.ID, nil); rec.Code != 200 {
		t.Errorf("Löschen trotz kaputtem Pfad: %d", rec.Code)
	}
	if _, err := os.Stat(app.st.dir + "/" + dataFileName); err != nil {
		t.Fatal("Datendatei wurde gelöscht!")
	}

	// endgültiges Löschen des Pächters entfernt sein Personendokument samt Datei
	full, _ := app.st.dokumentDatei(vertrag.Datei)
	do(h, "DELETE", "/api/admin/paechter/"+p.ID, nil)
	do(h, "DELETE", "/api/admin/paechter/"+p.ID+"/endgueltig", nil)
	if _, err := os.Stat(full); err == nil {
		t.Error("Pachtvertrag liegt nach dem Löschen noch auf der Platte")
	}
	if ak := decode[gartenakte](t, do(h, "GET", "/api/admin/gartenakte?gartennr=5", nil)); len(ak.Dokumente) != 0 || ak.Paechter != nil {
		t.Errorf("Akte nach dem Löschen: %+v", ak)
	}
	// Personendokument ohne Pächter geht nicht
	if rec := hochladen(t, app, map[string]string{"gartennr": "5", "zurPerson": "1"}, "v.pdf", pdf); rec.Code != 400 {
		t.Errorf("Personendokument für freien Garten: %d", rec.Code)
	}
}

func TestWertermittlung(t *testing.T) {
	_, h := newTestApp(t)
	for _, bad := range []map[string]any{
		{"gartennr": "5", "datum": "2026-01-01", "positionen": []any{}},
		{"gartennr": "5", "datum": "x", "positionen": []any{map[string]any{"bezeichnung": "Laube", "einzelwert": 100}}},
		{"gartennr": "5", "datum": "2026-01-01", "positionen": []any{map[string]any{"bezeichnung": "Laube", "einzelwert": 100, "abschlag": 120}}},
		{"gartennr": "5", "datum": "2026-01-01", "positionen": []any{map[string]any{"bezeichnung": "Laube", "einzelwert": -5}}},
	} {
		if rec := do(h, "POST", "/api/admin/wertermittlungen", bad); rec.Code != 400 {
			t.Errorf("ungültig angenommen: %v → %d", bad, rec.Code)
		}
	}
	x := decode[Wertermittlung](t, do(h, "POST", "/api/admin/wertermittlungen", map[string]any{
		"gartennr": "5", "datum": "2026-05-01", "positionen": []any{
			map[string]any{"kategorie": "Baulichkeiten", "bezeichnung": "Laube", "menge": 16, "einzelwert": 95, "abschlag": 35},
			map[string]any{"kategorie": "Anpflanzungen", "bezeichnung": "Apfelbaum", "menge": 3, "einzelwert": 60},
			map[string]any{"bezeichnung": "", "einzelwert": 0},
			map[string]any{"kategorie": "Abzug", "bezeichnung": "Entsorgung", "einzelwert": 80},
		}}))
	if x.Summe != 1088 || len(x.Positionen) != 3 || x.Anlass != "Pächterwechsel" {
		t.Fatalf("Wertermittlung: %+v", x)
	}
	// Abzüge machen den Wert nie negativ
	y := decode[Wertermittlung](t, do(h, "POST", "/api/admin/wertermittlungen", map[string]any{
		"gartennr": "6", "datum": "2026-05-01", "positionen": []any{map[string]any{"kategorie": "Abzug", "bezeichnung": "Rückbau", "einzelwert": 500}}}))
	if y.Summe != 0 {
		t.Errorf("negativer Wert: %v", y.Summe)
	}
	if rec := do(h, "GET", "/api/admin/wertermittlungen/"+x.ID+"/pdf", nil); rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Errorf("PDF: %d", rec.Code)
	}
	x.Positionen = x.Positionen[:1]
	if z := decode[Wertermittlung](t, do(h, "PUT", "/api/admin/wertermittlungen/"+x.ID, x)); z.Summe != 988 || z.ID != x.ID {
		t.Errorf("Ändern: %+v", z)
	}
	if ak := decode[gartenakte](t, do(h, "GET", "/api/admin/gartenakte?gartennr=5", nil)); len(ak.Wertermittlungen) != 1 {
		t.Errorf("Akte: %+v", ak.Wertermittlungen)
	}
	if rec := do(h, "DELETE", "/api/admin/wertermittlungen/"+x.ID, nil); rec.Code != 200 {
		t.Errorf("Löschen: %d", rec.Code)
	}
}

func TestUebergabeprotokollViele(t *testing.T) {
	// viele Positionen: mehrere Seiten, kein Absturz
	x := Wertermittlung{Gartennr: "1", Datum: "2026-01-01", Anlass: "Pächterwechsel"}
	for i := 0; i < 80; i++ {
		x.Positionen = append(x.Positionen, WertPosition{Kategorie: wertKategorien[i%4], Bezeichnung: strings.Repeat("Sehr lange Position ", 8), Menge: 1, Einzelwert: 10})
	}
	x, err := cleanWertermittlung(x)
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := buildWertermittlung(defaultSettings(), x)
	if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("PDF mit vielen Positionen: %v", err)
	}
}

func TestJahresmeldungUndBesichtigung(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	m := decode[jahresmeldung](t, do(h, "GET", "/api/admin/jahresmeldung?year="+strconv.Itoa(st.Year), nil))
	wert := map[string]float64{}
	for _, z := range m.Zeilen {
		wert[z.Angabe] = z.Wert
	}
	if wert["davon verpachtet"] != float64(len(demoListe)) || wert["neu vergebene Gärten im Jahr"] != 0 || wert["Interessenten auf der Warteliste (heute)"] != 3 {
		t.Errorf("Jahresmeldung: %v", wert)
	}
	// eine echte Vergabe zählt, das Eintragen langjähriger Pächter nicht
	l := decode[[]bewerberResp](t, do(h, "GET", "/api/admin/warteliste", nil))
	do(h, "PUT", "/api/admin/lageplan", map[string]any{"positionen": map[string]any{"50": map[string]int{"x": 0, "y": 400}}})
	jahr := time.Now().Year()
	do(h, "POST", "/api/admin/warteliste/"+l[0].ID+"/vergeben", map[string]any{"gartennr": "50", "mitgliedsnr": "50", "gartengroesse": 200, "seit": strconv.Itoa(jahr) + "-03-01"})
	app.st.mu.Lock()
	m2, _ := app.st.jahresmeldungLocked(jahr, time.Now())
	app.st.mu.Unlock()
	for _, z := range m2.Zeilen {
		if z.Angabe == "neu vergebene Gärten im Jahr" && z.Wert != 1 {
			t.Errorf("Vergabe nicht gezählt: %v", z.Wert)
		}
	}
	if rec := do(h, "GET", "/api/admin/jahresmeldung?format=xlsx", nil); rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("PK")) {
		t.Errorf("Excel: %d", rec.Code)
	}

	// Besichtigung: Datum, Uhrzeit und Ergebnis werden geprüft
	b := l[1].Bewerber
	b.BesichtigungAm, b.BesichtigungUm, b.BesichtigungErgebnis = "2026-11-10", "25:00", "besichtigt"
	if rec := do(h, "PUT", "/api/admin/warteliste/"+b.ID, b); rec.Code != 400 {
		t.Errorf("ungültige Uhrzeit: %d", rec.Code)
	}
	b.BesichtigungUm, b.BesichtigungErgebnis = "17:30", "Unsinn"
	got := decode[Bewerber](t, do(h, "PUT", "/api/admin/warteliste/"+b.ID, b))
	if got.BesichtigungUm != "17:30" || got.BesichtigungErgebnis != "" {
		t.Errorf("Besichtigung: %+v", got)
	}
	b.BesichtigungAm = ""
	if got := decode[Bewerber](t, do(h, "PUT", "/api/admin/warteliste/"+b.ID, b)); got.BesichtigungUm != "" || got.BesichtigungGarten != "" {
		t.Errorf("ohne Datum bleiben Reste: %+v", got)
	}
}
