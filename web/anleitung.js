'use strict';
/* Kleingarten-Manager – Oberfläche, Teil: Anleitung (Hilfe-Seite mit Suche). Texte in einfachen Worten. */

// Jede Anleitung: Titel, Stichworte für die Suche, Schritte, Sprungziel (wie bei den Cockpit-Zielen).
const ANLEITUNGEN = [
  { titel: 'Zum ersten Mal starten', stichworte: 'anfang einrichten erste schritte neu verein', ziel: 'admin:einstellungen', knopf: 'Zu den Einstellungen',
    schritte: ['Öffne »Einstellungen« und trage Vereinsname, Bank und Preise ein.', 'Unter »Bereiche« schaltest du ab, was dein Verein nicht braucht.', 'Lege dann deine Pächter an (Menü »Pächter«).', 'Setze ein Passwort unter »Einstellungen → Passwort & Sicherheit«.'] },
  { titel: 'Einen Pächter anlegen', stichworte: 'pächter neu anlegen mitglied garten hinzufügen', ziel: 'admin:paechter', knopf: 'Zu den Pächtern',
    schritte: ['Öffne »Pächter« und drücke »+ Pächter«.', 'Trage Mitgliedsnummer, Name, Gartennummer und Größe ein.', 'Hat der Pächter eine E-Mail-Adresse, steht der Versand automatisch auf E-Mail, sonst auf Post.', 'Drücke »Speichern«. Viele Pächter auf einmal geht über »Import«.'] },
  { titel: 'Zählerstände eintragen', stichworte: 'zähler wasser strom ablesen ablesung stand verbrauch', ziel: 'tab:eingabe', knopf: 'Zu den Zählerständen',
    schritte: ['Öffne »Zählerstände«. Die Schnelleingabe ist schon offen.', 'Tippe die Gartennummer und drücke Enter.', 'Trage Wasser und Strom ein. Der alte Stand steht schon da.', 'Mit dem Handy ablesen? Berichte → »Ablesung mit dem Handy«.'] },
  { titel: 'Rechnungen erstellen und verschicken', stichworte: 'rechnung ausstellen pdf drucken email versenden', ziel: 'tab:rechnungen', knopf: 'Zu den Rechnungen',
    schritte: ['Sind alle Zählerstände drin, öffne »Rechnungen«.', 'Prüfe die Rechnungen. Fehlt noch etwas, zeigt das Programm es an.', 'Drücke »Alle offenen ausstellen«. Danach ändert sich die Rechnung nicht mehr.', 'Dann »Postversand drucken« oder »E-Mails vorbereiten«.'] },
  { titel: 'Eine Zahlung eintragen', stichworte: 'zahlung bezahlt eingang geld überweisung teilbetrag rate', ziel: 'tab:zahlungen', knopf: 'Zu den Zahlungen',
    schritte: ['Öffne »Zahlungen«.', 'Setze bei der bezahlten Rechnung den Haken. Bei einem Teilbetrag trägst du die Summe in das kleine Feld ein.', 'Viele Zahlungen auf einmal? Lies den Kontoauszug (CSV) ein – das Programm ordnet sie zu.', 'Wer nicht zahlen kann: Im Zahlungs-Dialog gibt es den Reiter »Ratenplan«.'] },
  { titel: 'Mahnen', stichworte: 'mahnung erinnerung überfällig zahlungserinnerung', ziel: 'tab:zahlungen', knopf: 'Zu den Zahlungen',
    schritte: ['Öffne »Zahlungen«.', 'Drücke »Mahn-Vorschlag«. Das Programm zeigt, wer jetzt dran ist.', 'Nimm den Haken weg bei wem du nicht mahnen willst.', 'Drücke »Weiter zu den Briefen«. Es wird nie von allein gemahnt.'] },
  { titel: 'Gartenwechsel (Kündigung und Neuvergabe)', stichworte: 'kündigung wechsel nachfolger übergabe garten frei warteliste wertermittlung', ziel: 'admin:paechter', knopf: 'Zu den Pächtern',
    schritte: ['Beim Pächter im Menü »Mehr« wähle »Garten abgeben …«.', 'Wähle »Mit Kündigung«. Der Assistent zeigt dann jeden Schritt: Bestätigung, Nachfolger, Wertermittlung, Abnahme, Schlüssel.', 'Der Nachfolger kommt von der Warteliste (Menü »Pächter«, Reiter »Warteliste«).', 'Am Ende startest du den Pächterwechsel mit Übergabeprotokoll.'] },
  { titel: 'Eine Ausgabe oder Spende buchen', stichworte: 'ausgabe kasse buchen beleg spende quittung bar', ziel: 'admin:kassenbericht', knopf: 'Zur Kasse',
    schritte: ['Öffne »Kasse«. Im Kassenbericht gibt es »+ Ausgabe«.', 'Trage Datum, Betrag und Kategorie ein und hänge den Beleg an.', 'Mit »Speichern und nächste« geht es gleich mit der nächsten Ausgabe weiter.', 'Spenden stehen im Reiter »Spenden«. Dort erstellst du auch die Spendenbescheinigung.'] },
  { titel: 'Mängel bei der Begehung erfassen', stichworte: 'mängel begehung rundgang frist foto garten kontrolle', ziel: 'admin:maengel', knopf: 'Zu den Mängeln',
    schritte: ['Öffne »Anlage« und dort »Mängel & Lauben«.', 'Starte den Begehungsmodus und gehe von Garten zu Garten.', 'Trage Mängel mit Frist ein. Fotos kannst du anhängen.', 'Das Programm erinnert an Fristen und schreibt die Briefe vor.'] },
  { titel: 'Eine Versammlung vorbereiten', stichworte: 'mitgliederversammlung termin einladung protokoll beschluss anwesenheit', ziel: 'tab:termine', knopf: 'Zu den Terminen',
    schritte: ['Lege unter »Termine« die Versammlung an.', 'Mit »Einladung« druckst du die Einladung, mit »Anwesenheitsliste« die Liste.', 'Nach der Versammlung schreibst du mit »Protokoll« Abstimmungen auf – sie landen im Beschlussbuch.', '»Paket« legt alles für die Versammlung in eine Datei.'] },
  { titel: 'Das Jahr abschließen', stichworte: 'jahreswechsel abschluss jahr neu abrechnung ende dezember', ziel: 'admin:jahreswechsel', knopf: 'Zum Jahreswechsel',
    schritte: ['Öffne »Berichte & Jahresabschluss«, Reiter »Jahreswechsel«.', 'Die Ampel zeigt, was noch fehlt (offene Rechnungen, fehlende Stände …).', 'Drücke »Jahr abschließen«. Das alte Jahr wird gesichert und bleibt nur lesbar.', 'Das neue Jahr startet mit den Preisen vom alten.'] },
  { titel: 'Sichern und Daten wiederherstellen', stichworte: 'sicherung backup usb stick wiederherstellen verschlüsselt daten verloren', ziel: 'admin:daten', knopf: 'Zur Sicherung',
    schritte: ['Das Programm sichert bei jeder Änderung selbst in den Sicherungsordner.', 'Stelle unter »Einstellungen → Import, Export, Sicherung« einen zweiten Ordner (USB-Stick) ein.', 'Mit »Wartungs-Check« siehst du, ob alles in Ordnung ist.', 'Zum Zurückholen wähle eine Sicherung und »Ansehen« oder »Wiederherstellen«.'] },
  { titel: 'Brieftexte anpassen', stichworte: 'brief text vorlage anrede gruß unterschrift mahnung formulierung', ziel: 'admin:einstellungen', knopf: 'Zu den Einstellungen',
    schritte: ['Öffne »Einstellungen« und klicke unten bei »Weitere Einstellungen« auf »Brieftexte«.', 'Drücke bei einem Text auf »Bearbeiten« und schreibe ihn so, wie euer Verein spricht.', 'Mit den Knöpfen wie {Frist} setzt du Angaben ein, die das Programm selbst füllt.', '»Standardtext einsetzen« holt den Text des Programms zurück.'] },
  { titel: 'Fristen aus der Satzung eintragen', stichworte: 'frist satzung einladung mitgliederversammlung kündigung kündigungsfrist erinnerung', ziel: 'admin:einstellungen', knopf: 'Zu den Einstellungen',
    schritte: ['Öffne »Einstellungen« und klicke unten bei »Weitere Einstellungen« auf »Fristen aus der Satzung«.', 'Trage ein, wie viele Tage vorher die Einladung zur Versammlung raus muss.', 'Trage ein, bis wann eine Kündigung eingegangen sein muss und wann die Pacht dann endet.', 'Lege die Versammlung als Termin an. Das Cockpit sagt dir dann, bis zu welchem Tag die Einladung raus muss.'] },
  { titel: 'Mehrere Pächter auf einmal ändern', stichworte: 'mehrere pächter versandart umlage sammeln auswahl alle auf einmal post email', ziel: 'admin:paechter', knopf: 'Zu den Pächtern',
    schritte: ['Öffne »Pächter«.', 'Setze vorne in der Tabelle bei den gewünschten Pächtern einen Haken.', 'Drücke auf »Auswahl ändern«.', 'Wähle die neue Versandart oder Umlage. Was auf »nicht ändern« steht, bleibt, wie es ist.'] },
  { titel: 'Zwei Rechner im Verein benutzen', stichworte: 'zweiter rechner abgleich mehrplatz usb kassenwart vorsitzender', ziel: 'admin:daten', knopf: 'Zum Abgleich',
    schritte: ['Es darf immer nur ein Rechner mit dem Datenordner arbeiten.', 'Für den zweiten Rechner: »Abgleich-Datei« auf einem Stick erstellen.', 'Auf dem anderen Rechner liest du sie ein. Du siehst vorher, was neu dazukommt.', 'Es kommen nur neue Einträge dazu, nichts wird überschrieben. Fehlende Zählerstände werden ergänzt.', 'Vorauszahlungen und Barkasse gehen nicht mit. Die bucht nur ein Rechner.'] },
  { titel: 'Datenschutz: Auskunft und Löschen', stichworte: 'dsgvo datenschutz auskunft löschen person ausgeschieden einwilligung', ziel: 'admin:datenschutz', knopf: 'Zum Datenschutz',
    schritte: ['Auskunft: Beim Pächter im Menü »Mehr« → »Mappe …« → »Auskunft nach DSGVO«.', 'Löschen: Unter »Einstellungen → Datenschutz« siehst du, wer gelöscht werden darf.', 'Geburtstage werden nur mit Einverständnis gespeichert.', 'Was das Programm über Personen speichert, steht in der Datei DATENSCHUTZ.md.'] },
];

async function anleitungDialog() {
  const suche = h('input', { type: 'search', placeholder: 'Wobei brauchst du Hilfe? z. B. Rechnung, Mahnung, Sicherung', 'aria-label': 'Anleitung suchen', autocomplete: 'off', style: 'width:100%' });
  const liste = h('div', { class: 'anleitung-liste' });
  let dlg = null;
  const zeichne = () => {
    const q = suche.value.trim();
    const l = ANLEITUNGEN.filter((a) => !q || sucheTrifft([a.titel, a.stichworte, ...a.schritte], q));
    liste.replaceChildren(...(l.length ? l.map((a) => h('details', { class: 'anleitung', open: l.length <= 2 },
      h('summary', null, h('b', null, a.titel)),
      h('ol', null, a.schritte.map((t) => h('li', null, t))),
      h('button', { class: 'btn small', onclick: () => { if (dlg) dlg.close(); zielOeffnen(a.ziel); } }, a.knopf + ' →')))
      : [h('p', { class: 'hint' }, 'Dazu habe ich nichts gefunden. Versuch ein anderes Wort oder lies »Wörter erklärt« im Hilfe-Menü.')]));
  };
  suche.addEventListener('input', zeichne);
  zeichne();
  const inhalt = h('div', null, h('p', { class: 'hint' }, 'Schritt für Schritt, in einfachen Worten. Tippe ein Wort oder klicke auf eine Frage.'), suche, liste);
  setTimeout(() => { dlg = inhalt.closest('dialog'); suche.focus(); }, 50);
  await modal('Anleitung', inhalt, [{ label: 'Schließen', cls: 'primary', value: true }]);
}
