package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Mängel mit Foto: Je Mangel ein Bild, abgelegt im Datenordner unter »Maengelfotos«. Beim Hochladen wird das Bild neu
// kodiert: Das entfernt Standortdaten und Kameradaten (EXIF), richtet es nach der Kameradrehung aus und begrenzt die
// Größe. Das Foto erscheint im Schreiben an den Pächter. Es soll nur den Mangel zeigen, nicht Personen.

const (
	fotoOrdner   = "Maengelfotos"
	fotoMaxKante = 1600
	fotoMaxUpl   = 12 << 20
)

// fotoDatei prüft den gespeicherten Pfad und liefert den vollständigen Dateinamen.
func (s *Store) fotoDatei(rel string) (string, bool) {
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, fotoOrdner+"/") || strings.Contains(rel, "..") || path.Ext(rel) != ".jpg" || strings.Count(rel, "/") != 1 {
		return "", false
	}
	return filepath.Join(s.dir, filepath.FromSlash(rel)), true
}

// exifDrehung liest die Ausrichtung (1–8) aus einem JPEG; 1 = keine Drehung nötig.
func exifDrehung(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(b) && b[i] == 0xFF {
		m := b[i+1]
		if m == 0xDA || m == 0xD9 {
			break
		}
		n := int(binary.BigEndian.Uint16(b[i+2 : i+4]))
		if n < 2 || i+2+n > len(b) {
			break
		}
		seg := b[i+4 : i+2+n]
		if m == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			t := seg[6:]
			var bo binary.ByteOrder = binary.BigEndian
			if t[0] == 'I' {
				bo = binary.LittleEndian
			}
			off := int(bo.Uint32(t[4:8]))
			if off < 8 || off+2 > len(t) {
				return 1
			}
			cnt := int(bo.Uint16(t[off : off+2]))
			for k := 0; k < cnt; k++ {
				e := off + 2 + k*12
				if e+12 > len(t) {
					break
				}
				if bo.Uint16(t[e:e+2]) == 0x0112 {
					if v := int(bo.Uint16(t[e+8 : e+10])); v >= 1 && v <= 8 {
						return v
					}
				}
			}
		}
		i += 2 + n
	}
	return 1
}

// fotoVerarbeiten dekodiert, dreht, verkleinert und kodiert das Bild neu als JPEG.
func fotoVerarbeiten(data []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width*cfg.Height > 60_000_000 {
		return nil, bad("Das Bild lässt sich nicht lesen. Bitte ein JPG- oder PNG-Foto wählen")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, bad("Das Bild lässt sich nicht lesen. Bitte ein JPG- oder PNG-Foto wählen")
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	drehung := exifDrehung(data)
	tauscht := drehung >= 5
	ow, oh := w, h
	if tauscht {
		ow, oh = h, w
	}
	faktor := 1.0
	if m := max(ow, oh); m > fotoMaxKante {
		faktor = float64(fotoMaxKante) / float64(m)
	}
	nw, nh := max(1, int(float64(ow)*faktor)), max(1, int(float64(oh)*faktor))
	out := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.Draw(out, out.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			// Zielpunkt → Punkt im ausgerichteten Bild → Punkt im Original
			ax, ay := int(float64(x)/faktor), int(float64(y)/faktor)
			ax, ay = min(ax, ow-1), min(ay, oh-1)
			sx, sy := ax, ay
			switch drehung {
			case 3:
				sx, sy = w-1-ax, h-1-ay
			case 6:
				sx, sy = ay, h-1-ax
			case 8:
				sx, sy = w-1-ay, ax
			}
			sx, sy = min(max(sx, 0), w-1), min(max(sy, 0), h-1)
			out.Set(x, y, img.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *App) handleMangelFotoHochladen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	r.Body = http.MaxBytesReader(w, r.Body, fotoMaxUpl+(1<<20))
	if err := r.ParseMultipartForm(fotoMaxUpl); err != nil {
		writeErr(w, bad("Die Datei ist zu groß oder konnte nicht gelesen werden (höchstens 12 MB)"))
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, bad("Bitte ein Foto auswählen"))
		return
	}
	defer file.Close()
	if ext := strings.ToLower(filepath.Ext(hdr.Filename)); ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		writeErr(w, bad("Bitte ein Foto als JPG oder PNG auswählen"))
		return
	}
	data, err := io.ReadAll(file)
	if err != nil {
		writeErr(w, bad("Datei konnte nicht gelesen werden"))
		return
	}
	jpg, err := fotoVerarbeiten(data)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	idx := -1
	for i := range a.st.d.Maengel {
		if a.st.d.Maengel[i].ID == id {
			idx = i
		}
	}
	if idx < 0 {
		writeErr(w, notFound("Mangel nicht gefunden"))
		return
	}
	dir := filepath.Join(a.st.dir, fotoOrdner)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		writeErr(w, err)
		return
	}
	name := safeName(id, 20) + "-" + newID() + ".jpg"
	if err := os.WriteFile(filepath.Join(dir, name), jpg, 0o600); err != nil {
		writeErr(w, err)
		return
	}
	a.st.d.Maengel[idx].Foto = fotoOrdner + "/" + name
	a.st.audit("Foto zu Mangel hinterlegt: Garten %s", a.st.d.Maengel[idx].Gartennr)
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	a.st.fotosAufraeumenLocked()
	writeJSON(w, 200, map[string]string{"foto": a.st.d.Maengel[idx].Foto})
}

func (a *App) handleMangelFotoLoeschen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Maengel {
		if a.st.d.Maengel[i].ID == id {
			a.st.d.Maengel[i].Foto = ""
			a.st.audit("Foto zu Mangel entfernt: Garten %s", a.st.d.Maengel[i].Gartennr)
			if err := a.st.saveLocked(); err != nil {
				writeErr(w, err)
				return
			}
			a.st.fotosAufraeumenLocked()
			writeJSON(w, 200, map[string]bool{"ok": true})
			return
		}
	}
	writeErr(w, notFound("Mangel nicht gefunden"))
}

func (a *App) handleMangelFoto(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	var rel string
	for _, m := range a.st.d.Maengel {
		if m.ID == id {
			rel = m.Foto
		}
	}
	full, ok := a.st.fotoDatei(rel)
	a.st.mu.Unlock()
	if rel == "" || !ok {
		writeErr(w, notFound("Zu diesem Mangel gibt es kein Foto"))
		return
	}
	fi, err := os.Lstat(full)
	if err != nil || !fi.Mode().IsRegular() {
		writeErr(w, notFound("Die Fotodatei fehlt"))
		return
	}
	f, err := os.Open(full)
	if err != nil {
		writeErr(w, notFound("Die Fotodatei fehlt"))
		return
	}
	defer f.Close()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeContent(w, r, "foto.jpg", fi.ModTime(), f)
}

// fotosAufraeumenLocked löscht Fotodateien, die kein Mangel (auch keiner im Papierkorb) mehr verwendet.
// Das gilt nach dem Entfernen eines Fotos, dem Leeren des Papierkorbs und dem endgültigen Löschen einer Person.
func (s *Store) fotosAufraeumenLocked() {
	dir := filepath.Join(s.dir, fotoOrdner)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	benutzt := map[string]bool{}
	for _, m := range s.d.Maengel {
		if m.Foto != "" {
			benutzt[path.Base(filepath.ToSlash(m.Foto))] = true
		}
	}
	for _, e := range s.d.Papierkorb {
		if e.Art != "mangel" {
			continue
		}
		var m Mangel
		if json.Unmarshal(e.Daten, &m) == nil && m.Foto != "" {
			benutzt[path.Base(filepath.ToSlash(m.Foto))] = true
		}
	}
	for _, e := range entries {
		if e.Type().IsRegular() && !benutzt[e.Name()] {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// fotoLaden liest das Foto eines Mangels für das Schreiben; nil, wenn keins da ist.
func (s *Store) fotoLaden(m Mangel) []byte {
	full, ok := s.fotoDatei(m.Foto)
	if !ok {
		return nil
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return nil
	}
	return b
}
