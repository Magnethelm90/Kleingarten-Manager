package main

// Aufnahmeantrag: Vordruck für Bewerber von der Warteliste. Name und Anschrift sind vorausgefüllt, der Rest wird von
// Hand ergänzt. Enthält die Einwilligung zum Datenschutz und die Anerkennung von Satzung, Gartenordnung und
// Pachtvertrag. Wird dem Angebotsschreiben beigelegt; gespeichert wird nichts.

func buildAufnahmeantrag(s Settings, b Bewerber, garten, datum string) ([]byte, error) {
	pdf := neuesPDF(s, "Aufnahmeantrag", false)
	mr, mg, mb := markenfarbe(s.Farbe)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	y := 34.0
	pdf.SetFont("lib", "B", 15)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 8, "Aufnahmeantrag und Pachtantrag", "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	y += 9
	pdf.SetFont("lib", "", 10)
	pdf.SetXY(leftX, y)
	wunsch := "Hiermit beantrage ich die Aufnahme in den Verein " + s.VereinName + " und die Verpachtung eines Kleingartens"
	if garten != "" {
		wunsch += " (Garten " + garten + ")"
	}
	pdf.MultiCell(rightX-leftX, 5.2, wunsch+".", "", "L", false)
	y = pdf.GetY() + 5

	feld := func(label, wert string) {
		pdf.SetFont("lib", "", 8.5)
		pdf.SetTextColor(90, 90, 90)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(46, 7, label, "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("lib", "", 10.5)
		pdf.CellFormat(rightX-leftX-46, 7, wert, "", 0, "L", false, 0, "")
		pdf.SetDrawColor(150, 150, 150)
		pdf.Line(leftX+46, y+7, rightX, y+7)
		y += 9
	}
	abschnitt := func(t string) {
		y += 2
		pdf.SetFont("lib", "B", 10.5)
		pdf.SetTextColor(mr, mg, mb)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(rightX-leftX, 6, t, "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		y += 8
	}
	abschnitt("Angaben zur Person")
	name := b.Name
	if b.Anrede != "" {
		name = b.Anrede + " " + b.Name
	}
	feld("Name", name)
	feld("Geburtsdatum", "")
	feld("Straße, Hausnummer", b.Strasse)
	feld("PLZ, Ort", b.PLZOrt)
	feld("Telefon", b.Telefon)
	feld("E-Mail", b.Email)
	feld("Personen im Haushalt", "")

	kaestchen := func(text string) {
		pdf.SetDrawColor(60, 60, 60)
		pdf.Rect(leftX, y+0.8, 4, 4, "D")
		pdf.SetFont("lib", "", 9.5)
		pdf.SetXY(leftX+7, y)
		pdf.MultiCell(rightX-leftX-7, 5, text, "", "L", false)
		y = pdf.GetY() + 2.5
	}
	abschnitt("Erklärungen")
	kaestchen("Ich erkenne die Satzung des Vereins, die Gartenordnung und die Bestimmungen des Pachtvertrags an und halte sie ein. Ich habe sie erhalten oder die Möglichkeit zur Einsicht gehabt.")
	kaestchen("Ich bin damit einverstanden, dass der Verein meine Angaben für die Mitglieder- und Pachtverwaltung, für Rechnungen und für die Kommunikation mit mir speichert und verarbeitet (Art. 6 Abs. 1 lit. b DSGVO). Die Hinweise zum Datenschutz habe ich erhalten.")
	kaestchen("Ich bin damit einverstanden, dass mir der Verein Einladungen, Rechnungen und Rundschreiben per E-Mail schickt. Das kann ich jederzeit widerrufen.")
	kaestchen("Ich bestätige, dass ich keinen weiteren Kleingarten pachte und den Garten kleingärtnerisch nutze (Bundeskleingartengesetz).")

	y += 4
	pdf.SetFont("lib", "", 8.5)
	pdf.SetTextColor(90, 90, 90)
	pdf.SetXY(leftX, y+12)
	pdf.CellFormat(75, 5, "Ort, Datum", "T", 0, "L", false, 0, "")
	pdf.SetXY(leftX+95, y+12)
	pdf.CellFormat(75, 5, "Unterschrift der Antragstellerin / des Antragstellers", "T", 0, "L", false, 0, "")
	y += 26
	pdf.SetTextColor(0, 0, 0)
	pdf.SetDrawColor(150, 150, 150)
	pdf.Line(leftX, y, rightX, y)
	y += 4
	pdf.SetFont("lib", "B", 9)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 5, "Vom Vorstand auszufüllen", "", 0, "L", false, 0, "")
	y += 6
	pdf.SetFont("lib", "", 9)
	for _, z := range []string{"Antrag eingegangen am:", "Beschluss des Vorstands vom:", "Garten / Mitgliedsnummer:"} {
		pdf.SetXY(leftX, y)
		pdf.CellFormat(50, 6, z, "", 0, "L", false, 0, "")
		pdf.Line(leftX+50, y+6, leftX+110, y+6)
		y += 8
	}
	pdf.SetFont("lib", "", 7.5)
	pdf.SetTextColor(100, 100, 100)
	pdf.SetXY(leftX, 280)
	pdf.CellFormat(rightX-leftX, 4, "Vordruck vom "+germanDate(datum)+" · Die Angaben werden nur für die Vergabe und Verwaltung der Gärten verwendet.", "", 0, "L", false, 0, "")
	return pdfBytes(pdf)
}
