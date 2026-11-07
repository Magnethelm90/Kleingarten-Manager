package main

import (
	"reflect"
	"testing"
)

// Emoji im Namen oder in Texten dürfen keine PDF-Erzeugung mehr abbrechen (die Schrift kennt nur Zeichen bis U+FFFF).
func TestEmojiInTextenUndPDF(t *testing.T) {
	_, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Anna 🌻 Müller", "gartengroesse": 300}))
	if p.Name != "Anna  Müller" {
		t.Errorf("Name: %q", p.Name)
	}
	abl := Ablesung{WasserVJ: fp(0), WasserAkt: fp(40), StromVJ: fp(0), StromAkt: fp(300), Stunden: fp(12), Hinweis: "Zähler 💧 tropft"}
	do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2025", abl)
	rec := do(h, "GET", "/api/invoice/"+p.ID+".pdf?year=2025", nil)
	pdfAblegen(t, "emoji-rechnung.pdf", rec.Body.Bytes())

	d := &Data{Paechter: []Paechter{{Name: "X 😀"}}, Haushalt: map[string]Haushaltsplan{"2025 🎉": {Notiz: "🌱 Plan"}},
		Jahre: map[string]*Jahr{"2025": {Ablesungen: map[string]Ablesung{"a": {Hinweis: "🙂ok"}}}}}
	texteBereinigen(reflect.ValueOf(d))
	if d.Paechter[0].Name != "X " || d.Haushalt["2025 "].Notiz != " Plan" || d.Jahre["2025"].Ablesungen["a"].Hinweis != "ok" {
		t.Errorf("bereinigt: %+v %+v %+v", d.Paechter, d.Haushalt, d.Jahre["2025"].Ablesungen)
	}
}
