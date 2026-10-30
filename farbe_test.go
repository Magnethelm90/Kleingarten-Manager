package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testBild(t *testing.T, w, h int, alsJPEG bool) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, h/2, color.RGBA{30, 90, 168, 255})
	}
	var b bytes.Buffer
	var err error
	if alsJPEG {
		err = jpeg.Encode(&b, img, nil)
	} else {
		err = png.Encode(&b, img)
	}
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestMarkenfarbeBleibtLesbar(t *testing.T) {
	for _, f := range []string{"", "#2f6f3e", "#ffe066", "#ffffff", "#00ff00", "kaputt"} {
		r, g, b := markenfarbe(f)
		if k := 1.05 / (relHelligkeit(float64(r), float64(g), float64(b)) + 0.05); k < 4.5 {
			t.Errorf("%q: Kontrast zu Weiß nur %.2f", f, k)
		}
	}
	if r, g, b := markenfarbe("#1e5aa8"); r != 0x1e || g != 0x5a || b != 0xa8 {
		t.Errorf("ausreichend dunkle Farbe darf nicht verändert werden: %d %d %d", r, g, b)
	}
}

func TestVereinsfarbeEinstellung(t *testing.T) {
	_, h := newTestApp(t)
	s := decode[stateResp](t, do(h, "GET", "/api/state", nil)).Settings
	s.Farbe = "#1E5AA8"
	if rec := do(h, "PUT", "/api/admin/settings", s); rec.Code != 200 {
		t.Fatalf("Farbe speichern: %d %s", rec.Code, rec.Body.String())
	}
	if got := decode[stateResp](t, do(h, "GET", "/api/state", nil)).Settings.Farbe; got != "#1e5aa8" {
		t.Errorf("gespeicherte Farbe: %q", got)
	}
	s.Farbe = "rot"
	if rec := do(h, "PUT", "/api/admin/settings", s); rec.Code != 400 {
		t.Errorf("ungültige Farbe: %d", rec.Code)
	}
}

func TestVereinslogo(t *testing.T) {
	app, h := newTestApp(t)
	// keine Bilder, zu groß, kaputt: abgelehnt
	for name, inhalt := range map[string][]byte{
		"text.png":   []byte("kein Bild"),
		"riesig.png": testBild(t, maxLogoPixel+1, 20, false),
		"winzig.png": testBild(t, 4, 4, false),
	} {
		if rec := doMultipart(h, "/api/admin/logo", name, inhalt); rec.Code != 400 {
			t.Errorf("%s: %d, erwartet 400", name, rec.Code)
		}
	}
	// JPEG wird angenommen und als PNG abgelegt
	rec := doMultipart(h, "/api/admin/logo", "logo.jpg", testBild(t, 300, 120, true))
	if rec.Code != 200 {
		t.Fatalf("Logo hochladen: %d %s", rec.Code, rec.Body.String())
	}
	name := decode[map[string]string](t, rec)["logo"]
	if !logoNameGueltig(name) {
		t.Fatalf("Dateiname %q", name)
	}
	raw, err := os.ReadFile(filepath.Join(app.st.dir, logoOrdner, name))
	if err != nil || !bytes.HasPrefix(raw, []byte("\x89PNG")) {
		t.Fatalf("gespeichertes Logo: %v", err)
	}
	if rec := do(h, "GET", "/api/logo", nil); rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" {
		t.Errorf("Logo abrufen: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}

	// Speichern der Einstellungen lässt das Logo unangetastet (auch wenn das Feld fehlt oder manipuliert ist)
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	s := st.Settings
	s.Logo = "../../etc/passwd"
	do(h, "PUT", "/api/admin/settings", s)
	if got := decode[stateResp](t, do(h, "GET", "/api/state", nil)).Settings.Logo; got != name {
		t.Fatalf("Logo nach dem Speichern der Einstellungen: %q", got)
	}

	// das Logo erscheint auf der Rechnung, die ausgestellte Rechnung behält es nach dem Entfernen
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "A", "gartengroesse": 300}))
	do(h, "PUT", "/api/ablesung/"+p.ID, Ablesung{WasserVJ: fp(1), WasserAkt: fp(2), StromVJ: fp(1), StromAkt: fp(2), Stunden: fp(12)})
	do(h, "POST", "/api/invoices/issue", issueReq{})
	inv := decode[stateResp](t, do(h, "GET", "/api/state", nil)).Issued[p.ID]
	pdfMitLogo := do(h, "GET", "/api/archive/"+inv.ID+".pdf", nil).Body.Bytes()
	if n := strings.Count(string(pdfMitLogo), "/Subtype /Image"); n < 1 {
		t.Errorf("kein Logo in der Rechnung (%d Bilder)", n)
	}
	if rec := do(h, "DELETE", "/api/admin/logo", nil); rec.Code != 200 {
		t.Fatalf("Logo entfernen: %d", rec.Code)
	}
	if rec := do(h, "GET", "/api/logo", nil); rec.Code != 404 {
		t.Errorf("nach dem Entfernen: %d", rec.Code)
	}
	live := do(h, "GET", "/api/invoice/"+p.ID+".pdf", nil).Body.Bytes()
	if strings.Count(string(live), "/Subtype /Image") != 0 {
		t.Error("neue Rechnung ohne Logo enthält trotzdem ein Bild")
	}
	again := do(h, "GET", "/api/archive/"+inv.ID+".pdf", nil).Body.Bytes()
	if strings.Count(string(again), "/Subtype /Image") < 1 {
		t.Error("ausgestellte Rechnung hat ihr Logo verloren")
	}
}
