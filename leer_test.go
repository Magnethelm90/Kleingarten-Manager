package main

import (
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Ein frischer, leerer Verein: keine Lese-Route darf »null« statt einer leeren Liste liefern,
// sonst stürzt die Oberfläche bei ».length« oder ».map« ab.
func TestLeererVereinOhneNull(t *testing.T) {
	src, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`HandleFunc\("GET (/api/[^"{}]+)",`)
	_, h := newTestApp(t)
	feld := regexp.MustCompile(`"[A-Za-z]+":null`)
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		rec := do(h, "GET", m[1], nil)
		if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
			continue
		}
		body := strings.TrimSpace(rec.Body.String())
		if body == "null" {
			t.Errorf("GET %s liefert null", m[1])
		}
		for _, f := range feld.FindAllString(body, -1) {
			t.Logf("GET %s: %s", m[1], f)
		}
	}
}

func TestMonateDazu(t *testing.T) {
	for _, c := range [][3]string{{"2026-01-31", "1", "2026-02-28"}, {"2028-01-31", "1", "2028-02-29"}, {"2026-03-31", "-1", "2026-02-28"},
		{"2026-12-31", "-3", "2026-09-30"}, {"2026-01-15", "13", "2027-02-15"}, {"2026-08-31", "6", "2027-02-28"}} {
		d, _ := parseDate(c[0])
		n, _ := strconv.Atoi(c[1])
		if got := monateDazu(d, n).Format("2006-01-02"); got != c[2] {
			t.Errorf("%s + %s Monate = %s, erwartet %s", c[0], c[1], got, c[2])
		}
	}
	// Monatliche Verlängerung darf vom Monatsende nicht wegwandern
	v := Vertrag{Ende: "2025-01-31", Verlaengert: 1}
	if got := v.ansicht(time.Date(2025, 4, 15, 0, 0, 0, 0, time.UTC)).LaeuftBis; got != "2025-04-30" {
		t.Errorf("Verlängerung: %s, erwartet 2025-04-30", got)
	}
}

func TestProgrammDateienVollstaendig(t *testing.T) {
	sub, _ := fs.Sub(webFS, "web")
	l := programmDateien(sub)
	if len(l) < 12 {
		t.Fatalf("index.html bindet zu wenig ein: %v", l)
	}
	for _, n := range l {
		if _, err := fs.Stat(sub, n); err != nil {
			t.Errorf("%s fehlt", n)
		}
	}
}
