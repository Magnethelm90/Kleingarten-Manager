package main

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"
)

func rechnungFixture() (Settings, Paechter, Ablesung) {
	s := defaultSettings()
	s.IBAN, s.BIC = "DE89370400440532013000", "COBADEFFXXX"
	p := Paechter{Mitgliedsnr: "35-95", Gartennr: "35", Anrede: "Herr", Name: "Max Mustermann", Strasse: "Gartenweg 1",
		PLZOrt: "12345 Musterstadt", Gartengroesse: 300, WasserzaehlerNr: "W123", StromzaehlerNr: "S456", Versand: "Emailsendung"}
	a := Ablesung{WasserVJ: fp(148), WasserAkt: fp(191), StromVJ: fp(2136), StromAkt: fp(2302), Stunden: fp(20),
		Abschlag: 150, Versicherung: 12, Grundsteuer: 8}
	return s, p, a
}

func hinweis(zeilen int) string {
	var l []string
	for i := 1; i <= zeilen; i++ {
		l = append(l, fmt.Sprintf("Hinweiszeile %d: Zählerstand wurde am Jahrestag vom Vorstand abgelesen.", i))
	}
	return strings.Join(l, "\n")
}

var seitenRegexp = regexp.MustCompile(`/Type /Page[^s]`)

// Ein Hinweistext darf das Ausstellen nicht verhindern. Früher scheiterte schon eine einzige
// Zeile, weil die Seite ohne Hinweis bereits fast voll ist.
func TestRechnungMitHinweisPasstAufEineSeite(t *testing.T) {
	s, p, a := rechnungFixture()
	for _, n := range []int{0, 1, 2, 3, 4} {
		a.Hinweis = hinweis(n)
		pdf, err := buildInvoice(s, p, a, calculate(s, p, a))
		if err != nil {
			t.Fatalf("%d Zeile(n) Hinweis: %v", n, err)
		}
		if seiten := len(seitenRegexp.FindAll(pdf, -1)); seiten != 1 {
			t.Errorf("%d Zeile(n) Hinweis: %d Seiten, erwartet 1", n, seiten)
		}
	}
	a.Hinweis = hinweis(8)
	if _, err := buildInvoice(s, p, a, calculate(s, p, a)); err == nil {
		t.Error("ein sehr langer Hinweis muss weiterhin abgelehnt werden statt die Seite zu sprengen")
	}
}

// Das gewohnte Aussehen ohne Hinweis darf sich nicht verschieben (offizielles Dokument).
func TestRechnungStandardlayoutUnveraendert(t *testing.T) {
	s, p, a := rechnungFixture()
	_, ende, err := renderInvoice(s, p, a, calculate(s, p, a), 0)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(ende-287.0) > 0.05 {
		t.Errorf("letzte Zeile der Standardrechnung bei %.2f mm, bisher 287,0 mm: Layout wurde verändert", ende)
	}
}

func TestRechnungMitHinweisAusstellenUeberAPI(t *testing.T) {
	_, h := newTestApp(t)
	pid := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "A", "gartengroesse": 300})).ID
	abl := Ablesung{WasserVJ: fp(148), WasserAkt: fp(191), StromVJ: fp(2136), StromAkt: fp(2302), Stunden: fp(20), Hinweis: "Zähler wurde im Mai getauscht."}
	do(h, "PUT", "/api/ablesung/"+pid+"?year=2025", abl)
	res := decode[struct {
		Created []string    `json:"created"`
		Skipped []issueSkip `json:"skipped"`
	}](t, do(h, "POST", "/api/invoices/issue?year=2025", issueReq{}))
	if len(res.Created) != 1 || len(res.Skipped) != 0 {
		t.Fatalf("Rechnung mit Hinweis muss sich ausstellen lassen: %+v", res)
	}
}

// Lange Namen und Straßen dürfen nie über den Rand laufen: erst kleinere Schrift, dann Umbruch;
// die Rechnung (und die Mahnung) bleibt auf einer Seite.
func TestLangeAnschriftBleibtAufEinerSeite(t *testing.T) {
	s, p, a := rechnungFixture()
	for _, name := range []string{
		strings.Repeat("X", 100),
		"Prof. Dr. Maximilian-Alexander Freiherr von Mustermann-Schmidt",
		"Normal",
	} {
		p.Name = name
		p.Strasse = strings.Repeat("Langer Straßenname ", 5)
		r := calculate(s, p, a)
		pdf, err := buildInvoice(s, p, a, r)
		if err != nil {
			t.Fatalf("Rechnung für %.20q...: %v", name, err)
		}
		if n := len(seitenRegexp.FindAll(pdf, -1)); n != 1 {
			t.Errorf("Rechnung für %.20q...: %d Seiten", name, n)
		}
		rec := &Rechnung{Jahr: 2025, Nummer: invoiceNumber(s, p), Settings: s, Paechter: p, Ablesung: a, Result: r, Ausgestellt: "2026-01-10T10:00:00+01:00", Status: statusGueltig}
		m, err := buildMahnung(s, p, rec)
		if err != nil {
			t.Fatalf("Mahnung für %.20q...: %v", name, err)
		}
		if n := len(seitenRegexp.FindAll(m, -1)); n != 1 {
			t.Errorf("Mahnung für %.20q...: %d Seiten", name, n)
		}
	}
}
