package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// luftbildTest erzeugt ein »Luftbild« mit Struktur (ein einfarbiges Bild gilt als leere Antwort).
func luftbildTest(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(60 + (x*7+y*3)%120), uint8(100 + (x/20+y/20)%2*60), 50, 255})
		}
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, nil)
	return b.Bytes()
}

func TestLageplanHintergrundUndUmrisse(t *testing.T) {
	app, h := newTestApp(t)
	// kein Bild, zu klein: abgelehnt
	if rec := doMultipart(h, "/api/admin/lageplan/bild", "x.png", []byte("kein Bild")); rec.Code != 400 {
		t.Errorf("Text als Bild: %d", rec.Code)
	}
	if rec := doMultipart(h, "/api/admin/lageplan/bild", "x.png", testBild(t, 50, 50, false)); rec.Code != 400 {
		t.Errorf("zu kleines Bild: %d", rec.Code)
	}
	// großes Bild wird verkleinert und als JPEG gespeichert
	rec := doMultipart(h, "/api/admin/lageplan/bild", "plan.jpg", luftbildTest(4000, 2000))
	if rec.Code != 200 {
		t.Fatalf("Bild hochladen: %d %s", rec.Code, rec.Body.String())
	}
	k := decode[LageKarte](t, rec)
	if !karteNameGueltig(k.Bild) || k.BildBreite != karteMaxKante || k.BildHoehe != 1500 || k.Massstab <= 0 || k.Deckkraft <= 0 {
		t.Fatalf("Karte nach dem Hochladen: %+v", k)
	}
	if rec := do(h, "GET", "/api/lageplan-bild", nil); rec.Code != 200 || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("Bild abrufen: %d", rec.Code)
	}
	erstes := k.Bild

	// Anordnung mit Umriss, Einrichtung und Leitung
	body := map[string]any{
		"positionen": map[string]any{
			"1": map[string]any{"x": 0, "y": 0, "umriss": []map[string]int{{"x": 100, "y": 100}, {"x": 220, "y": 100}, {"x": 220, "y": 200}, {"x": 100, "y": 200}}},
			"2": map[string]any{"x": 300, "y": 100},
		},
		"karte": map[string]any{
			"massstab": 0.5, "deckkraft": 0.7, "quelle": "Plan: Vorstand",
			"objekte":   []map[string]any{{"art": "wasserzaehler", "x": 90, "y": 90, "bez": "Hauptzähler"}},
			"leitungen": []map[string]any{{"art": "wasser", "punkte": []map[string]int{{"x": 90, "y": 90}, {"x": 400, "y": 90}}}},
		},
	}
	if rec := do(h, "PUT", "/api/admin/lageplan", body); rec.Code != 200 {
		t.Fatalf("Anordnung speichern: %d %s", rec.Code, rec.Body.String())
	}
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if p := st.Lageplan["1"]; p.X != 100 || p.Y != 100 || len(p.Umriss) != 4 {
		t.Errorf("Umriss: %+v", p)
	}
	if st.LageKarte.Massstab != 0.5 || st.LageKarte.Deckkraft != 0.7 || st.LageKarte.Quelle != "Plan: Vorstand" ||
		len(st.LageKarte.Objekte) != 1 || len(st.LageKarte.Leitungen) != 1 {
		t.Errorf("Karte: %+v", st.LageKarte)
	}

	// Unsinn wird abgelehnt
	for name, b := range map[string]map[string]any{
		"Umriss mit 2 Punkten": {"positionen": map[string]any{"1": map[string]any{"x": 0, "y": 0, "umriss": []map[string]int{{"x": 1, "y": 1}, {"x": 2, "y": 2}}}}},
		"negative Ecke":        {"positionen": map[string]any{"1": map[string]any{"x": 0, "y": 0, "umriss": []map[string]int{{"x": -1, "y": 1}, {"x": 2, "y": 2}, {"x": 3, "y": 0}}}}},
		"unbekannte Art":       {"positionen": map[string]any{}, "karte": map[string]any{"massstab": 1, "deckkraft": 1, "objekte": []map[string]any{{"art": "<script>", "x": 1, "y": 1}}}},
		"Linie mit 1 Punkt":    {"positionen": map[string]any{}, "karte": map[string]any{"massstab": 1, "deckkraft": 1, "leitungen": []map[string]any{{"art": "wasser", "punkte": []map[string]int{{"x": 1, "y": 1}}}}}},
		"Deckkraft 0":          {"positionen": map[string]any{}, "karte": map[string]any{"massstab": 1, "deckkraft": 0}},
	} {
		if rec := do(h, "PUT", "/api/admin/lageplan", b); rec.Code != 400 {
			t.Errorf("%s: %d, erwartet 400", name, rec.Code)
		}
	}

	// Ausdruck mit Bild, Umriss, Leitung und Einrichtung
	app.st.mu.Lock()
	lp := app.st.lagePlanLocked(true)
	s := app.st.d.Settings
	app.st.mu.Unlock()
	if len(lp.Bild) == 0 {
		t.Fatal("Bild fehlt im Lageplan")
	}
	pdf, err := buildLageplan(s, lp, true)
	if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("Lageplan-PDF: %v", err)
	}
	if dir := os.Getenv("GA_PDF_DIR"); dir != "" {
		_ = os.WriteFile(filepath.Join(dir, "lageplan-karte.pdf"), pdf, 0o600)
	}

	// neues Bild ersetzt das alte (Datei wird gelöscht), Größe und Deckkraft bleiben
	rec = doMultipart(h, "/api/admin/lageplan/bild", "plan2.jpg", luftbildTest(800, 600))
	k = decode[LageKarte](t, rec)
	if k.Bild == erstes || k.Massstab != 0.5 || k.Deckkraft != 0.7 {
		t.Errorf("zweites Bild: %+v", k)
	}
	if _, err := os.Stat(filepath.Join(app.st.dir, karteOrdner, erstes)); !os.IsNotExist(err) {
		t.Errorf("altes Bild liegt noch da: %v", err)
	}
	// entfernen: Bild weg, Einrichtungen bleiben
	if rec := do(h, "DELETE", "/api/admin/lageplan/bild", nil); rec.Code != 200 {
		t.Fatalf("entfernen: %d", rec.Code)
	}
	st = decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if st.LageKarte.Bild != "" || len(st.LageKarte.Objekte) != 1 {
		t.Errorf("nach dem Entfernen: %+v", st.LageKarte)
	}
	if rec := do(h, "GET", "/api/lageplan-bild", nil); rec.Code != 404 {
		t.Errorf("Bild nach dem Entfernen: %d", rec.Code)
	}
}

func TestLuftbildUndOrtssuche(t *testing.T) {
	_, h := newTestApp(t)
	var anfrage string
	bild := luftbildTest(600, 450)
	leer := testBild(t, 600, 450, true) // fast einfarbig: außerhalb des Landes
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/wms"):
			anfrage = r.URL.RawQuery
			w.Header().Set("Content-Type", "image/jpeg")
			if r.URL.Path == "/wms-leer" {
				_, _ = w.Write(leer)
				return
			}
			if r.URL.Path == "/wms-fehler" {
				_, _ = w.Write([]byte("<ServiceException>Layer unbekannt</ServiceException>"))
				return
			}
			_, _ = w.Write(bild)
		case r.URL.Path == "/suche":
			if !strings.Contains(r.Header.Get("User-Agent"), appName) {
				w.WriteHeader(403)
				return
			}
			_, _ = w.Write([]byte(`[{"display_name":"Gartenweg 1, Erfurt, Thüringen","lat":"50.98","lon":"11.03","address":{"ISO3166-2-lvl4":"DE-TH"}}]`))
		}
	}))
	defer srv.Close()
	alterClient, alteSuche := luftbildClient, ortssucheURL
	luftbildClient, ortssucheURL = srv.Client(), srv.URL+"/suche"
	luftbildDienste["test"] = luftbildDienst{"Testland", srv.URL + "/wms", "dop", "© Test", 0.2}
	luftbildDienste["testleer"] = luftbildDienst{"Testland", srv.URL + "/wms-leer", "dop", "© Test", 0.2}
	luftbildDienste["testfehler"] = luftbildDienst{"Testland", srv.URL + "/wms-fehler", "dop", "© Test", 0.2}
	defer func() {
		luftbildClient, ortssucheURL = alterClient, alteSuche
		delete(luftbildDienste, "test")
		delete(luftbildDienste, "testleer")
		delete(luftbildDienste, "testfehler")
	}()

	// Ortssuche: Anschrift über den Suchdienst, Koordinaten direkt
	treffer := decode[[]ortTreffer](t, do(h, "GET", "/api/admin/lageplan/ortssuche?q=Gartenweg+1+Erfurt", nil))
	if len(treffer) != 1 || treffer[0].Land != "th" || treffer[0].Lat != 50.98 {
		t.Fatalf("Ortssuche: %+v", treffer)
	}
	for _, q := range []string{"50.98, 11.03", "https://www.google.com/maps/@50.98,11.03,17z", "https://www.openstreetmap.org/?mlat=50.98&mlon=11.03#map=18"} {
		tr := decode[[]ortTreffer](t, do(h, "GET", "/api/admin/lageplan/ortssuche?q="+urlQuery(q), nil))
		if len(tr) != 1 || tr[0].Lat != 50.98 || tr[0].Lon != 11.03 {
			t.Errorf("Koordinaten aus %q: %+v", q, tr)
		}
	}

	auftrag := map[string]any{"land": "test", "lat": 50.98, "lon": 11.03, "breite": 400, "hoehe": 300}
	rec := do(h, "POST", "/api/admin/lageplan/luftbild", auftrag)
	if rec.Code != 200 {
		t.Fatalf("Luftbild: %d %s", rec.Code, rec.Body.String())
	}
	k := decode[LageKarte](t, rec)
	if k.Quelle != "© Test" || k.MeterJeBildpunkt <= 0 || k.Luftbild == nil || k.Luftbild.Land != "test" {
		t.Errorf("Karte nach Luftbild: %+v", k)
	}
	for _, teil := range []string{"REQUEST=GetMap", "CRS=EPSG%3A3857", "LAYERS=dop", "WIDTH=2000", "HEIGHT=1500"} {
		if !strings.Contains(anfrage, teil) {
			t.Errorf("WMS-Anfrage ohne %s: %s", teil, anfrage)
		}
	}
	// Fehlerfälle: leeres Bild, kein Bild, außerhalb Deutschlands, unbekanntes Land, eigener Dienst im eigenen Netz
	for name, a := range map[string]map[string]any{
		"leer":          {"land": "testleer", "lat": 50.98, "lon": 11.03, "breite": 400, "hoehe": 300},
		"Fehlermeldung": {"land": "testfehler", "lat": 50.98, "lon": 11.03, "breite": 400, "hoehe": 300},
		"Ausland":       {"land": "test", "lat": 40.0, "lon": 11.03, "breite": 400, "hoehe": 300},
		"zu groß":       {"land": "test", "lat": 50.98, "lon": 11.03, "breite": 5000, "hoehe": 300},
		"Land":          {"land": "xx", "lat": 50.98, "lon": 11.03, "breite": 400, "hoehe": 300},
		"eigen IP":      {"land": "eigen", "url": "https://192.168.1.1/wms", "layer": "a", "lat": 50.98, "lon": 11.03, "breite": 400, "hoehe": 300},
		"eigen http":    {"land": "eigen", "url": "http://example.org/wms", "layer": "a", "lat": 50.98, "lon": 11.03, "breite": 400, "hoehe": 300},
		"eigen Layer":   {"land": "eigen", "url": "https://example.org/wms", "layer": "a&REQUEST=x", "lat": 50.98, "lon": 11.03, "breite": 400, "hoehe": 300},
	} {
		if rec := do(h, "POST", "/api/admin/lageplan/luftbild", a); rec.Code != 400 {
			t.Errorf("%s: %d, erwartet 400", name, rec.Code)
		}
	}
	// nur für den Vorstand
	if rec := do(h, "GET", "/api/admin/lageplan/dienste", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Thüringen") {
		t.Errorf("Dienste: %d", rec.Code)
	}
}

func urlQuery(s string) string {
	r := strings.NewReplacer("%", "%25", "&", "%26", "#", "%23", "?", "%3F", " ", "+", "+", "%2B")
	return r.Replace(s)
}

func TestVerkleinernUndMitte(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1000, 10))
	out, f := verkleinern(img, 100)
	if out.Bounds().Dx() != 100 || out.Bounds().Dy() != 1 || f != 10 {
		t.Errorf("verkleinert: %v, Faktor %v", out.Bounds(), f)
	}
	x, y := umrissMitte([]Punkt{{0, 0}, {10, 0}, {10, 10}, {0, 10}})
	if x != 5 || y != 5 {
		t.Errorf("Mitte: %v %v", x, y)
	}
	x, _ = umrissMitte([]Punkt{{0, 0}, {10, 0}, {20, 0}})
	if x != 10 {
		t.Errorf("Mitte einer Linie: %v", x)
	}
}
