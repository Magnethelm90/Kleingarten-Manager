package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tresorHochladen(h interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}, datei []byte, passwort string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "x.kgsicherung")
	_, _ = fw.Write(datei)
	_ = mw.WriteField("passwort", passwort)
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/admin/sicherung-entschluesseln", &buf)
	req.Host = "127.0.0.1:8765"
	req.Header.Set("X-GA-Request", "1")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestVerschluesselteSicherung(t *testing.T) {
	alt := revealPathFn
	revealPathFn = func(string) {}
	defer func() { revealPathFn = alt }()
	a, h := newTestApp(t)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Geheimer Name", "gartengroesse": 100})

	if rec := do(h, "POST", "/api/admin/sicherung-verschluesselt", map[string]string{"passwort": "kurz"}); rec.Code != 400 {
		t.Fatalf("kurzes Passwort: %d", rec.Code)
	}
	const pw = "Gartenzwerg-2026"
	rec := do(h, "POST", "/api/admin/sicherung-verschluesselt", map[string]string{"passwort": pw})
	if rec.Code != 200 {
		t.Fatalf("erstellen: %d %s", rec.Code, rec.Body)
	}
	name := decode[map[string]any](t, rec)["datei"].(string)
	datei, err := os.ReadFile(filepath.Join(a.st.dir, tresorOrdner, name))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(datei, []byte("Geheimer Name")) || bytes.Contains(datei, []byte("paechter")) {
		t.Fatal("die verschlüsselte Datei enthält lesbaren Inhalt")
	}

	// falsches Passwort und veränderte Datei werden abgelehnt, ohne etwas abzulegen
	vorher := len(a.st.sicherungsNamen())
	if rec := tresorHochladen(h, datei, "falsches-passwort"); rec.Code != 400 {
		t.Errorf("falsches Passwort: %d", rec.Code)
	}
	kaputt := bytes.Clone(datei)
	kaputt[len(kaputt)-5] ^= 1
	if rec := tresorHochladen(h, kaputt, pw); rec.Code != 400 {
		t.Errorf("veränderte Datei: %d", rec.Code)
	}
	if rec := tresorHochladen(h, []byte("irgendwas"), pw); rec.Code != 400 {
		t.Errorf("fremde Datei: %d", rec.Code)
	}
	if len(a.st.sicherungsNamen()) != vorher {
		t.Error("abgelehnte Dateien dürfen keine Sicherung anlegen")
	}

	// richtiges Passwort: liegt danach als gewöhnliche Sicherung vor; der heutige Stand bleibt unberührt
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "2", "name": "Später", "gartengroesse": 100})
	rec = tresorHochladen(h, datei, pw)
	if rec.Code != 200 {
		t.Fatalf("öffnen: %d %s", rec.Code, rec.Body)
	}
	abgelegt := decode[map[string]any](t, rec)["name"].(string)
	if got := strings.Join(paechterNamen(a), ","); !strings.Contains(got, "Später") {
		t.Errorf("Öffnen darf den heutigen Stand nicht ändern: %s", got)
	}
	if rec := do(h, "POST", "/api/admin/sicherungen/wiederherstellen", map[string]string{"name": abgelegt}); rec.Code != 200 {
		t.Fatalf("wiederherstellen: %d %s", rec.Code, rec.Body)
	}
	if got := strings.Join(paechterNamen(a), ","); got != "Geheimer Name" {
		t.Errorf("nach dem Wiederherstellen: %s", got)
	}

	// endgültiges Löschen einer Person entfernt die verschlüsselten Dateien im Datenordner
	if n := a.st.entferneTresore(); n != 1 {
		t.Errorf("entfernt: %d", n)
	}
}

// Zwei Verschlüsselungen desselben Inhalts unterscheiden sich (frisches Salz und Nonce).
func TestTresorZufall(t *testing.T) {
	x, _ := tresorVerschluesseln([]byte("inhalt"), "passwort-lang")
	y, _ := tresorVerschluesseln([]byte("inhalt"), "passwort-lang")
	if bytes.Equal(x, y) {
		t.Fatal("gleiche Ausgabe")
	}
	if k, err := tresorEntschluesseln(x, "passwort-lang"); err != nil || string(k) != "inhalt" {
		t.Fatalf("%q %v", k, err)
	}
}
