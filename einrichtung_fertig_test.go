package main

import (
	"encoding/json"
	"testing"
)

// Der Merker »Einrichtung fertig« steht im Zustand; Beispieldaten entfernen setzt ihn zurück.
func TestEinrichtungFertigWirdGemerkt(t *testing.T) {
	_, h := newTestApp(t)
	fertig := func() any {
		rec := do(h, "GET", "/api/state", nil)
		var m map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		return m["einrichtungFertig"]
	}
	if fertig() != false {
		t.Fatalf("neu: Einrichtung darf nicht als fertig gelten, war %v", fertig())
	}
	for i := 0; i < 2; i++ { // wiederholtes Melden ist harmlos
		if rec := do(h, "POST", "/api/admin/einrichtung-fertig", nil); rec.Code != 200 {
			t.Fatalf("einrichtung-fertig: %d %s", rec.Code, rec.Body)
		}
	}
	if fertig() != true {
		t.Error("Merker wurde nicht gespeichert")
	}
}
