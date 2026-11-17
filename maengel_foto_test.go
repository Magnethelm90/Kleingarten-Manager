package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func farbBild(w, h int) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			im.Set(x, y, color.RGBA{uint8(x), uint8(y), 120, 255})
		}
	}
	return im
}

func TestFotoVerarbeiten(t *testing.T) {
	var p bytes.Buffer
	_ = png.Encode(&p, farbBild(3000, 1000))
	out, err := fotoVerarbeiten(p.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	cfg, f, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil || f != "jpeg" || cfg.Width != fotoMaxKante || cfg.Height != 533 {
		t.Fatalf("Ergebnis: %v %v %+v", err, f, cfg)
	}
	if _, err := fotoVerarbeiten([]byte("kein Bild")); err == nil {
		t.Error("Unsinn angenommen")
	}
	// Kameradrehung 6 (90° im Uhrzeigersinn): aus 40x20 wird 20x40
	var j bytes.Buffer
	_ = jpeg.Encode(&j, farbBild(40, 20), nil)
	b := j.Bytes()
	exif := []byte{0xFF, 0xE1, 0, 0x22, 'E', 'x', 'i', 'f', 0, 0, 'M', 'M', 0, 0x2A, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 6, 0, 0, 0, 0, 0, 0}
	mit := append(append([]byte{0xFF, 0xD8}, exif...), b[2:]...)
	if exifDrehung(mit) != 6 {
		t.Fatalf("Drehung: %d", exifDrehung(mit))
	}
	out, err = fotoVerarbeiten(mit)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, _ = image.DecodeConfig(bytes.NewReader(out))
	if cfg.Width != 20 || cfg.Height != 40 {
		t.Errorf("nicht gedreht: %+v", cfg)
	}
}

func TestMangelFoto(t *testing.T) {
	a, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "7", "name": "Gerd Sieben", "gartennr": "7", "gartengroesse": 100}))
	m := decode[Mangel](t, do(h, "POST", "/api/admin/maengel", map[string]any{"gartennr": "7", "datum": "2026-05-01", "art": "Hecke und Bäume", "beschreibung": "zu hoch"}))
	var pb bytes.Buffer
	_ = png.Encode(&pb, farbBild(200, 100))
	url := "/api/admin/maengel/" + m.ID + "/foto"
	if c := doMultipart(h, url, "x.exe", pb.Bytes()).Code; c != 400 {
		t.Errorf("falsche Endung: %d", c)
	}
	if c := doMultipart(h, url, "x.jpg", []byte("Text")).Code; c != 400 {
		t.Errorf("falscher Inhalt: %d", c)
	}
	if c := doMultipart(h, url, "x.png", pb.Bytes()).Code; c != 200 {
		t.Fatalf("Upload: %d", c)
	}
	if rec := do(h, "GET", url, nil); rec.Code != 200 || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("Abruf: %d", rec.Code)
	}
	// Bearbeiten darf das Foto nicht verlieren
	do(h, "PUT", "/api/admin/maengel/"+m.ID, map[string]any{"gartennr": "7", "datum": "2026-05-01", "art": "Hecke und Bäume", "beschreibung": "immer noch zu hoch"})
	a.st.mu.Lock()
	foto := a.st.d.Maengel[0].Foto
	a.st.mu.Unlock()
	if foto == "" {
		t.Fatal("Foto nach Bearbeiten weg")
	}
	// Schreiben mit Foto
	if rec := do(h, "GET", "/api/admin/maengelschreiben?gartennr=7", nil); rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) || !bytes.Contains(rec.Body.Bytes(), []byte("/Image")) {
		t.Errorf("PDF: %d", rec.Code)
	}
	datei := filepath.Join(a.st.dir, filepath.FromSlash(foto))
	// Löschen: Datei bleibt im Papierkorb, geht beim Leeren
	do(h, "DELETE", "/api/admin/maengel/"+m.ID, nil)
	a.st.mu.Lock()
	a.st.fotosAufraeumenLocked()
	a.st.mu.Unlock()
	if _, err := os.Stat(datei); err != nil {
		t.Fatal("Foto im Papierkorb gelöscht")
	}
	do(h, "DELETE", "/api/admin/papierkorb", nil)
	if _, err := os.Stat(datei); err == nil {
		t.Error("Foto nach Leeren noch da")
	}
	_ = p
}
