package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
)

// Statische Oberfläche: Jede Datei wird einmal gelesen, mit ETag versehen und gzip-komprimiert im Speicher gehalten.
// Der Browser fragt bei jedem Start nach (no-cache), bekommt aber bei unveränderter Datei nur »304 Not Modified«
// und sonst die komprimierte Fassung (app.js: ~490 KB → ~110 KB). Die Dateien sind fest ins Programm eingebettet,
// der Zwischenspeicher bleibt deshalb immer gültig.

type statischeDatei struct {
	typ, etag string
	roh, gz   []byte
}

type statischerServer struct {
	fsys    fs.FS
	weiter  http.Handler // alles, was nicht zwischengespeichert wird (Bilder, Verzeichnisse)
	mu      sync.Mutex
	dateien map[string]*statischeDatei
}

func neuerStatischerServer(fsys fs.FS, weiter http.Handler) *statischerServer {
	return &statischerServer{fsys: fsys, weiter: weiter, dateien: map[string]*statischeDatei{}}
}

func (s *statischerServer) laden(name string) *statischeDatei {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.dateien[name]; ok {
		return d
	}
	roh, err := fs.ReadFile(s.fsys, name)
	if err != nil {
		s.dateien[name] = nil
		return nil
	}
	sum := sha256.Sum256(roh)
	d := &statischeDatei{roh: roh, etag: `"` + hex.EncodeToString(sum[:10]) + `"`, typ: mime.TypeByExtension(path.Ext(name))}
	if d.typ == "" {
		d.typ = "application/octet-stream"
	}
	var buf bytes.Buffer
	if zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression); err == nil {
		_, _ = zw.Write(roh)
		if zw.Close() == nil && buf.Len() < len(roh) {
			d.gz = buf.Bytes()
		}
	}
	s.dateien[name] = d
	return d
}

func (s *statischerServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	switch path.Ext(name) {
	case ".js", ".css", ".html", ".svg":
	default:
		s.weiter.ServeHTTP(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		s.weiter.ServeHTTP(w, r)
		return
	}
	d := s.laden(name)
	if d == nil {
		s.weiter.ServeHTTP(w, r)
		return
	}
	h := w.Header()
	h.Set("Cache-Control", "no-cache")
	h.Set("ETag", d.etag)
	h.Set("Vary", "Accept-Encoding")
	if r.Header.Get("If-None-Match") == d.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Type", d.typ)
	body := d.roh
	if d.gz != nil && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		h.Set("Content-Encoding", "gzip")
		body = d.gz
	}
	h.Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}
