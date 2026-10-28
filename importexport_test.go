package main

import (
	"strings"
	"testing"
)

func TestImportSynonymeUndWarnungen(t *testing.T) {
	table := [][]string{
		{"Mitgl.-Nr.", "Nachname", "Versandart", "Gartengröße", "Bemerkung"},
		{"1", "Anna", "Brief", "1,5", "x"},
	}
	rows, warn, err := parseImport(table, true, Settings{}, nil)
	if err != nil || len(rows) != 1 {
		t.Fatalf("Import: %v %v", err, rows)
	}
	if rows[0].Paechter.Gartengroesse != 1.5 {
		t.Errorf("Text-Zahl mit Komma im Excel-Rohmodus: %v", rows[0].Paechter.Gartengroesse)
	}
	if len(rows[0].Warnungen) != 1 || rows[0].Paechter.Versand != "Emailsendung" {
		t.Errorf("unbekannte Versandart: %v %q", rows[0].Warnungen, rows[0].Paechter.Versand)
	}
	if len(warn) != 1 || !strings.Contains(warn[0], "Bemerkung") {
		t.Errorf("ignorierte Spalte nicht gemeldet: %v", warn)
	}
}

// Wer den eigenen Excel-Export wieder einliest, soll keine Warnung über die berechneten Spalten bekommen.
func TestImportEigenerExportOhneSpaltenWarnung(t *testing.T) {
	head := append([]string{"Mitgliedsnr.", "Name"}, exportBerechnet...)
	row := make([]string, len(head))
	row[0], row[1] = "1", "Anna"
	_, warn, err := parseImport([][]string{head, row}, true, Settings{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(warn) != 0 {
		t.Errorf("unerwartete Warnung beim eigenen Export: %v", warn)
	}
}
