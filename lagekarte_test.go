package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// planBildTest erzeugt ein Bild mit Struktur (wie ein eingescannter Plan).
func planBildTest(w, h int) []byte {
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
	rec := doMultipart(h, "/api/admin/lageplan/bild", "plan.jpg", planBildTest(4000, 2000))
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
	rec = doMultipart(h, "/api/admin/lageplan/bild", "plan2.jpg", planBildTest(800, 600))
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
