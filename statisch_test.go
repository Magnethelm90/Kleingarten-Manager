package main

import (
	"compress/gzip"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatischerServer(t *testing.T) {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		t.Fatal(err)
	}
	s := neuerStatischerServer(sub, http.FileServerFS(sub))
	get := func(pfad string, kopf map[string]string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", pfad, nil)
		for k, v := range kopf {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	roh := get("/app.js", nil)
	if roh.Code != 200 || roh.Header().Get("ETag") == "" || roh.Header().Get("Content-Encoding") != "" {
		t.Fatalf("roh: %d %v", roh.Code, roh.Header())
	}
	gz := get("/app.js", map[string]string{"Accept-Encoding": "gzip"})
	if gz.Header().Get("Content-Encoding") != "gzip" || gz.Body.Len() >= roh.Body.Len() {
		t.Fatalf("gzip: %v %d/%d", gz.Header(), gz.Body.Len(), roh.Body.Len())
	}
	zr, err := gzip.NewReader(gz.Body)
	if err != nil {
		t.Fatal(err)
	}
	entpackt, _ := io.ReadAll(zr)
	if string(entpackt) != roh.Body.String() {
		t.Fatal("entpackter Inhalt weicht ab")
	}
	if c := get("/app.js", map[string]string{"If-None-Match": roh.Header().Get("ETag")}); c.Code != 304 {
		t.Fatalf("erwartet 304, war %d", c.Code)
	}
	if i := get("/", nil); i.Code != 200 || i.Header().Get("Content-Type") == "" {
		t.Fatalf("Startseite: %d", i.Code)
	}
	if n := get("/gibt-es-nicht.js", nil); n.Code != 404 {
		t.Fatalf("fehlende Datei: %d", n.Code)
	}
	if v := get("/../../etc/passwd.js", nil); v.Code == 200 {
		t.Fatal("Pfad aus dem Verzeichnis heraus")
	}
}
