package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestApiZufall schickt an jede Route zufällige, aber zum Eingabetyp passende Daten (Grenzwerte, kaputte
// Daten, echte IDs aus den Beispieldaten). Keine Route darf abstürzen oder mit 500 antworten, und danach
// muss sich der Datenbestand verlustfrei speichern und wieder laden lassen.
// Läuft nur mit KM_FUZZ=1 (Anzahl Durchläufe je Route über KM_FUZZ_N, Standard 25).
func TestApiZufall(t *testing.T) {
	if os.Getenv("KM_FUZZ") == "" {
		t.Skip("nur mit KM_FUZZ=1")
	}
	n := 25
	if v, err := strconv.Atoi(os.Getenv("KM_FUZZ_N")); err == nil && v > 0 {
		n = v
	}
	seed := time.Now().UnixNano()
	if v, err := strconv.ParseInt(os.Getenv("KM_FUZZ_SEED"), 10, 64); err == nil {
		seed = v
	}
	t.Logf("Startwert KM_FUZZ_SEED=%d", seed)
	rnd := rand.New(rand.NewSource(seed))

	typen, eingaben := eingabeTypen(t)
	dir := t.TempDir()
	st, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{st: st, port: testPort, sessions: map[string]time.Time{}, psessions: map[string]time.Time{}}
	h := app.routes(http.NotFoundHandler())
	do(h, "POST", "/api/admin/beispieldaten", map[string]any{})

	src, _ := os.ReadFile("api.go")
	re := regexp.MustCompile(`HandleFunc\("([A-Z]+) (/[^"]*)", (.*)$`)
	type route struct{ methode, pfad, handler string }
	var routen []route
	for _, z := range strings.Split(string(src), "\n") {
		m := re.FindStringSubmatch(strings.TrimSpace(z))
		if m == nil || strings.HasSuffix(m[2], "/oeffnen") || strings.Contains(m[2], "open-") || m[2] == "/api/quit" || m[2] == "/api/admin/check-update" || m[2] == "/api/admin/password" {
			continue
		}
		hs := regexp.MustCompile(`a\.(handle\w+)`).FindAllStringSubmatch(m[3], -1)
		name := ""
		if len(hs) > 0 {
			name = hs[len(hs)-1][1]
		}
		routen = append(routen, route{m[1], m[2], name})
	}
	if len(routen) < 200 {
		t.Fatalf("nur %d Routen gefunden", len(routen))
	}

	ids := map[string]bool{}
	beobachtet := map[string][]any{}
	idsJeListe := map[string][]string{}
	var merke func(v any)
	merke = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, w := range x {
				switch w.(type) {
				case string, float64, bool:
					if len(beobachtet[k]) < 40 {
						beobachtet[k] = append(beobachtet[k], w)
					}
				}
				merke(w)
			}
		case []any:
			for _, w := range x {
				merke(w)
			}
		}
	}
	sammle := func() {
		idRe := regexp.MustCompile(`"(?:id|paechterId|gartennr|mitgliedsnr)":"([^"]{1,40})"`)
		for _, r := range routen {
			if r.methode == "GET" && !strings.Contains(r.pfad, "{") {
				body := do(h, "GET", r.pfad, nil).Body.Bytes()
				for _, m := range idRe.FindAllSubmatch(body, -1) {
					ids[string(m[1])] = true
					idsJeListe[r.pfad] = append(idsJeListe[r.pfad], string(m[1]))
				}
				var v any
				if json.Unmarshal(body, &v) == nil {
					merke(v)
				}
			}
		}
	}
	sammle()
	idListe := func() []string {
		l := make([]string, 0, len(ids))
		for k := range ids {
			l = append(l, k)
		}
		sort.Strings(l)
		return l
	}()
	t.Logf("%d Routen, %d bekannte IDs", len(routen), len(idListe))

	g := &zufallsWerte{rnd: rnd, typen: typen, ids: idListe, beobachtet: beobachtet}
	// passende IDs für {id}: aus der Liste, deren Pfad vor dem Platzhalter steht
	idsFuer := func(pfad string) []string {
		i := strings.Index(pfad, "/{")
		if i < 0 {
			return nil
		}
		basis := pfad[:i]
		switch basis {
		case "/api/ablesung", "/api/zaehler":
			basis = "/api/admin/paechter"
		case "/api/payment", "/api/invoice", "/api/archive", "/api/invoice-email":
			basis = "/api/archiv-alle"
		}
		return idsJeListe[basis]
	}
	ohneTyp := []string{}
	for _, rt := range routen {
		if _, ok := eingaben[rt.handler]; !ok && rt.methode != "GET" {
			ohneTyp = append(ohneTyp, rt.methode+" "+rt.pfad+" ("+rt.handler+")")
		}
	}
	t.Logf("%d schreibende Routen ohne erkannten Eingabetyp: %s", len(ohneTyp), strings.Join(ohneTyp, ", "))
	fehler := map[string]string{}
	erfolg := map[string]int{}
	param := regexp.MustCompile(`\{[a-zA-Z]+\}`)
	for runde := 0; runde < n; runde++ {
		for _, r := range rnd.Perm(len(routen)) {
			rt := routen[r]
			passend := idsFuer(rt.pfad)
			pfad := param.ReplaceAllStringFunc(rt.pfad, func(string) string {
				if len(passend) > 0 && rnd.Intn(4) > 0 {
					return url.PathEscape(passend[rnd.Intn(len(passend))])
				}
				return url.PathEscape(g.pfadTeil())
			})
			pfad += g.anfrage()
			var body []byte
			if typ, ok := eingaben[rt.handler]; ok && rt.methode != "GET" {
				body, _ = json.Marshal(g.wert(typ, 0))
			} else if rt.methode != "GET" {
				body = []byte("{}")
			}
			code, antwort, absturz := sicherAufrufen(h, rt.methode, pfad, body)
			if code < 300 {
				erfolg[rt.methode+" "+rt.pfad]++
			}
			if absturz != "" || code >= 500 {
				k := rt.methode + " " + rt.pfad
				if _, schon := fehler[k]; !schon {
					fehler[k] = fmt.Sprintf("%d %s\n  Pfad: %s\n  Eingabe: %.400s\n  %s", code, strings.TrimSpace(antwort), pfad, body, absturz)
				}
			}
		}
		if runde%5 == 4 {
			sammle() // neu angelegte Einträge ebenfalls treffen
		}
	}
	nie := []string{}
	for _, rt := range routen {
		if erfolg[rt.methode+" "+rt.pfad] == 0 && rt.methode != "GET" {
			nie = append(nie, rt.methode+" "+rt.pfad)
		}
	}
	t.Logf("%d von %d schreibenden Routen nie erfolgreich: %s", len(nie), len(routen), strings.Join(nie, ", "))
	for _, k := range sortierteSchluessel(fehler) {
		t.Errorf("%s -> %s", k, fehler[k])
	}

	// Danach: alles lesbar, speicherbar und verlustfrei wieder ladbar
	for _, rt := range routen {
		if rt.methode == "GET" && !strings.Contains(rt.pfad, "{") {
			if code, antwort, absturz := sicherAufrufen(h, "GET", rt.pfad, nil); code >= 500 || absturz != "" {
				t.Errorf("nach dem Zufallstest: GET %s -> %d %s %s", rt.pfad, code, antwort, absturz)
			}
		}
	}
	st.mu.Lock()
	if err := st.saveLocked(); err != nil {
		t.Fatalf("Speichern nach dem Zufallstest: %v", err)
	}
	vorher, _ := json.Marshal(st.d)
	st.mu.Unlock()
	st2, err := openStore(dir)
	if err != nil {
		t.Fatalf("Laden nach dem Zufallstest: %v", err)
	}
	nachher, _ := json.Marshal(st2.d)
	if !bytes.Equal(vorher, nachher) {
		var a, b map[string]any
		_ = json.Unmarshal(vorher, &a)
		_ = json.Unmarshal(nachher, &b)
		for k := range a {
			if !reflect.DeepEqual(a[k], b[k]) {
				x, _ := json.Marshal(a[k])
				y, _ := json.Marshal(b[k])
				if leer := map[string]bool{"null": true, "[]": true, "{}": true}; leer[string(x)] && leer[string(y)] {
					continue
				}
				i := 0
				for i < len(x) && i < len(y) && x[i] == y[i] {
					i++
				}
				von := max(0, i-300)
				t.Errorf("Feld %s ist nach Speichern und Laden anders:\n vorher: %.500s\n nachher: %.500s", k, x[von:], y[von:])
			}
		}
	}
}

func sortierteSchluessel(m map[string]string) []string {
	l := make([]string, 0, len(m))
	for k := range m {
		l = append(l, k)
	}
	sort.Strings(l)
	return l
}

func sicherAufrufen(h http.Handler, methode, pfad string, body []byte) (code int, antwort, absturz string) {
	defer func() {
		if e := recover(); e != nil {
			st := string(debug.Stack())
			if i := strings.Index(st, "panic("); i >= 0 {
				st = st[i:]
			}
			if len(st) > 1500 {
				st = st[:1500]
			}
			absturz = fmt.Sprintf("ABSTURZ: %v\n%s", e, st)
		}
	}()
	req := httptest.NewRequest(methode, pfad, bytes.NewReader(body))
	req.Host = "127.0.0.1:8765"
	req.Header.Set("X-GA-Request", "1")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	a := rec.Body.String()
	if len(a) > 300 {
		a = a[:300]
	}
	return rec.Code, a, ""
}

// eingabeTypen liest aus dem Quelltext alle Strukturen und für jeden Handler den Typ, den er per readJSON einliest.
func eingabeTypen(t *testing.T) (map[string]ast.Expr, map[string]ast.Expr) {
	fset := token.NewFileSet()
	dateien, _ := filepath.Glob("*.go")
	typen := map[string]ast.Expr{}
	eingaben := map[string]ast.Expr{}
	for _, d := range dateien {
		if strings.HasSuffix(d, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, d, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			switch x := decl.(type) {
			case *ast.GenDecl:
				for _, s := range x.Specs {
					if ts, ok := s.(*ast.TypeSpec); ok {
						typen[ts.Name.Name] = ts.Type
					}
				}
			case *ast.FuncDecl:
				if x.Body == nil {
					continue
				}
				lokal := map[string]ast.Expr{}
				ast.Inspect(x.Body, func(n ast.Node) bool {
					switch y := n.(type) {
					case *ast.ValueSpec:
						if y.Type != nil {
							for _, nm := range y.Names {
								lokal[nm.Name] = y.Type
							}
						}
					case *ast.CallExpr:
						if id, ok := y.Fun.(*ast.Ident); ok && id.Name == "readJSON" && len(y.Args) == 3 {
							arg := y.Args[2]
							if u, ok := arg.(*ast.UnaryExpr); ok {
								arg = u.X
							}
							if id2, ok := arg.(*ast.Ident); ok {
								if typ, ok := lokal[id2.Name]; ok {
									eingaben[x.Name.Name] = typ
								}
							}
						}
					}
					return true
				})
			}
		}
	}
	return typen, eingaben
}

type zufallsWerte struct {
	rnd        *rand.Rand
	typen      map[string]ast.Expr
	ids        []string
	beobachtet map[string][]any
}

var (
	zufallsTexte = []string{"", " ", "x", "Müller-Lüdenscheidt", "2026-02-30", "2026-13-01", "0000-01-01", "9999-12-31", "2025-12-31",
		"2026-02-28", "2028-02-29", "../x", "..\\..\\x", "<b>fett</b>", "a\x00b", "Zeile1\nZeile2", "😀 Garten", "  Leerzeichen  ",
		"1", "2", "0", "-1", "abc", "1,5", "1.000,50", "DE89370400440532013000", "x@example.org", "Emailsendung", "Postversand",
		"monatlich", "jährlich", "einmalig", "ja", "nein", "unbekannt", "Laube", "Versammlung", "Arbeitseinsatz", "wasser", "strom"}
	zufallsZahlen = []float64{0, 1, -1, 0.5, -0.01, 12.345, 2026, 100, 1e9, -1e9, 1e15, 99999.995, 3}
)

func (g *zufallsWerte) text() string {
	switch g.rnd.Intn(10) {
	case 0, 1, 2:
		if len(g.ids) > 0 {
			return g.ids[g.rnd.Intn(len(g.ids))]
		}
	case 3:
		return time.Now().AddDate(0, 0, g.rnd.Intn(800)-400).Format("2006-01-02")
	case 4:
		return strings.Repeat("Langer Text ", 300+g.rnd.Intn(500))
	}
	return zufallsTexte[g.rnd.Intn(len(zufallsTexte))]
}

func (g *zufallsWerte) pfadTeil() string {
	if g.rnd.Intn(5) > 0 && len(g.ids) > 0 {
		return g.ids[g.rnd.Intn(len(g.ids))]
	}
	return []string{"x", "0", "-1", "..", "999999", "a b"}[g.rnd.Intn(6)]
}

func (g *zufallsWerte) anfrage() string {
	var teile []string
	if g.rnd.Intn(2) == 0 {
		teile = append(teile, "year="+[]string{"2026", "2025", "2027", "0", "-5", "99999", "abc", "1900"}[g.rnd.Intn(8)])
	}
	if g.rnd.Intn(3) == 0 {
		teile = append(teile, "art="+[]string{"brief", "aushang", "fragebogen", "erfassung", "auswertung", "aufnahmeantrag", "x", ""}[g.rnd.Intn(8)])
	}
	if g.rnd.Intn(4) == 0 {
		teile = append(teile, "alle=1", "anwenden=1", "q="+[]string{"Mül", "a", "", "%25", "x"}[g.rnd.Intn(5)])
	}
	if len(teile) == 0 {
		return ""
	}
	return "?" + strings.Join(teile, "&")
}

func (g *zufallsWerte) wert(typ ast.Expr, tiefe int) any {
	if tiefe > 4 {
		return nil
	}
	switch x := typ.(type) {
	case *ast.Ident:
		switch x.Name {
		case "string":
			return g.text()
		case "float64", "float32":
			return zufallsZahlen[g.rnd.Intn(len(zufallsZahlen))]
		case "int", "int64", "int32":
			return []int{0, 1, -1, 2, 3, 12, 60, 2026, 1900, 100000}[g.rnd.Intn(10)]
		case "bool":
			return g.rnd.Intn(2) == 0
		case "any":
			return nil
		}
		if s, ok := g.typen[x.Name]; ok {
			return g.wert(s, tiefe+1)
		}
		return nil
	case *ast.StarExpr:
		if g.rnd.Intn(4) == 0 {
			return nil
		}
		return g.wert(x.X, tiefe)
	case *ast.ArrayType:
		l := []any{}
		for i, k := 0, g.rnd.Intn(4); i < k; i++ {
			l = append(l, g.wert(x.Elt, tiefe+1))
		}
		if g.rnd.Intn(15) == 0 {
			for i := 0; i < 300; i++ {
				l = append(l, g.wert(x.Elt, tiefe+1))
			}
		}
		return l
	case *ast.MapType:
		m := map[string]any{}
		for i, k := 0, g.rnd.Intn(3); i < k; i++ {
			m[g.text()] = g.wert(x.Value, tiefe+1)
		}
		return m
	case *ast.StructType:
		m := map[string]any{}
		for _, f := range x.Fields.List {
			name := ""
			if f.Tag != nil {
				tag := reflect.StructTag(strings.Trim(f.Tag.Value, "`")).Get("json")
				name = strings.Split(tag, ",")[0]
				if name == "-" {
					continue
				}
			}
			if len(f.Names) == 0 { // eingebettet
				if eb, ok := g.wert(f.Type, tiefe).(map[string]any); ok {
					for k, v := range eb {
						m[k] = v
					}
				}
				continue
			}
			if g.rnd.Intn(8) == 0 {
				continue // Feld weglassen
			}
			for _, nm := range f.Names {
				k := name
				if k == "" {
					k = nm.Name
				}
				if l := g.beobachtet[k]; len(l) > 0 && g.rnd.Intn(10) < 7 {
					if _, liste := f.Type.(*ast.ArrayType); !liste {
						m[k] = l[g.rnd.Intn(len(l))]
						continue
					}
				}
				m[k] = g.wert(f.Type, tiefe+1)
			}
		}
		return m
	}
	return nil
}
