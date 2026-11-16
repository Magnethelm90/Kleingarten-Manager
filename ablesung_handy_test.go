package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func handyHochladen(h interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}, inhalt string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "e.json")
	_, _ = fw.Write([]byte(inhalt))
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/admin/ablesung-handy/import", &buf)
	req.Host = "127.0.0.1:8765"
	req.Header.Set("X-GA-Request", "1")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAblesungHandy(t *testing.T) {
	a, h := newTestApp(t)
	if rec := do(h, "GET", "/api/admin/ablesung-handy", nil); rec.Code != 400 {
		t.Fatalf("ohne Pächter: %d", rec.Code)
	}
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "</script><b>Eve", "gartennr": "3", "gartengroesse": 100, "wasserzaehlerNr": "W1"}))
	q := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "2", "name": "Bob", "gartennr": "4", "gartengroesse": 100}))
	jahr := a.st.d.Settings.Jahr
	a.st.mu.Lock()
	v := 100.0
	ab := a.st.d.Jahre[yearKey(jahr)].Ablesungen[p.ID]
	ab.WasserVJ = &v
	a.st.d.Jahre[yearKey(jahr)].Ablesungen[p.ID] = ab
	a.st.mu.Unlock()

	rec := do(h, "GET", "/api/admin/ablesung-handy", nil)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("Datei: %d", rec.Code)
	}
	if strings.Contains(body, "</script><b>Eve") || strings.Contains(body, "/*DATEN*/") || !strings.Contains(body, `</script>`) {
		t.Error("Daten müssen maskiert in die Seite eingebettet sein")
	}
	if strings.Count(body, "<script") != 1 {
		t.Error("genau ein Skript erwartet")
	}

	ok := `{"version":1,"jahr":` + strconv.Itoa(jahr) + `,"staende":[{"id":"` + p.ID + `","wasser":90.5,"strom":null},{"id":"` + q.ID + `","wasser":null,"strom":12}]}`
	r := decode[handyBericht](t, handyHochladen(h, ok))
	if r.Uebernommen != 2 || len(r.Hinweise) != 1 || !strings.Contains(r.Hinweise[0], "Garten 3") {
		t.Fatalf("Import: %+v", r)
	}
	a.st.mu.Lock()
	gew := a.st.d.Jahre[yearKey(jahr)].Ablesungen[p.ID]
	a.st.mu.Unlock()
	if gew.WasserAkt == nil || *gew.WasserAkt != 90.5 || gew.WasserVJ == nil || *gew.WasserVJ != 100 {
		t.Errorf("Wasser: %+v", gew)
	}
	// zweites Einlesen ändert nichts
	if r := decode[handyBericht](t, handyHochladen(h, ok)); r.Uebernommen != 0 || r.Unveraendert != 2 {
		t.Errorf("erneut: %+v", r)
	}
	// ungültige Dateien: nichts wird übernommen
	for _, schlecht := range []string{`kaputt`, `{"version":2}`, `{"version":1,"jahr":1999,"staende":[]}`,
		`{"version":1,"jahr":` + strconv.Itoa(jahr) + `,"staende":[{"id":"gibtesnicht","wasser":1}]}`,
		`{"version":1,"jahr":` + strconv.Itoa(jahr) + `,"staende":[{"id":"` + p.ID + `","wasser":-5}]}`} {
		if rec := handyHochladen(h, schlecht); rec.Code != 400 {
			t.Errorf("%q: %d", schlecht, rec.Code)
		}
	}
}
