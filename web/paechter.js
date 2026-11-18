'use strict';
/* Kleingarten-Manager – Oberfläche, Teil: Pächterwechsel, Warteliste und Mängel. Gemeinsamer Gültigkeitsbereich mit den anderen Dateien (siehe index.html für die Reihenfolge). */

// ------------------------------------------------------------------ Pächterwechsel

// Hinweis an Pächtern, die ihren Garten schon übergeben haben: sie bleiben bis zum Jahreswechsel für ihre
// Schlussrechnung im laufenden Jahr.
function schlussBadge(p) {
  if (!p.ausgeschiedenAm) return null;
  return h('span', { class: 'badge neu', style: 'margin-left:6px', title: `Garten am ${deDate(p.ausgeschiedenAm)} übergeben. Abgerechnet wird nur der Verbrauch bis zur Übergabe, die Beiträge fürs Folgejahr zahlt der Nachfolger. Kommt beim Jahreswechsel in den Papierkorb.` }, 'Schlussrechnung');
}

// nächste freie Mitgliedsnummer als Vorschlag (größte Zahl + 1)
function naechsteMitgliedsnr() {
  let max = 0;
  for (const p of S.state.paechter) { const n = parseInt(p.mitgliedsnr, 10); if (n > max) max = n; }
  return String(max + 1);
}

// ------------------------------------------------------------------ Pächterwechsel-Assistent
// Führt Schritt für Schritt durch den Wechsel eines Gartens. Der Stand kommt aus dem Fahrplan der Kündigung; jeder
// Schritt hat einen Knopf, der die vorhandene Funktion öffnet. Nach jeder Aktion zeigt der Assistent den neuen Stand.
function beitragskontoInhalt(k) {
  const dat = (iso) => (iso ? isoDatum(iso).toLocaleDateString('de-DE') : '–');
  const saldoText = k.saldo > 0.004 ? `Noch zu zahlen: ${eur(k.saldo)}` : k.saldo < -0.004 ? `Guthaben des Pächters: ${eur(-k.saldo)}` : 'Ausgeglichen';
  const tabelle = k.zeilen.length ? h('div', { class: 'tablewrap', style: 'max-height:42vh' }, h('table', { class: 'data' },
    h('thead', null, h('tr', null, ['Jahr', 'Rechnung', 'Datum', 'Fällig', 'Betrag', 'Bezahlt', 'Offen', 'Mahnungen'].map((t) => h('th', null, t)))),
    h('tbody', null, k.zeilen.map((z) => h('tr', null,
      h('td', null, String(z.jahr)), h('td', null, z.nummer), h('td', null, dat(z.datum)), h('td', null, dat(z.faellig)),
      h('td', { class: 'num' }, eur(z.betrag)),
      h('td', { class: 'num' }, z.bezahltAm ? `${eur(z.bezahlt)} (${dat(z.bezahltAm)})` : '–'),
      h('td', { class: 'num' }, Math.abs(z.offen) > 0.004 ? eur(z.offen) : '–'),
      h('td', null, z.mahnstufe ? String(z.mahnstufe) : '–')))))) : h('p', { class: 'hint' }, 'Für diesen Pächter gibt es noch keine Rechnungen.');
  const vz = k.vorauszahlungen.length ? h('p', { class: 'hint' }, 'Vorauszahlungen: ' + k.vorauszahlungen.map((v) => `${v.jahr}: ${eur(v.summe)} (${v.anzahl}×${v.gebucht ? '' : ', teils vorgemerkt'})`).join(' · ')) : null;
  return h('div', null,
    h('p', { class: 'hint' }, [k.gartennr ? `Garten ${k.gartennr}` : '', k.mitgliedsnr ? `Mitgliedsnr. ${k.mitgliedsnr}` : ''].filter(Boolean).join(' · ')),
    tabelle, vz, h('p', null, h('b', null, saldoText)));
}

const kontoPdfDrucken = (p) => pdfDrucken({ url: `/api/admin/paechter/${encodeURIComponent(p.id)}/konto/pdf`, oeffnenUrl: `/api/admin/paechter/${encodeURIComponent(p.id)}/konto/pdf/oeffnen`, titel: 'Beitragsübersicht', meldung: () => 'Die Beitragsübersicht wurde im PDF-Programm geöffnet – dort drucken.' });

// Eine Mappe je Pächter: Beiträge und alle Dokumente an einer Stelle (statt vieler Einzelpunkte im Menü).
async function paechterMappe(p) {
  let k;
  try { k = await api('GET', `/api/admin/paechter/${encodeURIComponent(p.id)}/konto`); } catch (e) { handleErr(e); return; }
  let aktiv = 'beitraege';
  const flaeche = h('div'), kopf = h('div');
  const zeile = (titel, text, label, fn) => h('div', { class: 'vordruck' }, h('div', null, h('b', null, titel), h('div', { class: 'hint' }, text)), h('div', { class: 'row' }, h('button', { class: 'btn', onclick: fn }, label)));
  const dokumente = () => h('div', { class: 'vordrucke' },
    zeile('Beitragsübersicht', 'Alle Rechnungen, Zahlungen und Mahnungen über die Jahre als PDF.', 'PDF', () => kontoPdfDrucken(p)),
    p.gartennr && p.gartengroesse > 0 ? zeile('Pachtvertrag', 'Unterpachtvertrag zum Unterschreiben, auf Wunsch in die Gartenakte.', 'Öffnen', () => pachtvertragDialog(p)) : null,
    p.gartennr && !p.ausgeschiedenAm ? zeile('Willkommensmappe', 'Garten, Zähler, Termine, Lageplan, Ansprechpartner.', 'PDF', () => willkommenDrucken(p)) : null,
    zeile('Ehrenamts-Nachweis', 'Bescheinigung über Stunden und Ämter.', 'PDF', () => ehrenamtDrucken(S.state.currentYear, p.id)),
    p.gartennr ? zeile('Gartenakte', 'Dokumente, Wertermittlungen, Übergaben und Verlauf.', 'Öffnen', () => gartenakteDialog(p.gartennr.trim())) : null,
    zeile('Auskunft nach DSGVO', 'Alle gespeicherten Daten dieser Person als Datei (Art. 15 DSGVO).', 'Herunterladen', () => { location.href = `/api/admin/paechter/${encodeURIComponent(p.id)}/auskunft`; }));
  let geb = { datum: '', einwilligung: false };
  const angaben = () => {
    const datum = h('input', { type: 'date', value: geb.datum || '', max: todayIso() });
    const ok = h('input', { type: 'checkbox', checked: !!geb.einwilligung });
    return h('div', { class: 'form' },
      h('p', { class: 'hint field wide' }, 'Freiwillige Angabe. Das Geburtsdatum wird nur gespeichert, wenn die Person damit einverstanden ist. Das Cockpit erinnert dann eine Woche vorher an die Glückwünsche. Ohne Haken wird nichts behalten.'),
      h('div', { class: 'field' }, h('label', null, 'Geburtsdatum'), datum),
      h('label', { class: 'inline field wide' }, ok, ' Die Person ist einverstanden, dass ihr Geburtstag gespeichert wird.'),
      h('div', { class: 'field wide' }, h('button', { class: 'btn primary', onclick: async () => {
        if (datum.value && !ok.checked) { toast('Bitte erst das Einverständnis anhaken – sonst wird nichts gespeichert.', 'err'); return; }
        try { geb = await api('PUT', `/api/admin/paechter/${encodeURIComponent(p.id)}/geburtstag`, { datum: datum.value, einwilligung: ok.checked }); toast(geb.datum ? 'Geburtstag gespeichert' : 'Nichts gespeichert', 'ok'); zeichne(); } catch (e) { handleErr(e); }
      } }, 'Speichern')));
  };
  const zeichne = () => {
    kopf.replaceChildren(reiterLeiste([['beitraege', 'Beiträge'], ['dokumente', 'Dokumente'], ...(modulAn('jubilaeen') ? [['angaben', 'Angaben']] : [])].map(([key, label]) => ({ label, aktiv: key === aktiv, geh: () => { aktiv = key; zeichne(); } })), 'Pächter-Mappe'));
    flaeche.replaceChildren(aktiv === 'beitraege' ? beitragskontoInhalt(k) : aktiv === 'angaben' ? angaben() : dokumente());
  };
  try { geb = await api('GET', `/api/admin/paechter/${encodeURIComponent(p.id)}/geburtstag`) || geb; } catch (e) { handleErr(e); }
  zeichne();
  await modal(`Mappe – ${k.name}`, h('div', null, kopf, flaeche), [{ label: 'Schließen', cls: 'primary', value: true }]);
}

async function wechselAssistent(p) {
  const holen = async () => (await api('GET', '/api/admin/kuendigungen')).kuendigungen.find((x) => x.paechterId === p.id && !x.abgeschlossen) || null;
  let k;
  try { k = await holen(); } catch (e) { handleErr(e); return; }
  const inhalt = h('div');
  let aktuell = p;
  const frisch = async () => {
    try { k = await holen(); await reload(); aktuell = S.state.paechter.find((x) => x.id === p.id) || aktuell; } catch (e) { handleErr(e); }
    zeichnen();
  };
  const dlg = { schliessen: null };
  const aktionen = {
    bestaetigung: ['Bestätigung drucken', () => pdfDrucken({ url: `/api/admin/kuendigungen/${encodeURIComponent(k.id)}/bestaetigung`, oeffnenUrl: `/api/admin/kuendigungen/${encodeURIComponent(k.id)}/bestaetigung/oeffnen`,
      titel: 'Kündigungsbestätigung', meldung: () => 'Die Bestätigung wurde im PDF-Programm geöffnet – dort drucken.' }).then(frisch)],
    warteliste: ['Zur Warteliste', () => { dlg.schliessen(true); adminTabWaehlen('warteliste'); render(); }],
    wertermittlung: ['Wertermittlung anlegen', async () => { await wertermittlungDialog(p.gartennr); await frisch(); }],
    raeumung: ['Zu den Mängeln', () => { dlg.schliessen(true); adminTabWaehlen('maengel'); render(); }],
    schluessel: ['Zu Schlüssel & Geräte', () => { dlg.schliessen(true); adminTabWaehlen('inventar'); render(); }],
    uebergabe: ['Pächterwechsel starten', async () => { await paechterwechselDialog(aktuell); await frisch(); }],
  };
  const markieren = async (key, an) => {
    try { await api('POST', `/api/admin/kuendigungen/${encodeURIComponent(k.id)}/schritt`, { schritt: key, erledigt: an }); await frisch(); } catch (e) { handleErr(e); }
  };
  const zeichnen = () => {
    if (!k) {
      inhalt.replaceChildren(
        h('p', null, `Der Wechsel beginnt mit der Kündigung. Danach führt dich dieser Assistent durch Bestätigung, Nachfolger, Wertermittlung, Abnahme, Schlüsselrückgabe und Übergabe.`),
        h('button', { class: 'btn primary', onclick: async () => { if (await kuendigungDialog(aktuell)) { S.kuend = null; await frisch(); } } }, 'Kündigung erfassen'));
      return;
    }
    const fertig = k.schritte.filter((x) => x.erledigt).length;
    const gesamt = k.schritte.length;
    const naechster = k.schritte.find((x) => !x.erledigt);
    inhalt.replaceChildren(
      h('p', { class: 'hint' }, `Garten ${k.gartennr} – ${k.name}: Kündigung vom ${deDate(k.eingang)} zum ${deDate(k.zum)}.`),
      h('div', { class: 'wa-balken', role: 'progressbar', 'aria-valuemin': 0, 'aria-valuemax': gesamt, 'aria-valuenow': fertig },
        h('i', { style: `width:${Math.round((fertig / gesamt) * 100)}%` })),
      h('p', { class: 'hint' }, `${fertig} von ${gesamt} Schritten erledigt.`),
      h('ol', { class: 'kd-schritte' }, k.schritte.map((x) => {
        const akt = aktionen[x.key];
        return h('li', { class: x.erledigt ? 'erledigt' : '' },
          h('label', { class: 'check' }, h('input', { type: 'checkbox', checked: !!x.erledigt, disabled: x.auto, onchange: (e) => markieren(x.key, e.target.checked) }),
            h('span', null, x.titel, x.erledigt ? h('span', { class: 'hint' }, x.auto ? ' – vom Programm erkannt' : ` – ${deDate(x.erledigt)}`) : null)),
          x.erledigt ? null : h('div', { class: 'hint kd-hilfe' }, x.hilfe),
          !x.erledigt && akt ? h('button', { class: 'btn small' + (naechster && naechster.key === x.key ? ' primary' : ''), onclick: akt[1] }, akt[0]) : null);
      })),
      fertig === gesamt ? h('p', null, h('b', null, 'Alle Schritte sind erledigt.'), ' Schließe die Kündigung unter »Pächter« im Reiter »Kündigungen« ab. Für den Nachfolger: Pachtvertrag und Willkommensmappe findest du beim neuen Pächter unter »Mehr«.') : h('span'));
  };
  zeichnen();
  const w = breiterDialog(`Pächterwechsel – Garten ${p.gartennr}`, inhalt, [{ label: 'Schließen', cls: 'primary', value: true }]);
  dlg.schliessen = () => { const b = [...document.querySelectorAll('dialog:last-of-type .dfoot button, dialog:last-of-type button')].find((x) => x.textContent === 'Schließen'); if (b) b.click(); };
  await w;
}

async function paechterwechselDialog(alt, bewerber) {
  const st = S.state;
  const ab = st.ablesungen[alt.id] || {};
  if (st.issued && st.issued[alt.id]) {
    toast(`Für ${alt.name} ist die Rechnung ${st.currentYear} schon ausgestellt. Bitte zuerst den Jahreswechsel machen, dann den Pächterwechsel.`, 'err');
    return false;
  }
  let liste = [], wert = null;
  try { liste = await api('GET', '/api/admin/warteliste'); } catch (e) { handleErr(e); }
  try { wert = (await api('GET', `/api/admin/gartenakte?gartennr=${encodeURIComponent((alt.gartennr || '').trim())}`)).wertermittlungen[0] || null; } catch (e) { /* ohne Vorschlag weiter */ }
  const t = (val, ph, typ) => h('input', { type: typ || 'text', value: val ?? '', placeholder: ph || '', maxlength: 100, autocomplete: 'off' });
  const zahl = (val, ph) => { const el = t(numIn(val), ph); el.setAttribute('inputmode', 'decimal'); return el; };
  const f = {
    datum: t(todayIso(), '', 'date'),
    wasserVJ: zahl(ab.wasserVJ), stromVJ: zahl(ab.stromVJ),
    wasser: zahl(null, ab.wasserVJ != null ? `Vorjahr: ${numIn(ab.wasserVJ)}` : ''),
    strom: zahl(null, ab.stromVJ != null ? `Vorjahr: ${numIn(ab.stromVJ)}` : ''),
    stunden: zahl(ab.stunden, '0, wenn keine'),
    warte: h('select', null, h('option', { value: '' }, liste.length ? '– neuen Pächter von Hand eintragen –' : '(Warteliste ist leer)'),
      liste.map((b) => h('option', { value: b.id, selected: bewerber && bewerber.id === b.id }, `${b.platz}. ${b.name}${b.wunsch ? ' – ' + b.wunsch : ''}`))),
    mitgliedsnr: t(naechsteMitgliedsnr()),
    anrede: h('select', null, ['Herr', 'Frau', 'Familie', 'Herr und Frau', ''].map((a) => h('option', { value: a }, a || '(keine)'))),
    name: t(''), strasse: t(''), plzOrt: t(st.settings.ort),
    versand: h('select', null, ['Emailsendung', 'Postversand'].map((a) => h('option', { value: a }, a))),
    email: t('', 'name@beispiel.de', 'email'), telefon: t('', '', 'tel'),
    groesse: zahl(alt.gartengroesse),
    abloese: zahl(wert ? wert.summe : null, 'leer = keine'),
    schluessel: t('', 'z. B. 2 Torschlüssel, 1 Laubenschlüssel'),
    bemerkungen: h('textarea', { rows: 3, maxlength: 1500, style: 'width:100%', placeholder: 'Zustand von Laube und Garten, Mängel, Absprachen …' }),
  };
  const uebernehmen = () => {
    const b = liste.find((x) => x.id === f.warte.value);
    if (!b) return;
    f.anrede.value = b.anrede || ''; f.name.value = b.name; f.strasse.value = b.strasse || ''; f.plzOrt.value = b.plzOrt || st.settings.ort;
    f.email.value = b.email || ''; f.telefon.value = b.telefon || ''; f.versand.value = b.email ? 'Emailsendung' : 'Postversand';
  };
  f.warte.addEventListener('change', uebernehmen);
  uebernehmen();
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const sect = (titel) => h('div', { class: 'section-title' }, titel);
  const body = h('div', { class: 'form' },
    h('p', { class: 'hint wide', style: 'grid-column:1/-1;margin:0' },
      `${alt.name} gibt Garten ${alt.gartennr} ab. ${alt.name} bekommt eine Schlussrechnung über den Verbrauch bis zur Übergabe (ohne Pacht und Beiträge fürs Folgejahr) und kommt beim Jahreswechsel in den Papierkorb. Der neue Pächter startet mit den heutigen Zählerständen. Vorher wird automatisch eine Sicherung angelegt.`),
    sect('1. Übergabe und Zählerstände'),
    fld('Tag der Übergabe *', f.datum),
    ab.wasserVJ == null ? fld('Wasser: Vorjahresstand *', f.wasserVJ, 'fehlt noch beim bisherigen Pächter') : null,
    ab.stromVJ == null ? fld('Strom: Vorjahresstand *', f.stromVJ, 'fehlt noch beim bisherigen Pächter') : null,
    fld(`Wasserzähler ${alt.wasserzaehlerNr || ''} – Stand bei Übergabe (m³) *`, f.wasser),
    fld(`Stromzähler ${alt.stromzaehlerNr || ''} – Stand bei Übergabe (kWh) *`, f.strom),
    fld(`Arbeitsstunden von ${alt.name} bis zur Übergabe *`, f.stunden, `Pflicht sind ${nfFlex.format(st.settings.pflichtstunden)} h; fehlende werden berechnet`),
    sect('2. Neuer Pächter'),
    fld('Von der Warteliste', f.warte, liste.length ? 'Übernimmt Name und Kontakt; der Eintrag wird danach von der Warteliste genommen.' : null, true),
    fld('Mitgliedsnummer *', f.mitgliedsnr, 'Neue Nummer; die alte bleibt bis zum Jahreswechsel beim bisherigen Pächter'),
    fld('Anrede', f.anrede), fld('Name *', f.name), fld('Straße und Hausnummer', f.strasse), fld('PLZ und Ort', f.plzOrt),
    fld('Versandart', f.versand), fld('E-Mail-Adresse', f.email), fld('Telefon', f.telefon),
    fld('Gartengröße in m²', f.groesse),
    sect('3. Vereinbarungen (für das Übergabeprotokoll)'),
    fld('Ablösesumme in €', f.abloese, wert ? `aus der Wertermittlung vom ${deDate(wert.datum)} (${eur(wert.summe)}); zahlt der neue an den bisherigen Pächter` : 'zahlt der neue an den bisherigen Pächter; eine Wertermittlung lässt sich in der Gartenakte anlegen'), fld('Übergebene Schlüssel', f.schluessel),
    fld('Zustand und Bemerkungen', f.bemerkungen, null, true));
  let ergebnis = null;
  const ok = await modal(`Pächterwechsel Garten ${alt.gartennr}`, body, [
    { label: 'Abbrechen', value: false },
    { label: 'Wechsel durchführen', cls: 'primary', value: true, action: async () => {
      const n = (el) => parseNum(el.value);
      const werte = { wasserVJ: n(f.wasserVJ), stromVJ: n(f.stromVJ), wasserStand: n(f.wasser), stromStand: n(f.strom), stunden: n(f.stunden), abloese: n(f.abloese), groesse: n(f.groesse) };
      if (Object.values(werte).some((x) => Number.isNaN(x) || (x != null && x < 0))) { toast('Zählerstände, Stunden, Größe und Ablöse müssen Zahlen ab 0 sein.', 'err'); return false; }
      const daten = {
        altId: alt.id, datum: f.datum.value, wasserVJ: werte.wasserVJ, stromVJ: werte.stromVJ, wasserStand: werte.wasserStand, stromStand: werte.stromStand,
        stunden: werte.stunden, abloese: werte.abloese, schluessel: f.schluessel.value, bemerkungen: f.bemerkungen.value.trim(), wartelisteId: f.warte.value,
        neu: { mitgliedsnr: f.mitgliedsnr.value, anrede: f.anrede.value, name: f.name.value, strasse: f.strasse.value, plzOrt: f.plzOrt.value,
          versand: f.versand.value, email: f.email.value.trim(), telefon: f.telefon.value, gartengroesse: werte.groesse ?? 0 },
      };
      try { ergebnis = await api('POST', '/api/admin/paechterwechsel', daten); } catch (e) { handleErr(e); return false; }
    } },
  ]);
  if (!ok || !ergebnis) return false;
  S.uebergaben = null; S.papierkorb = null;
  await reload();
  const u = ergebnis.uebergabe;
  const weiter = await modal('Pächterwechsel erledigt', h('div', null,
    h('p', null, `Garten ${u.gartennr} gehört ab ${deDate(u.datum)} ${u.neuName}.`),
    h('ul', null,
      h('li', null, 'Das Übergabeprotokoll zum Unterschreiben kannst du jetzt drucken (auch später in der Gartenakte oder unter »Pächter« → Gartenübergaben).'),
      h('li', null, `Die Schlussrechnung für ${u.altName} kannst du gleich ausstellen oder später zusammen mit den anderen Rechnungen.`))),
  [{ label: 'Schließen', value: '' }, { label: 'Willkommensmappe', value: 'willkommen' }, { label: 'Schlussrechnung ausstellen', value: 'rechnung' }, { label: 'Protokoll drucken', cls: 'primary', value: 'protokoll' }]);
  if (weiter === 'protokoll') await protokollDrucken(u);
  if (weiter === 'willkommen') await willkommenDrucken({ id: u.neuId, name: u.neuName });
  if (weiter === 'rechnung') { try { await issue([u.altId]); } catch (e) { handleErr(e); } }
  return true;
}

const protokollDrucken = (u) => pdfDrucken({
  url: `/api/admin/uebergabe/${u.id}/protokoll`, oeffnenUrl: `/api/admin/uebergabe/${u.id}/protokoll/oeffnen`,
  titel: `Übergabeprotokoll Garten ${u.gartennr}`, meldung: () => 'Das Übergabeprotokoll wurde im PDF-Programm geöffnet – dort drucken.',
});

async function loadUebergaben() {
  if (S.uebergabenLoading) return;
  S.uebergabenLoading = true;
  try { S.uebergaben = await api('GET', '/api/admin/uebergaben'); } catch (e) { handleErr(e); S.uebergaben = []; }
  S.uebergabenLoading = false;
  render();
}

function uebergabenCard() {
  if (S.uebergaben == null) { if (!S.uebergabenLoading) loadUebergaben(); return null; }
  if (!S.uebergaben.length) return null;
  return h('div', { class: 'card', style: 'margin-top:16px' },
    h('h3', { style: 'margin-top:0' }, 'Gartenübergaben'),
    h('div', { class: 'tablewrap' }, h('table', null,
      h('thead', null, h('tr', null, ['Datum', 'Garten', 'Bisher', 'Neu', 'Ablöse', ''].map((x) => h('th', null, x)))),
      h('tbody', null, S.uebergaben.map((u) => h('tr', null,
        h('td', null, deDate(u.datum)), h('td', null, u.gartennr), h('td', null, u.altName), h('td', null, u.neuName),
        h('td', { class: 'num' }, u.abloese ? eur(u.abloese) : '–'),
        h('td', null, h('button', { class: 'btn small', onclick: () => protokollDrucken(u) }, 'Protokoll drucken'))))))));
}

// ------------------------------------------------------------------ Admin: Warteliste

async function loadWarteliste() {
  if (S.wartelisteLoading) return;
  S.wartelisteLoading = true;
  try { S.warteliste = await api('GET', '/api/admin/warteliste'); } catch (e) { handleErr(e); S.warteliste = []; }
  S.wartelisteLoading = false;
  render();
}

function bewerberDialog(b) {
  const isNew = !b;
  const cur = b || { seit: todayIso(), anrede: '', name: '', strasse: '', plzOrt: S.state.settings.ort, email: '', telefon: '', wunsch: '', notiz: '', bestaetigtAm: '' };
  const t = (val, ph, typ) => h('input', { type: typ || 'text', value: val || '', placeholder: ph || '', maxlength: 100, autocomplete: 'off' });
  const f = {
    seit: t(cur.seit, '', 'date'), bestaetigt: t(cur.bestaetigtAm, '', 'date'),
    anrede: h('select', { value: cur.anrede }, ['Herr', 'Frau', 'Familie', 'Herr und Frau', ''].map((a) => h('option', { value: a, selected: a === cur.anrede }, a || '(keine)'))),
    name: t(cur.name), strasse: t(cur.strasse), plzOrt: t(cur.plzOrt),
    email: t(cur.email, 'name@beispiel.de', 'email'), telefon: t(cur.telefon, '', 'tel'),
    wunsch: t(cur.wunsch, 'z. B. klein, mit Laube'),
    notiz: h('textarea', { rows: 2, maxlength: 300, style: 'width:100%' }, cur.notiz || ''),
    bAm: t(cur.besichtigungAm, '', 'date'), bUm: t(cur.besichtigungUm, '', 'time'), bGarten: t(cur.besichtigungGarten, 'z. B. 12'),
    bErg: h('select', null, [['', 'noch offen'], ['besichtigt', 'besichtigt'], ['möchte den Garten', 'möchte den Garten'], ['kein Interesse', 'kein Interesse'], ['nicht erschienen', 'nicht erschienen']]
      .map(([v, l]) => h('option', { value: v, selected: v === (cur.besichtigungErgebnis || '') }, l))),
  };
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const body = h('div', { class: 'form' },
    fld('Angemeldet am *', f.seit, 'bestimmt den Platz auf der Liste'), fld('Interesse zuletzt bestätigt am', f.bestaetigt),
    fld('Anrede', f.anrede), fld('Name *', f.name), fld('Straße und Hausnummer', f.strasse), fld('PLZ und Ort', f.plzOrt),
    fld('E-Mail-Adresse', f.email), fld('Telefon', f.telefon), fld('Wunsch', f.wunsch, null, true),
    fld('Notiz (nur intern – bitte keine sensiblen Angaben)', f.notiz, null, true),
    h('div', { class: 'section-title' }, 'Besichtigung eines freien Gartens'),
    fld('Besichtigung am', f.bAm), fld('Uhrzeit', f.bUm), fld('Garten', f.bGarten), fld('Ergebnis', f.bErg));
  return modal(isNew ? 'Auf die Warteliste setzen' : `Warteliste – ${cur.name}`, body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const data = { seit: f.seit.value, bestaetigtAm: f.bestaetigt.value, anrede: f.anrede.value, name: f.name.value, strasse: f.strasse.value, plzOrt: f.plzOrt.value,
        email: f.email.value.trim(), telefon: f.telefon.value, wunsch: f.wunsch.value, notiz: f.notiz.value.trim(),
        besichtigungAm: f.bAm.value, besichtigungUm: f.bUm.value, besichtigungGarten: f.bGarten.value, besichtigungErgebnis: f.bErg.value };
      try {
        if (isNew) await api('POST', '/api/admin/warteliste', data);
        else await api('PUT', `/api/admin/warteliste/${cur.id}`, data);
        toast('Gespeichert', 'ok');
      } catch (e) { handleErr(e); return false; }
    } },
  ]);
}

async function gartenVergeben(b) {
  const st = S.state;
  const gaerten = [...st.paechter].filter((p) => !p.ausgeschiedenAm && p.gartennr).sort((x, y) => x.gartennr.localeCompare(y.gartennr, 'de', { numeric: true }));
  if (!gaerten.length) { toast('Es gibt keinen Garten mit Gartennummer, der übergeben werden könnte.', 'err'); return; }
  const sel = h('select', { style: 'min-width:280px' }, gaerten.map((p) => h('option', { value: p.id }, `Garten ${p.gartennr} – bisher ${p.name}`)));
  const ok = await modal(`Garten an ${b.name} vergeben`, h('div', null,
    h('p', null, 'Welcher Garten wird frei?'), sel,
    h('p', { class: 'hint' }, 'Im nächsten Schritt trägst du Übergabetag und Zählerstände ein. Die Angaben von der Warteliste sind dort schon ausgefüllt.')),
  [{ label: 'Abbrechen', value: false }, { label: 'Weiter', cls: 'primary', value: true }]);
  if (!ok) return;
  const alt = gaerten.find((p) => p.id === sel.value);
  if (await paechterwechselDialog(alt, b)) { S.warteliste = null; render(); }
}

function adminWarteliste() {
  if (S.warteliste == null) {
    if (!S.wartelisteLoading) loadWarteliste();
    return h('p', { class: 'hint' }, 'Wird geladen …');
  }
  const l = S.warteliste;
  const nachfragen = l.filter((b) => b.nachfragen).length;
  const neu = async () => { if (await bewerberDialog(null)) { S.warteliste = null; render(); } };
  const bestaetigen = async (b) => {
    try { await api('PUT', `/api/admin/warteliste/${b.id}`, { ...b, bestaetigtAm: todayIso() }); toast('Interesse als bestätigt vermerkt', 'ok'); S.warteliste = null; render(); } catch (e) { handleErr(e); }
  };
  const loeschen = async (b) => {
    if (!(await confirmBox(`${b.name} von der Warteliste löschen? Die Angaben werden auch aus den Sicherungen entfernt. Das kann nicht rückgängig gemacht werden.`, 'Löschen', true))) return;
    try { await api('DELETE', `/api/admin/warteliste/${b.id}`); toast('Gelöscht', 'ok'); S.warteliste = null; render(); } catch (e) { handleErr(e); }
  };
  return h('div', null,
    h('p', { class: 'hint' }, 'Interessenten für einen Garten, in der Reihenfolge ihrer Anmeldung. Wird ein Garten frei, klickst du beim nächsten auf »Garten vergeben«: Der Pächterwechsel übernimmt Name und Kontakt und nimmt den Eintrag von der Liste.'),
    nachfragen ? h('div', { class: 'banner warn' }, `${anz(nachfragen, 'Eintrag', 'Einträge')} seit über 2 Jahren nicht bestätigt. Bitte nachfragen, ob noch Interesse besteht, und sonst löschen – nicht mehr benötigte Daten sollen nicht unbegrenzt liegen bleiben.`) : null,
    h('div', { class: 'toolbar' },
      h('button', { class: 'btn primary', onclick: neu }, '+ Auf die Warteliste setzen'),
      h('span', { class: 'stat' }, `${anz(l.length, 'Interessent', 'Interessenten')}`)),
    l.length ? h('div', { class: 'tablewrap' }, h('table', null,
      h('thead', null, h('tr', null, ['Platz', 'Name und Wunsch', 'Kontakt', 'Angemeldet', 'Besichtigung', ''].map((x) => h('th', null, x)))),
      h('tbody', null, l.map((b) => h('tr', null,
        h('td', { class: 'num' }, `${b.platz}.`),
        h('td', { class: 'wrap' }, h('b', null, [b.anrede, b.name].filter(Boolean).join(' ')), b.notiz ? h('span', { title: b.notiz, style: 'margin-left:4px;cursor:help', class: 'notiz-zeichen' }, icon('bericht', 14)) : null,
          b.wunsch ? h('div', { class: 'hint' }, b.wunsch) : null),
        h('td', { class: 'wrap klein' }, [b.telefon, b.email, [b.strasse, b.plzOrt].filter(Boolean).join(', ')].filter(Boolean).map((x, i) => [i ? h('br') : null, x])),
        h('td', { class: 'klein' }, deDate(b.seit), h('div', { class: 'hint' }, b.bestaetigtAm ? `bestätigt ${deDate(b.bestaetigtAm)}` : 'nicht bestätigt'),
          b.nachfragen ? h('span', { class: 'badge warn' }, 'nachfragen') : null),
        h('td', { class: 'klein' }, b.besichtigungAm ? [`${deDate(b.besichtigungAm)}${b.besichtigungUm ? ' ' + b.besichtigungUm : ''}`,
          h('div', { class: 'hint' }, b.besichtigungGarten ? 'Garten ' + b.besichtigungGarten : ''),
          h('span', { class: 'badge ' + (b.besichtigungErgebnis === 'möchte den Garten' ? 'ok' : b.besichtigungErgebnis ? 'neu' : 'warn') }, b.besichtigungErgebnis || (b.besichtigungAm >= todayIso() ? 'geplant' : 'Ergebnis eintragen'))] : '–'),
        h('td', { class: 'aktionen' },
          h('button', { class: 'btn small primary', onclick: () => gartenVergeben(b) }, 'Garten vergeben'), ' ',
          h('button', { class: 'btn small', onclick: async () => { if (await bewerberDialog(b)) { S.warteliste = null; S.dashWarteliste = null; render(); } } }, 'Bearbeiten'), ' ',
          mehrMenue([
            ...[['angebot', 'Brief: Garten anbieten'], ['nachfrage', 'Brief: Interesse nachfragen'], ['bestaetigung', 'Brief: Anmeldung bestätigen'], ['aufnahmeantrag', 'Aufnahmeantrag (Vordruck)']].map(([art, label]) => ({
              label, aktion: () => pdfDrucken({ url: `/api/admin/warteliste/${encodeURIComponent(b.id)}/brief?art=${art}`, oeffnenUrl: `/api/admin/warteliste/${encodeURIComponent(b.id)}/brief/oeffnen?art=${art}`,
                titel: label, meldung: () => 'Der Brief wurde im PDF-Programm geöffnet – dort drucken oder als PDF verschicken.' }) })),
            { label: 'Interesse heute bestätigt', aktion: () => bestaetigen(b) },
            { label: 'Auskunft nach DSGVO (Datei)', aktion: () => { location.href = `/api/admin/warteliste/${encodeURIComponent(b.id)}/auskunft`; } },
            { label: 'Von der Liste löschen', gefahr: true, aktion: () => loeschen(b) }])))))))
      : leer('personen', 'Niemand auf der Warteliste', 'Wer sich für einen Garten interessiert, kommt hier auf die Liste. Wird ein Garten frei, siehst du sofort, wer als Nächstes dran ist.', '+ Auf die Warteliste setzen', neu));
}

// ------------------------------------------------------------------ Tab: Mängel (Gartenbegehung)

const MANGEL_ARTEN = ['Pflege und Unkraut', 'Hecke und Bäume', 'Laube und Bauten', 'Abfall und Lagerung', 'Wege und Zäune', 'Wasser und Strom', 'Sonstiges'];

async function loadMaengel() {
  if (S.maengelLoading) return;
  S.maengelLoading = true;
  try { S.maengel = await api('GET', '/api/admin/maengel'); } catch (e) { handleErr(e); S.maengel = []; }
  S.maengelLoading = false;
  render();
}

const begehungslisteDrucken = () => pdfDrucken({
  url: '/api/admin/begehungsliste', oeffnenUrl: '/api/admin/begehungsliste/oeffnen',
  titel: 'Begehungsliste', meldung: () => 'Die Begehungsliste wurde im PDF-Programm geöffnet – dort drucken und zur Begehung mitnehmen.',
});
// Schnell-Aushang für den Schaukasten (wird nicht gespeichert)
const AUSHANG_VORLAGEN = [
  { name: 'Leer', titel: '', text: '' },
  { name: 'Wasser gesperrt', titel: 'Wasser abgestellt', text: 'Wegen Arbeiten an der Leitung ist das Wasser in der Anlage abgestellt.\nWir bitten um Verständnis.' },
  { name: 'Arbeitseinsatz', titel: 'Gemeinschaftsarbeit', text: 'Treffpunkt: Vereinsheim\nBitte Arbeitshandschuhe und, wenn vorhanden, eigenes Werkzeug mitbringen.\nDie Stunden werden auf die Pflichtstunden angerechnet.' },
  { name: 'Fundsache', titel: 'Fundsache', text: 'In der Anlage wurde gefunden:\n\nAbzuholen beim Vorstand.' },
  { name: 'Weg gesperrt', titel: 'Weg gesperrt', text: 'Der Weg ist wegen Bauarbeiten vorübergehend gesperrt. Bitte die Umleitung nutzen.' },
  { name: 'Container', titel: 'Grünschnitt-Container', text: 'Ein Container für Grünschnitt steht am Eingang bereit.\nBitte nur Grünschnitt, keine Erde, Steine oder Abfälle.' },
];
async function aushangDialog() {
  const f = {
    vorlage: h('select', null, AUSHANG_VORLAGEN.map((v, i) => h('option', { value: i }, v.name))),
    titel: h('input', { type: 'text', maxlength: 80, placeholder: 'z. B. Wasser abgestellt' }),
    datum: h('input', { type: 'date' }),
    text: h('textarea', { rows: 7, maxlength: 1500, style: 'width:100%' }),
    zusatz: h('input', { type: 'text', maxlength: 120, placeholder: 'z. B. Rückfragen: Wasserwart, Garten 12' }),
  };
  f.vorlage.addEventListener('change', () => { const v = AUSHANG_VORLAGEN[Number(f.vorlage.value)]; f.titel.value = v.titel; f.text.value = v.text; });
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const drucken = async () => {
    if (!f.titel.value.trim() || !f.text.value.trim()) { toast('Bitte Überschrift und Text eintragen.', 'err'); return false; }
    await pdfDrucken({ url: '/api/admin/aushang', oeffnenUrl: '/api/admin/aushang/oeffnen', titel: 'Aushang',
      body: { titel: f.titel.value, text: f.text.value, datum: f.datum.value, zusatz: f.zusatz.value },
      meldung: () => 'Der Aushang wurde im PDF-Programm geöffnet – dort drucken.' });
    return false;
  };
  await modal('Aushang für den Schaukasten', h('div', { class: 'form' },
    fld('Vorlage', f.vorlage), fld('Datum (optional)', f.datum, 'erscheint groß unter der Überschrift'),
    fld('Überschrift *', f.titel, null, true), fld('Text *', f.text, 'Für alle sichtbar – keine Namen oder persönlichen Angaben.', true),
    fld('Zusatzzeile (optional)', f.zusatz, null, true),
    h('p', { class: 'hint wide' }, 'Der Aushang wird nicht gespeichert. Für Mitteilungen, die auch per Post oder E-Mail an alle gehen, gibt es die Rundschreiben.')),
  [{ label: 'Schließen', value: false }, { label: 'Drucken', cls: 'primary', action: drucken }]);
}
const saisonlisteDrucken = (art) => pdfDrucken({
  url: '/api/admin/saisonliste?art=' + art, oeffnenUrl: '/api/admin/saisonliste/oeffnen?art=' + art,
  titel: art === 'herbst' ? 'Saison-Liste Herbst' : 'Saison-Liste Frühjahr',
  meldung: () => 'Die Saison-Liste wurde im PDF-Programm geöffnet – dort drucken und zum Rundgang mitnehmen.',
});
const MAENGEL_STUFEN = [
  { key: 'erst', knopf: 'Aufforderung schreiben', titel: 'Aufforderung' },
  { key: 'erinnerung', knopf: 'Erinnerung schreiben', titel: 'Erinnerung' },
  { key: 'zweit', knopf: 'Zweitschreiben (letzte Frist)', titel: 'Zweitschreiben' }];
const maengelschreibenDrucken = (nr, stufe, frist) => pdfDrucken({
  url: `/api/admin/maengelschreiben?gartennr=${encodeURIComponent(nr)}&stufe=${stufe}&frist=${frist}`,
  oeffnenUrl: `/api/admin/maengelschreiben/oeffnen?gartennr=${encodeURIComponent(nr)}&stufe=${stufe}&frist=${frist}`,
  titel: `Aufforderung Garten ${nr}`, meldung: () => 'Das Schreiben wurde im PDF-Programm geöffnet – dort drucken oder als PDF verschicken.',
});
const mangelStand = (m) => MAENGEL_STUFEN.reduce((hoch, x, i) => (m.stufen && m.stufen[x.key] ? i : hoch), -1);
// Nächstes Schreiben im Fahrplan: neue Frist abfragen, Schreiben öffnen und den Versand vermerken.
async function maengelSchreiben(nr, stufe) {
  const frist = h('input', { type: 'date', value: isoInTagen(14) });
  const ok = await modal(stufe.titel + ' – Garten ' + nr, h('div', null,
    h('p', { class: 'hint' }, 'Bis wann sollen die Mängel beseitigt sein? Die Frist gilt für alle offenen Mängel dieses Gartens. Das Schreiben öffnet sich im PDF-Programm. Danach vermerkt das Programm, dass es verschickt wurde, und erinnert an den nächsten Schritt.'),
    h('label', null, 'Neue Frist ', frist)),
  [{ label: 'Abbrechen', value: false }, { label: 'Schreiben erstellen', cls: 'primary', value: true }]);
  if (!ok || !frist.value) return;
  await maengelschreibenDrucken(nr, stufe.key, frist.value);
  try { await api('POST', '/api/admin/maengel-schreiben', { gartennr: nr, stufe: stufe.key, frist: frist.value }); S.maengel = null; render(); } catch (e) { handleErr(e); }
}

function mangelDialog(m, gartennr) {
  const isNew = !m || !m.id;
  const cur = { gartennr: gartennr || '', datum: todayIso(), art: merkeLesen('mangel-art', 'Pflege und Unkraut'), beschreibung: '', frist: '', erledigtAm: '', ...(m || {}) };
  const f = {
    gartennr: h('input', { type: 'text', value: cur.gartennr, maxlength: 20, style: 'width:6em', placeholder: 'z. B. 12' }),
    datum: h('input', { type: 'date', value: cur.datum }),
    art: h('select', null, MANGEL_ARTEN.map((a) => h('option', { value: a, selected: a === cur.art }, a))),
    beschreibung: h('textarea', { rows: 3, maxlength: 300, style: 'width:100%', placeholder: 'Was ist zu beseitigen? z. B. Hecke zur Wegseite höher als 1,25 m, bitte zurückschneiden' }, cur.beschreibung),
    frist: h('input', { type: 'date', value: cur.frist || (isNew ? isoInTagen(14) : '') }),
    erledigt: h('input', { type: 'date', value: cur.erledigtAm }),
    foto: h('input', { type: 'file', accept: 'image/jpeg,image/png' }),
    fotoWeg: h('input', { type: 'checkbox' }),
  };
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const hatFoto = !isNew && !!cur.foto;
  const body = h('div', { class: 'form' },
    fld('Garten *', f.gartennr), fld('Begehung am *', f.datum), fld('Art', f.art),
    fld('Beschreibung *', f.beschreibung, null, true),
    fld('Frist zur Beseitigung', f.frist), fld('Erledigt am', f.erledigt, 'leer = noch offen'),
    fld(hatFoto ? 'Foto ersetzen' : 'Foto (optional)', h('div', null,
      hatFoto ? h('img', { src: `/api/admin/maengel/${encodeURIComponent(cur.id)}/foto?t=${Date.now()}`, alt: 'Foto des Mangels', style: 'max-width:12em;max-height:9em;display:block;margin-bottom:.4em;border:1px solid var(--line, #ccc)' }) : null,
      f.foto,
      hatFoto ? h('label', { class: 'inline' }, f.fotoWeg, ' Foto entfernen') : null),
    'Erscheint im Schreiben an den Pächter. Bitte nur den Mangel fotografieren, keine Personen. Standortdaten werden entfernt.', true));
  return modal(isNew ? 'Mangel eintragen' : 'Mangel bearbeiten', body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const data = { gartennr: f.gartennr.value, datum: f.datum.value, art: f.art.value, beschreibung: f.beschreibung.value, frist: f.frist.value, erledigtAm: f.erledigt.value };
      merkeSchreiben('mangel-art', f.art.value);
      try {
        const gespeichert = isNew ? await api('POST', '/api/admin/maengel', data) : await api('PUT', `/api/admin/maengel/${cur.id}`, data);
        const id = isNew ? gespeichert.id : cur.id;
        if (f.foto.files && f.foto.files[0]) {
          const fd = new FormData();
          fd.append('file', f.foto.files[0]);
          await api('POST', `/api/admin/maengel/${encodeURIComponent(id)}/foto`, fd, true);
        } else if (hatFoto && f.fotoWeg.checked) {
          await api('DELETE', `/api/admin/maengel/${encodeURIComponent(id)}/foto`);
        }
        toast('Gespeichert', 'ok');
      } catch (e) { handleErr(e); return false; }
    } }]);
}

async function loadNutzung() {
  if (S.nutzungLoading) return;
  S.nutzungLoading = true;
  try { S.nutzung = await api('GET', '/api/admin/nutzung'); } catch (e) { handleErr(e); S.nutzung = []; }
  S.nutzungLoading = false;
  render();
}

function nutzungDialog(nr, letzte) {
  const zahl = (v) => h('input', { type: 'text', inputmode: 'decimal', value: v == null ? '' : String(v), style: 'width:5em' });
  const f = { datum: h('input', { type: 'date', value: todayIso() }), anbau: zahl(letzte ? letzte.anbau : ''), erholung: zahl(letzte ? letzte.erholung : ''),
    bebauung: zahl(letzte ? letzte.bebauung : ''), notiz: h('input', { type: 'text', maxlength: 200, placeholder: 'z. B. Rasen statt Beete hinter der Laube' }) };
  const summe = h('b', null);
  const rechnen = () => {
    const w = [f.anbau, f.erholung, f.bebauung].map((x) => parseNum(x.value) || 0);
    const s = w.reduce((a, b) => a + b, 0);
    summe.textContent = `Summe ${nfFlex.format(s)} %${w[0] < 100 / 3 ? ' · Anbau unter einem Drittel' : ''}`;
    summe.className = w[0] < 100 / 3 || Math.abs(s - 100) > 10 ? 'warn-text' : '';
  };
  [f.anbau, f.erholung, f.bebauung].forEach((x) => x.addEventListener('input', rechnen));
  rechnen();
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  return modal(`Nutzung Garten ${nr} schätzen`, h('div', null,
    h('p', { class: 'hint' }, 'Grob geschätzte Anteile der Gartenfläche in Prozent. In der Regel soll mindestens ein Drittel dem Anbau von Obst und Gemüse dienen.'),
    h('div', { class: 'form' }, fld('Begehung am', f.datum, null, true),
      fld('Anbau %', f.anbau, 'Obst, Gemüse, Kräuter, Beeren'), fld('Erholung %', f.erholung, 'Rasen, Zierpflanzen, Sitzplatz'), fld('Bebauung %', f.bebauung, 'Laube, Wege, befestigte Flächen'),
      fld('Notiz', f.notiz, null, true)), h('p', null, summe)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    try {
      await api('POST', '/api/admin/nutzung', { gartennr: nr, datum: f.datum.value, anbau: parseNum(f.anbau.value) ?? 0, erholung: parseNum(f.erholung.value) ?? 0,
        bebauung: parseNum(f.bebauung.value) ?? 0, notiz: f.notiz.value });
      toast('Gespeichert', 'ok');
    } catch (e) { handleErr(e); return false; }
  } }]);
}

function nutzungKarte() {
  if (S.nutzung == null) { if (!S.nutzungLoading) loadNutzung(); return null; }
  const l = S.nutzung;
  const geschaetzt = l.filter((z) => z.letzte), unter = l.filter((z) => z.unterDrittel);
  const balken = (n) => h('div', { class: 'nutzung-balken', title: `Anbau ${n.anbau} % · Erholung ${n.erholung} % · Bebauung ${n.bebauung} %` },
    h('span', { class: 'n-anbau', style: `width:${n.anbau}%` }), h('span', { class: 'n-erholung', style: `width:${n.erholung}%` }), h('span', { class: 'n-bebauung', style: `width:${n.bebauung}%` }),
    h('i', { class: 'n-drittel' }));
  const schaetzen = async (z) => { if (await nutzungDialog(z.gartennr, z.letzte)) { S.nutzung = null; render(); } };
  return h('details', { class: 'card', style: 'margin-top:12px', open: unter.length > 0 },
    h('summary', null, h('b', null, 'Kleingärtnerische Nutzung (Drittelregelung)'),
      h('span', { class: 'hint' }, ` · ${geschaetzt.length} von ${l.length} Gärten geschätzt`), unter.length ? h('span', { class: 'badge warn', style: 'margin-left:8px' }, `${unter.length} unter einem Drittel Anbau`) : null),
    h('p', { class: 'hint', style: 'margin-top:10px' }, 'Bei der Begehung grob schätzen, wie sich die Fläche aufteilt. Die Linie im Balken markiert ein Drittel. Das ist eine Gesprächsgrundlage, keine Vermessung – maßgeblich sind Satzung, Gartenordnung und die Vorgaben eures Verbands.'),
    h('div', { class: 'nutzung-legende hint' }, h('span', { class: 'n-anbau' }), ' Anbau ', h('span', { class: 'n-erholung' }), ' Erholung ', h('span', { class: 'n-bebauung' }), ' Bebauung'),
    h('div', { class: 'tablewrap' }, h('table', null,
      h('thead', null, h('tr', null, ['Garten', 'Pächter', 'Aufteilung', 'Anbau', 'Geschätzt', ''].map((x) => h('th', null, x)))),
      h('tbody', null, l.map((z) => h('tr', null, h('td', null, z.gartennr), h('td', null, z.name),
        h('td', { style: 'min-width:180px' }, z.letzte ? balken(z.letzte) : h('span', { class: 'hint' }, 'noch nicht geschätzt')),
        h('td', { class: 'num' }, z.letzte ? [`${z.letzte.anbau} %`, z.unterDrittel ? h('span', { class: 'badge warn', style: 'margin-left:6px' }, 'unter ⅓') : null] : '–'),
        h('td', { class: 'klein' }, z.letzte ? deDate(z.letzte.datum) : ''),
        h('td', null, h('button', { class: 'btn small', onclick: () => schaetzen(z) }, 'Schätzen'))))))));
}

// Begehung durchführen: Garten für Garten Mängel ankreuzen und die Nutzung schätzen, gespeichert wird beim Weitergehen.
async function begehungDurchfuehren() {
  let nutzung;
  try { nutzung = await api('GET', '/api/admin/nutzung'); } catch (e) { handleErr(e); return; }
  if (!nutzung.length) { toast('Es gibt noch keine Gärten mit Pächtern.', 'err'); return; }
  const offen = (S.maengel || []).filter((m) => !m.erledigtAm);
  let i = 0, gespeichert = 0;
  const datum = todayIso();
  const frist = isoInTagen(14);
  const box = h('div');
  const zeichne = () => {
    const z = nutzung[i];
    const alt = offen.filter((m) => m.gartennr === z.gartennr);
    const arten = MANGEL_ARTEN.map((a) => ({ art: a, cb: h('input', { type: 'checkbox' }), text: h('input', { type: 'text', maxlength: 300, placeholder: 'Was ist zu tun?', style: 'display:none;width:100%' }) }));
    arten.forEach((a) => a.cb.addEventListener('change', () => { a.text.style.display = a.cb.checked ? '' : 'none'; if (a.cb.checked) a.text.focus(); }));
    const zahl = (v) => h('input', { type: 'text', inputmode: 'decimal', value: v == null ? '' : String(v), style: 'width:5em' });
    const l = z.letzte;
    const nf = { anbau: zahl(l ? l.anbau : ''), erholung: zahl(l ? l.erholung : ''), bebauung: zahl(l ? l.bebauung : '') };
    const speichern = async () => {
      // erst die Nutzung (wird vom Server geprüft), dann die Mängel; gespeicherte Mängel verlieren ihr Häkchen,
      // damit ein erneuter Versuch nach einem Fehler nichts doppelt anlegt
      try {
        const w = Object.fromEntries(Object.entries(nf).map(([k, el]) => [k, el.value.trim() === '' ? null : parseNum(el.value)]));
        if (Object.values(w).some((v) => Number.isNaN(v))) { toast('Bitte die Nutzung in Prozent als Zahl eintragen.', 'err'); return false; }
        const geaendert = !l ? Object.values(w).some((v) => v != null) : ['anbau', 'erholung', 'bebauung'].some((k) => (w[k] ?? 0) !== l[k]);
        if (geaendert) {
          await api('POST', '/api/admin/nutzung', { gartennr: z.gartennr, datum, anbau: w.anbau ?? 0, erholung: w.erholung ?? 0, bebauung: w.bebauung ?? 0, notiz: '' });
          z.letzte = { datum, anbau: w.anbau ?? 0, erholung: w.erholung ?? 0, bebauung: w.bebauung ?? 0 };
        }
        for (const a of arten.filter((x) => x.cb.checked)) {
          await api('POST', '/api/admin/maengel', { gartennr: z.gartennr, datum, art: a.art, beschreibung: a.text.value.trim() || a.art, frist, erledigtAm: '' });
          a.cb.checked = false; a.text.style.display = 'none';
          offen.push({ gartennr: z.gartennr, art: a.art, beschreibung: a.text.value.trim() || a.art, frist });
          gespeichert++;
        }
      } catch (e) { handleErr(e); return false; }
      return true;
    };
    const gehe = async (schritt, sichern) => {
      if (sichern && !(await speichern())) return;
      i = Math.min(nutzung.length - 1, Math.max(0, i + schritt));
      zeichne();
    };
    const letzter = i === nutzung.length - 1;
    box.replaceChildren(...h('div', null,
      h('div', { class: 'begehung-kopf' }, h('span', { class: 'big' }, `Garten ${z.gartennr}`), h('span', null, z.name), h('span', { class: 'hint' }, `${i + 1} von ${nutzung.length}`)),
      alt.length ? h('div', { class: 'banner warn', style: 'margin-top:8px' }, 'Noch offen: ', alt.map((m) => `${m.art} – ${m.beschreibung}${m.frist ? ` (Frist ${deDate(m.frist)})` : ''}`).join(' · ')) : null,
      h('h4', null, 'Neue Mängel'),
      h('div', { class: 'begehung-maengel' }, arten.map((a) => h('div', null, h('label', null, a.cb, ' ', a.art), a.text))),
      h('h4', null, 'Nutzung in % (Drittelregelung)'),
      h('div', { class: 'toolbar' }, h('label', null, 'Anbau ', nf.anbau), h('label', null, 'Erholung ', nf.erholung), h('label', null, 'Bebauung ', nf.bebauung),
        l ? h('span', { class: 'hint' }, `zuletzt ${deDate(l.datum)}`) : null),
      h('div', { class: 'toolbar', style: 'margin-top:14px' },
        h('button', { class: 'btn', disabled: i === 0, onclick: () => gehe(-1, false) }, '← Zurück'),
        h('div', { class: 'spacer' }),
        h('button', { class: 'btn primary', onclick: async () => { if (letzter) { if (await speichern()) { box.replaceChildren(h('p', null, h('b', null, 'Begehung abgeschlossen.'), ` ${gespeichert} Mängel eingetragen.`)); } } else gehe(1, true); } },
          letzter ? 'Speichern und abschließen' : 'Speichern und weiter →')),
      h('p', { class: 'hint' }, `Neue Mängel bekommen die Frist ${deDate(frist)}. Ohne Häkchen und ohne Änderung wird nichts gespeichert.`)).childNodes);
  };
  zeichne();
  await breiterDialog(`Begehung am ${deDate(datum)}`, box, [{ label: 'Beenden', value: true }]);
  if (gespeichert) toast(`${gespeichert} Mängel eingetragen`, 'ok');
  S.maengel = null; S.nutzung = null; S.aufgaben = null;
  render();
}

function adminBegehung() {
  if (!modulAn('bauten')) return adminMaengel();
  const bauten = S.maengelSeite === 'bauten';
  return h('div', null, reiterLeiste([
    { label: 'Mängel und Nutzung', aktiv: !bauten, geh: () => { S.maengelSeite = 'maengel'; render(); } },
    { label: 'Laubenkataster', aktiv: bauten, geh: () => { S.maengelSeite = 'bauten'; render(); } }]),
  bauten ? adminBauten() : adminMaengel());
}

// ---- Laubenkataster: Bauten je Garten, Grenze 24 m² für Laube mit überdachtem Freisitz (§ 3 BKleingG)

function bauwerkDialog(b, gartennr) {
  const d = S.bauten;
  const f = {
    garten: h('input', { type: 'text', maxlength: 20, value: b ? b.gartennr : gartennr || '', style: 'width:6em' }),
    art: h('select', null, d.arten.map((a) => h('option', { value: a, selected: b ? b.art === a : a === 'Laube' }, a))),
    flaeche: h('input', { type: 'text', inputmode: 'decimal', value: b ? numIn(b.flaeche) : '', style: 'width:7em' }),
    baujahr: h('input', { type: 'number', min: 1900, max: new Date().getFullYear(), value: b && b.baujahr ? b.baujahr : '', style: 'width:7em' }),
    gen: h('select', null, [['unbekannt', 'unbekannt'], ['ja', 'ja'], ['nein', 'nein']].map(([v, t]) => h('option', { value: v, selected: (b ? b.genehmigt : 'unbekannt') === v }, t))),
    schutz: h('input', { type: 'checkbox', checked: b ? !!b.bestandsschutz : false }),
    notiz: h('input', { type: 'text', maxlength: 300, value: b ? b.notiz || '' : '' }),
  };
  const fld = (label, el, hint) => h('div', { class: 'field' }, h('label', null, label), el, hint ? h('span', { class: 'fhint' }, hint) : null);
  return modal(b ? `Garten ${b.gartennr}: ${b.art}` : 'Bauwerk eintragen', h('div', { class: 'form' },
    fld('Garten *', f.garten), fld('Art', f.art), fld('Grundfläche in m² *', f.flaeche, 'Außenmaße, z. B. 4 × 5 m = 20'), fld('Baujahr', f.baujahr),
    fld('Genehmigt', f.gen), h('div', { class: 'field' }, h('label', null, ' '), h('label', null, f.schutz, ' Bestandsschutz'), h('span', { class: 'fhint' }, 'z. B. vor 1983 rechtmäßig errichtet')),
    h('div', { class: 'field wide' }, h('label', null, 'Notiz'), f.notiz)),
  [b ? { label: 'Löschen', cls: 'danger', value: true, action: async () => {
    if (!(await confirmBox(`${b.art} in Garten ${b.gartennr} löschen?`, 'Löschen', true))) return false;
    try { S.bauten = await api('DELETE', `/api/admin/bauten/${b.id}`); } catch (e) { handleErr(e); return false; }
  } } : null, { label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    const fl = parseNum(f.flaeche.value);
    if (fl == null || Number.isNaN(fl) || fl <= 0) { toast('Bitte die Grundfläche in m² eintragen.', 'err'); return false; }
    try {
      S.bauten = await api('POST', '/api/admin/bauten', { id: b ? b.id : '', gartennr: f.garten.value, art: f.art.value, flaeche: fl,
        baujahr: Number(f.baujahr.value || 0), genehmigt: f.gen.value, bestandsschutz: f.schutz.checked, notiz: f.notiz.value });
    } catch (e) { handleErr(e); return false; }
  } }]);
}

function adminBauten() {
  if (S.bauten == null) {
    if (!S.bautenLaedt) {
      S.bautenLaedt = true;
      api('GET', '/api/admin/bauten').then((d) => { S.bauten = d; }).catch((e) => { handleErr(e); S.bauten = { gaerten: [], arten: [], grenze: 24 }; })
        .finally(() => { S.bautenLaedt = false; render(); });
    }
    return h('p', { class: 'hint' }, 'Wird geladen …');
  }
  const d = S.bauten;
  const nachher = async (p) => { if (await p) { S.aufgaben = null; render(); } };
  const ueber = d.gaerten.filter((g) => g.ueber).length;
  const erfasst = d.gaerten.filter((g) => g.bauten.length).length;
  return h('div', null,
    h('p', { class: 'hint' }, `Lauben, Freisitze, Gewächshäuser und Schuppen je Garten. Laube und überdachter Freisitz zusammen dürfen höchstens ${nfFlex.format(d.grenze)} m² Grundfläche haben (§ 3 Abs. 2 Bundeskleingartengesetz); ältere, rechtmäßig errichtete Lauben haben oft Bestandsschutz. Die Bauten gehören zum Garten und bleiben beim Pächterwechsel erhalten.`),
    h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: () => nachher(bauwerkDialog(null)) }, '+ Bauwerk'),
      h('button', { class: 'btn', onclick: () => pdfDrucken({ url: '/api/admin/bauten/liste', oeffnenUrl: '/api/admin/bauten/liste/oeffnen', titel: 'Laubenkataster', meldung: () => 'Das Laubenkataster wurde im PDF-Programm geöffnet.' }) }, 'Liste drucken'),
      h('div', { class: 'spacer' }), h('span', { class: 'stat' }, `${erfasst} von ${d.gaerten.length} Gärten erfasst`),
      ueber ? h('span', { class: 'badge warn' }, `${ueber} über ${nfFlex.format(d.grenze)} m²`) : null),
    d.gaerten.length ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Garten', 'Pächter', 'Laube + Freisitz', 'Bauten', ''].map((t) => h('th', null, t)))),
      h('tbody', null, d.gaerten.map((g) => h('tr', null,
        h('td', null, g.gartennr), h('td', null, g.name || h('span', { class: 'hint' }, 'frei')),
        h('td', null, g.laube ? `${nfFlex.format(g.laube)} m²` : '–', ' ',
          g.ueber ? h('span', { class: 'badge warn' }, 'zu groß') : g.bestandsschutz && g.laube > d.grenze ? h('span', { class: 'badge neu' }, 'Bestandsschutz') : null),
        h('td', { class: 'wrap' }, g.bauten.length ? g.bauten.map((b) => h('div', { class: 'bau-zeile' },
          h('button', { class: 'linkbtn', onclick: () => nachher(bauwerkDialog(b)) }, `${b.art} ${nfFlex.format(b.flaeche)} m²${b.baujahr ? ` (${b.baujahr})` : ''}`),
          b.genehmigt === 'nein' ? h('span', { class: 'badge err' }, 'nicht genehmigt') : b.genehmigt === 'unbekannt' ? h('span', { class: 'hint' }, ' · Genehmigung offen') : null)) : h('span', { class: 'hint' }, 'noch nichts erfasst')),
        h('td', { class: 'aktionen' }, h('button', { class: 'btn small', onclick: () => nachher(bauwerkDialog(null, g.gartennr)) }, '+ Bauwerk')))))))
      : h('div', { class: 'card' }, leer('haus', 'Noch keine Gärten', 'Sobald Pächter eingetragen sind, erscheinen ihre Gärten hier.')));
}

function adminMaengel() {
  if (S.maengel == null) {
    if (!S.maengelLoading) loadMaengel();
    return h('p', { class: 'hint' }, 'Wird geladen …');
  }
  const heute = todayIso();
  const alle = S.maengel;
  const offen = alle.filter((m) => !m.erledigtAm);
  const erledigt = alle.filter((m) => m.erledigtAm);
  const neu = async (nr) => { if (await mangelDialog(null, nr)) { S.maengel = null; render(); } };
  const bearbeiten = async (m) => { if (await mangelDialog(m)) { S.maengel = null; render(); } };
  const speichern = async (m, aenderung) => {
    try { await api('PUT', `/api/admin/maengel/${m.id}`, { gartennr: m.gartennr, datum: m.datum, art: m.art, beschreibung: m.beschreibung, frist: m.frist || '', erledigtAm: m.erledigtAm || '', ...aenderung }); S.maengel = null; render(); } catch (e) { handleErr(e); }
  };
  const loeschen = async (m) => {
    try { const r = await api('DELETE', `/api/admin/maengel/${m.id}`); S.maengel = null; render(); geloeschtToast(r, 'Mangel', () => { S.maengel = null; render(); }); } catch (e) { handleErr(e); }
  };
  const auswahl = S.maengelAuswahl || (S.maengelAuswahl = new Set());
  for (const id of [...auswahl]) if (!offen.some((m) => m.id === id)) auswahl.delete(id);
  const sammelErledigt = async () => {
    const l = offen.filter((m) => auswahl.has(m.id));
    if (!l.length) return;
    const setze = (m, erledigtAm) => api('PUT', `/api/admin/maengel/${m.id}`, { gartennr: m.gartennr, datum: m.datum, art: m.art, beschreibung: m.beschreibung, frist: m.frist || '', erledigtAm });
    try {
      for (const m of l) await setze(m, heute);
      auswahl.clear(); S.maengel = null; render();
      toast(`${anz(l.length, 'Mangel', 'Mängel')} als erledigt markiert`, 'ok', { label: 'Rückgängig', fn: async () => {
        try { for (const m of l) await setze(m, ''); S.maengel = null; render(); } catch (e) { handleErr(e); S.maengel = null; render(); }
      } });
    } catch (e) { handleErr(e); S.maengel = null; render(); }
  };
  const zeile = (m) => h('li', { class: 'mangel' + (m.erledigtAm ? ' erledigt' : '') },
    m.erledigtAm ? null : h('input', { type: 'checkbox', class: 'mangel-wahl', checked: auswahl.has(m.id), 'aria-label': `Mangel ${m.art} in Garten ${m.gartennr} auswählen`,
      onchange: (e) => { if (e.target.checked) auswahl.add(m.id); else auswahl.delete(m.id); render(); } }),
    h('div', { class: 'mangel-text' }, h('b', null, m.art), m.foto ? h('span', { class: 'badge' }, 'Foto') : null, h('div', null, m.beschreibung),
      h('div', { class: 'hint' }, `Begehung ${deDate(m.datum)}`, m.frist && !m.erledigtAm ? ` · Frist ${deDate(m.frist)}` : '', m.erledigtAm ? ` · erledigt ${deDate(m.erledigtAm)}` : '')),
    !m.erledigtAm && m.frist && m.frist < heute ? h('span', { class: 'badge warn' }, 'Frist abgelaufen') : null,
    h('div', { class: 'termin-aktionen' },
      m.erledigtAm ? h('button', { class: 'btn small', onclick: () => speichern(m, { erledigtAm: '' }) }, 'Wieder öffnen')
        : h('button', { class: 'btn small primary', onclick: () => speichern(m, { erledigtAm: heute }) }, 'Erledigt'),
      h('button', { class: 'btn small', onclick: () => bearbeiten(m) }, 'Bearbeiten'),
      mehrMenue([{ label: 'Löschen', gefahr: true, aktion: () => loeschen(m) }])));
  const gruppen = new Map();
  for (const m of offen) { if (!gruppen.has(m.gartennr)) gruppen.set(m.gartennr, []); gruppen.get(m.gartennr).push(m); }
  const garten = [...gruppen.entries()].sort((a, b) => a[0].localeCompare(b[0], 'de', { numeric: true }));
  return h('div', null,
    h('p', { class: 'hint' }, 'Beim Rundgang gefundene Mängel je Garten mit Frist. Mit der Begehungsliste gehst du durch die Anlage, hier trägst du die Mängel ein und schreibst dem Pächter eine Aufforderung. Nur der Vorstand sieht diese Liste.'),
    h('div', { class: 'toolbar' },
      h('button', { class: 'btn primary', onclick: () => neu('') }, '+ Mangel eintragen'),
      h('button', { class: 'btn', onclick: begehungslisteDrucken }, 'Begehungsliste drucken'),
      h('button', { class: 'btn', onclick: begehungDurchfuehren, title: 'Garten für Garten durchgehen und Mängel und Nutzung gleich eintragen, z. B. mit dem Laptop beim Rundgang' }, 'Begehung durchführen'),
      auswahl.size ? h('button', { class: 'btn primary', onclick: sammelErledigt }, `${auswahl.size} als erledigt markieren`) : null,
      auswahl.size ? h('button', { class: 'btn', onclick: () => { const q = [...auswahl].map(encodeURIComponent).join(','); pdfDrucken({ url: `/api/admin/maengel-liste?ids=${q}`, oeffnenUrl: `/api/admin/maengel-liste/oeffnen?ids=${q}`, titel: 'Mängelliste', meldung: () => 'Die Mängelliste wurde im PDF-Programm geöffnet – dort drucken.' }); } }, 'Auswahl drucken') : null,
      offen.length > 1 ? h('button', { class: 'btn', onclick: () => { if (auswahl.size === offen.length) auswahl.clear(); else offen.forEach((m) => auswahl.add(m.id)); render(); } }, auswahl.size === offen.length ? 'Auswahl aufheben' : 'Alle auswählen') : null,
      h('div', { class: 'spacer' }),
      h('span', { class: 'stat' }, `${offen.length} offen · ${erledigt.length} erledigt`)),
    garten.length ? garten.map(([nr, l]) => h('div', { class: 'card mangel-garten' },
      h('div', { class: 'fs-kopf' }, h('b', null, `Garten ${nr}`, l[0].name ? ` · ${l[0].name}` : ''),
        h('span', { class: 'toolbar-gruppe' },
          h('button', { class: 'btn small', onclick: () => neu(nr) }, '+ Mangel'),
          (() => {
            const stand = Math.max(-1, ...l.map(mangelStand));
            if (stand >= MAENGEL_STUFEN.length - 1) return h('span', { class: 'badge warn', title: 'Das letzte Schreiben ist verschickt. Über Abmahnung und weitere Schritte entscheidet der Vorstand.' }, 'Abmahnung beraten');
            const n = MAENGEL_STUFEN[stand + 1];
            return h('button', { class: 'btn small primary', onclick: () => maengelSchreiben(nr, n) }, n.knopf);
          })())),
      (() => {
        const hist = MAENGEL_STUFEN.map((x) => { const d = l.map((m) => m.stufen && m.stufen[x.key]).filter(Boolean).sort().pop(); return d ? `${x.titel} ${deDate(d)}` : null; }).filter(Boolean);
        return hist.length ? h('div', { class: 'hint', style: 'margin:2px 0 6px' }, 'Verschickt: ' + hist.join(' · ')) : null;
      })(),
      h('ul', { class: 'mangel-liste' }, l.map(zeile))))
      : h('div', { class: 'card' }, leer('fertig', 'Keine offenen Mängel', 'Bei der nächsten Begehung gefundene Mängel trägst du hier ein. Die Begehungsliste zum Ausdrucken liegt oben bereit.', '+ Mangel eintragen', () => neu(''))),
    nutzungKarte(),
    erledigt.length ? h('details', { class: 'card', style: 'margin-top:12px' }, h('summary', null, `Erledigt (${erledigt.length})`),
      h('ul', { class: 'mangel-liste' }, erledigt.map((m) => h('li', { class: 'mangel erledigt' }, h('div', { class: 'mangel-text' },
        h('b', null, `Garten ${m.gartennr} · ${m.art}`), h('div', null, m.beschreibung), h('div', { class: 'hint' }, `erledigt ${deDate(m.erledigtAm)}`)),
      h('div', { class: 'termin-aktionen' }, h('button', { class: 'btn small', onclick: () => speichern(m, { erledigtAm: '' }) }, 'Wieder öffnen'),
        mehrMenue([{ label: 'Löschen', gefahr: true, aktion: () => loeschen(m) }])))))) : null);
}
