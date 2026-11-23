package main

import "testing"

func TestAuffaellig(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	if !auffaellig(f(60), f(20), 10) {
		t.Error("Verdreifachung nicht erkannt")
	}
	if auffaellig(f(24), f(20), 1) || auffaellig(f(15), f(10), 10) || auffaellig(f(5), nil, 1) || auffaellig(f(5), f(0), 1) {
		t.Error("harmloser Anstieg gemeldet")
	}
}

func TestKostenvergleichRoute(t *testing.T) {
	_, h := newTestApp(t)
	if c := do(h, "GET", "/api/admin/kostenvergleich", nil).Code; c != 200 {
		t.Fatalf("Code %d", c)
	}
	k := decode[kostenvergleich](t, do(h, "GET", "/api/admin/kostenvergleich", nil))
	if k.Gaerten == nil || k.Hinweise == nil || len(k.Jahre) != len(k.Verein) {
		t.Fatalf("Aufbau: %+v", k)
	}
}
