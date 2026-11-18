'use strict';
/* Kleingarten-Manager – Oberfläche, Teil: Termine, Gartenakte, Berichte, Befragung. Gemeinsamer Gültigkeitsbereich mit den anderen Dateien (siehe index.html für die Reihenfolge). */

// ------------------------------------------------------------------ Tab: Termine (Jahresplaner)

const TERMIN_ARTEN = ['Arbeitseinsatz', 'Versammlung', 'Ablesung', 'Fest', 'Vorstand', 'Sonstiges'];
const MONATE = ['Januar', 'Februar', 'März', 'April', 'Mai', 'Juni', 'Juli', 'August', 'September', 'Oktober', 'November', 'Dezember'];
const WOCHENTAGE = ['So', 'Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa'];
const isoDatum = (iso) => { const [y, m, d] = iso.split('-').map(Number); return new Date(y, m - 1, d); };
const terminZeit = (t) => (t.von ? `${t.von}${t.bis ? '–' + t.bis : ''} Uhr` : 'ganztägig');

function terminDialog(t) {
  const isNew = !t || !t.id;
  const cur = { datum: todayIso(), von: '', bis: '', titel: '', art: merkeLesen('termin-art', 'Arbeitseinsatz'), ort: merkeLesen('termin-ort', ''), notiz: '', tops: '', ...(t || {}) };
  const f = {
    datum: h('input', { type: 'date', value: cur.datum }),
    von: h('input', { type: 'time', value: cur.von }), bis: h('input', { type: 'time', value: cur.bis }),
    titel: h('input', { type: 'text', value: cur.titel, maxlength: 100, placeholder: 'z. B. Frühjahrsarbeitseinsatz' }),
    art: h('select', null, TERMIN_ARTEN.map((a) => h('option', { value: a, selected: a === cur.art }, a))),
    ort: h('input', { type: 'text', value: cur.ort, maxlength: 100, placeholder: 'z. B. Vereinsheim' }),
    notiz: h('textarea', { rows: 2, maxlength: 300, style: 'width:100%', placeholder: 'z. B. Bitte Arbeitshandschuhe mitbringen' }, cur.notiz || ''),
    tops: h('textarea', { rows: 5, maxlength: 3000, style: 'width:100%', placeholder: 'Ein Punkt pro Zeile, z. B.\nBegrüßung\nKassenbericht\nVerschiedenes' }, cur.tops || ''),
    serie: h('select', null, [['', 'einmalig'], ['woechentlich', 'jede Woche'], ['14tage', 'alle zwei Wochen'], ['monatlich', 'jeden Monat am selben Tag'], ['monatlich_wochentag', 'jeden Monat am selben Wochentag (z. B. erster Dienstag)']].map(([v, l]) => h('option', { value: v }, l))),
    serieBis: h('input', { type: 'date', value: isoInTagen(365) }),
  };
  // Neuer Termin: der Titel folgt der Art (z. B. »Versammlung« → »Mitgliederversammlung«), solange er nicht von Hand geändert wurde.
  const TITEL_VORSCHLAG = { Arbeitseinsatz: 'Arbeitseinsatz', Versammlung: 'Mitgliederversammlung', Ablesung: 'Zählerablesung', Fest: 'Sommerfest', Vorstand: 'Vorstandssitzung', Sonstiges: '' };
  if (isNew) {
    let vorschlag = f.titel.value ? null : TITEL_VORSCHLAG[f.art.value];
    if (vorschlag) f.titel.value = vorschlag;
    f.art.addEventListener('change', () => {
      if (!f.titel.value || f.titel.value === vorschlag) { vorschlag = TITEL_VORSCHLAG[f.art.value] || ''; f.titel.value = vorschlag; }
    });
  }
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const body = h('div', { class: 'form' },
    fld('Titel *', f.titel, null, true), fld('Datum *', f.datum), fld('Art', f.art),
    fld('Beginn', f.von, 'leer = ganztägig'), fld('Ende', f.bis), fld('Ort', f.ort),
    fld('Hinweis (für alle sichtbar – keine Namen oder persönlichen Angaben)', f.notiz, null, true),
    fld('Tagesordnung für die Einladung (optional)', f.tops, 'Leer lassen: bei Versammlungen werden die üblichen Punkte eingesetzt. Für alle sichtbar – keine Namen.', true),
    isNew ? fld('Wiederholung', f.serie, 'Legt einzelne Termine an (höchstens 60), die du danach einzeln ändern oder löschen kannst.') : null,
    isNew ? fld('Serie bis', f.serieBis) : null);
  return modal(isNew ? 'Termin eintragen' : 'Termin bearbeiten', body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const data = { datum: f.datum.value, von: f.von.value, bis: f.bis.value, titel: f.titel.value, art: f.art.value, ort: f.ort.value, notiz: f.notiz.value.trim(), tops: f.tops.value };
      merkeSchreiben('termin-art', f.art.value);
      merkeSchreiben('termin-ort', f.ort.value.trim());
      try {
        if (isNew && f.serie.value) {
          const r = await api('POST', '/api/admin/termine-serie', { ...data, rhythmus: f.serie.value, bis: f.serieBis.value });
          toast(`${r.angelegt} Termine angelegt`, 'ok');
          return true;
        }
        if (isNew) await api('POST', '/api/admin/termine', data);
        else await api('PUT', `/api/admin/termine/${cur.id}`, data);
        toast('Gespeichert', 'ok');
      } catch (e) { handleErr(e); return false; }
    } },
  ]);
}

// Jahresbericht für die Mitgliederversammlung: Kennzahlen, Kasse, Ausgaben, Verlauf, Termine – ohne Namen.
const jahresberichtDrucken = (jahr) => pdfDrucken({
  url: `/api/admin/jahresbericht?year=${jahr}`, oeffnenUrl: `/api/admin/jahresbericht/oeffnen?year=${jahr}`,
  titel: `Jahresbericht ${jahr}`, meldung: () => 'Der Jahresbericht wurde im PDF-Programm geöffnet – dort drucken.',
});

const jahresplanDrucken = (jahr) => pdfDrucken({
  url: `/api/termine-druck?jahr=${jahr}`, oeffnenUrl: `/api/termine-druck/oeffnen?jahr=${jahr}`,
  titel: `Vereinstermine ${jahr}`, meldung: () => 'Der Jahresplan wurde im PDF-Programm geöffnet – dort drucken oder aushängen.',
});

const einladungDrucken = (t) => pdfDrucken({
  url: `/api/admin/termine/${t.id}/einladung`, oeffnenUrl: `/api/admin/termine/${t.id}/einladung/oeffnen`,
  titel: `Einladung ${t.titel}`, meldung: () => 'Die Einladung wurde im PDF-Programm geöffnet – dort drucken oder per E-Mail verschicken.',
});
// Protokoll einer Versammlung: Tagesordnung aus der Einladung, je Punkt Text und optional eine Abstimmung,
// die beim Speichern ins Beschlussbuch übernommen wird.
// Textbausteine für Protokolle: fertige Tagesordnungspunkte und Beschlussformeln, per Klick eingefügt und danach angepasst.
const PROTOKOLL_BAUSTEINE = {
  Vorstand: [
    { titel: 'Begrüßung und Beschlussfähigkeit', text: 'Die Sitzungsleitung begrüßt die Anwesenden und stellt die Beschlussfähigkeit fest.' },
    { titel: 'Genehmigung des letzten Protokolls', text: 'Das Protokoll der letzten Sitzung wurde vorgelesen und genehmigt.', beschluss: 'Das Protokoll der letzten Sitzung wird genehmigt.' },
    { titel: 'Bericht der Kassenführung', text: 'Kassenstand, Einnahmen und Ausgaben seit der letzten Sitzung.' },
    { titel: 'Zustand der Anlage und Mängel', text: 'Ergebnisse der Begehung, offene Mängel und Fristen.' },
    { titel: 'Pächterwechsel und Warteliste', text: 'Kündigungen, freie Gärten, Bewerber auf der Warteliste.' },
    { titel: 'Anträge und Anfragen', text: '' },
    { titel: 'Termine und Verschiedenes', text: 'Nächste Sitzung, Arbeitseinsätze, Sonstiges.' },
  ],
  Mitgliederversammlung: [
    { titel: 'Begrüßung und Beschlussfähigkeit', text: 'Die Versammlungsleitung begrüßt die Mitglieder, stellt die ordnungsgemäße Einladung und die Beschlussfähigkeit fest.' },
    { titel: 'Genehmigung der Tagesordnung', text: '', beschluss: 'Die Tagesordnung wird in der vorliegenden Fassung genehmigt.' },
    { titel: 'Bericht des Vorstands', text: 'Rückblick auf das Vereinsjahr.' },
    { titel: 'Kassenbericht', text: 'Einnahmen, Ausgaben und Kassenbestand zum Jahresende.' },
    { titel: 'Bericht der Kassenprüfung', text: 'Die Kassenprüfung hat die Kasse geprüft und berichtet.' },
    { titel: 'Entlastung des Vorstands', text: '', beschluss: 'Dem Vorstand wird für das abgelaufene Geschäftsjahr Entlastung erteilt.' },
    { titel: 'Haushaltsplan', text: 'Vorstellung des Haushaltsplans für das neue Jahr.', beschluss: 'Der Haushaltsplan wird wie vorgelegt beschlossen.' },
    { titel: 'Wahlen', text: '' },
    { titel: 'Anträge', text: '' },
    { titel: 'Verschiedenes', text: '' },
  ],
  Beschlussformeln: [
    { titel: 'Preise festsetzen', text: '', beschluss: 'Der Preis beträgt ab dem neuen Abrechnungsjahr … € je …' },
    { titel: 'Arbeitsstunden', text: '', beschluss: 'Je Garten sind … Arbeitsstunden im Jahr zu leisten. Für jede fehlende Stunde werden … € berechnet.' },
    { titel: 'Anschaffung', text: '', beschluss: 'Die Anschaffung von … zu einem Preis von höchstens … € wird beschlossen.' },
    { titel: 'Beitragsanpassung', text: '', beschluss: 'Der Vereinsbeitrag wird ab dem nächsten Jahr auf … € festgesetzt.' },
    { titel: 'Vertrag', text: '', beschluss: 'Der Vorstand wird ermächtigt, den Vertrag mit … abzuschließen.' },
  ],
};

async function sitzungDialog(t) {
  let d;
  try { d = await api('GET', `/api/admin/termine/${t.id}/protokoll`); } catch (e) { handleErr(e); return; }
  const p = d.protokoll;
  const zahl = (v) => h('input', { type: 'number', min: 0, value: v || '', style: 'width:6em' });
  const f = {
    ort: h('input', { type: 'text', maxlength: 120, value: p.ort || '' }),
    beginn: h('input', { type: 'time', value: p.beginn || '' }), ende: h('input', { type: 'time', value: p.ende || '' }),
    leitung: h('input', { type: 'text', maxlength: 100, value: p.leitung || '' }), prot: h('input', { type: 'text', maxlength: 100, value: p.protokollfuehrer || '' }),
    anwesend: zahl(p.anwesend), stimm: zahl(p.stimmberechtigt || (d.neu && p.gremium !== 'Vorstand' ? d.mitglieder : '')),
    faehig: h('input', { type: 'checkbox', checked: p.beschlussfaehig }),
  };
  const punkte = p.punkte.map((x) => ({ ...x }));
  const liste = h('div', { class: 'sitzung-punkte' });
  const bausteinEinfuegen = (b) => {
    punkte.push({ titel: b.titel, text: b.text || '', ...(b.beschluss ? { beschluss: { titel: b.beschluss, ja: 0, nein: 0, enthaltung: 0, ergebnis: 'angenommen' } } : {}) });
    zeichne();
  };
  const baustein = h('select', { 'aria-label': 'Textbaustein einfügen', onchange: (e) => {
    const [gruppe, i] = e.target.value.split(':');
    if (gruppe) bausteinEinfuegen(PROTOKOLL_BAUSTEINE[gruppe][Number(i)]);
    e.target.value = '';
  } }, h('option', { value: '' }, 'Textbaustein einfügen …'),
  ...Object.entries(PROTOKOLL_BAUSTEINE).map(([g, l]) => h('optgroup', { label: g === 'Beschlussformeln' ? 'Beschlussformeln' : g === 'Vorstand' ? 'Vorstandssitzung' : 'Mitgliederversammlung' },
    l.map((b, i) => h('option', { value: `${g}:${i}` }, b.titel)))));
  const zeichne = () => {
    liste.replaceChildren(...punkte.map((x, i) => {
      const titel = h('input', { type: 'text', maxlength: 200, value: x.titel, oninput: (e) => { x.titel = e.target.value; } });
      const text = h('textarea', { rows: 3, maxlength: 10000, class: 'rs-eingabe', placeholder: 'Was wurde berichtet, besprochen, vereinbart?', oninput: (e) => { x.text = e.target.value; } });
      text.value = x.text || '';
      const b = x.beschluss;
      let abst = null;
      if (b) {
        const n = (k) => h('input', { type: 'number', min: 0, value: b[k] || '', style: 'width:5em', oninput: (e) => { b[k] = Number(e.target.value || 0); } });
        abst = h('div', { class: 'sitzung-beschluss' },
          h('div', { class: 'field wide' }, h('label', null, 'Beschluss' + (x.beschlussId && d.nummern[x.beschlussId] ? ` ${d.nummern[x.beschlussId]}` : '')),
            h('input', { type: 'text', maxlength: 200, value: b.titel, placeholder: 'z. B. Der Wasserpreis beträgt ab 2027 2,10 € je m³.', oninput: (e) => { b.titel = e.target.value; } })),
          h('div', { class: 'toolbar' }, h('label', null, 'Ja ', n('ja')), h('label', null, 'Nein ', n('nein')), h('label', null, 'Enthaltung ', n('enthaltung')),
            h('select', { onchange: (e) => { b.ergebnis = e.target.value; } }, ['angenommen', 'abgelehnt', 'zur Kenntnis'].map((v) => h('option', { value: v, selected: b.ergebnis === v }, v))),
            h('div', { class: 'spacer' }), h('button', { class: 'btn small', onclick: () => { x.beschluss = null; zeichne(); } }, 'Abstimmung entfernen')));
      }
      return h('div', { class: 'card sitzung-punkt' },
        h('div', { class: 'toolbar' }, h('b', null, `TOP ${i + 1}`), titel,
          h('button', { class: 'btn small', disabled: i === 0, title: 'nach oben', onclick: () => { [punkte[i - 1], punkte[i]] = [punkte[i], punkte[i - 1]]; zeichne(); } }, '↑'),
          h('button', { class: 'btn small', title: 'Punkt entfernen', onclick: () => { punkte.splice(i, 1); zeichne(); } }, '✕')),
        text, abst || h('button', { class: 'btn small', onclick: () => { x.beschluss = { titel: '', ja: 0, nein: 0, enthaltung: 0, ergebnis: 'angenommen' }; zeichne(); } }, '+ Abstimmung'));
    }), h('div', { class: 'toolbar' },
      h('button', { class: 'btn', onclick: () => { punkte.push({ titel: '', text: '' }); zeichne(); } }, '+ Tagesordnungspunkt'),
      !punkte.length ? h('button', { class: 'btn primary', onclick: () => { for (const b of PROTOKOLL_BAUSTEINE[p.gremium === 'Vorstand' ? 'Vorstand' : 'Mitgliederversammlung']) bausteinEinfuegen(b); } }, 'Standard-Tagesordnung einfügen') : null,
      baustein));
  };
  zeichne();
  const speichern = async () => {
    try {
      d = await api('PUT', `/api/admin/termine/${t.id}/protokoll`, { gremium: p.gremium, ort: f.ort.value, beginn: f.beginn.value, ende: f.ende.value, leitung: f.leitung.value,
        protokollfuehrer: f.prot.value, anwesend: Number(f.anwesend.value || 0), stimmberechtigt: Number(f.stimm.value || 0), beschlussfaehig: f.faehig.checked, punkte });
      S.verw = null;
      return true;
    } catch (e) { handleErr(e); return false; }
  };
  const fld = (label, el) => h('div', { class: 'field' }, h('label', null, label), el);
  const ok = await breiterDialog(`Protokoll: ${t.titel} am ${deDate(t.datum)}`, h('div', null,
    h('div', { class: 'form' }, fld('Ort', f.ort), fld('Beginn', f.beginn), fld('Ende', f.ende), fld(p.gremium === 'Vorstand' ? 'Sitzungsleitung' : 'Versammlungsleitung', f.leitung),
      fld('Protokoll führt', f.prot), fld('Anwesend', f.anwesend), fld('davon/von stimmberechtigt', f.stimm), h('div', { class: 'field' }, h('label', null, ' '), h('label', null, f.faehig, ' beschlussfähig'))),
    h('p', { class: 'hint' }, 'Abstimmungen werden beim Speichern ins Beschlussbuch übernommen und dort mit diesem Protokoll aktuell gehalten.'),
    liste),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', value: 'speichern', action: speichern },
    { label: 'Speichern und drucken', cls: 'primary', value: 'drucken', action: speichern }]);
  if (ok === 'drucken') pdfDrucken({ url: `/api/admin/termine/${t.id}/protokoll/pdf`, oeffnenUrl: `/api/admin/termine/${t.id}/protokoll/pdf/oeffnen`, titel: 'Protokoll', meldung: () => 'Das Protokoll wurde im PDF-Programm geöffnet.' });
  if (ok) toast('Protokoll gespeichert', 'ok');
}

const anwesenheitDrucken = (t) => pdfDrucken({
  url: `/api/admin/termine/${t.id}/anwesenheit`, oeffnenUrl: `/api/admin/termine/${t.id}/anwesenheit/oeffnen`,
  titel: `Anwesenheitsliste ${t.titel}`, meldung: () => 'Die Anwesenheitsliste wurde im PDF-Programm geöffnet – dort drucken.',
});

const stundenlisteDrucken = (t) => pdfDrucken({
  url: `/api/admin/termine/${t.id}/stundenliste`, oeffnenUrl: `/api/admin/termine/${t.id}/stundenliste/oeffnen`,
  titel: `Stundenliste ${t.titel}`, meldung: () => 'Die Stundenliste wurde im PDF-Programm geöffnet – dort drucken und zum Einsatz mitnehmen.',
});

// zeitspanneStunden versteht »9-11:30«, »09:00–12:00« oder »9 bis 12« und liefert die Stunden (sonst null).
function zeitspanneStunden(text) {
  const m = String(text).trim().match(/^(\d{1,2})(?:[:.](\d{2}))?\s*(?:-|–|bis)\s*(\d{1,2})(?:[:.](\d{2}))?$/i);
  if (!m) return null;
  const von = Number(m[1]) * 60 + Number(m[2] || 0), bis = Number(m[3]) * 60 + Number(m[4] || 0);
  if (Number(m[1]) > 23 || Number(m[3]) > 24 || Number(m[2] || 0) > 59 || Number(m[4] || 0) > 59 || bis <= von) return null;
  return Math.round((bis - von) / 60 * 100) / 100;
}
// Stunden: Zahl (»2,5«) oder Zeitspanne (»9-11:30«)
const stundenWert = (text) => { const z = zeitspanneStunden(text); return z != null ? z : parseNum(text); };

// Stunden eines Arbeitseinsatzes abhaken: wer da war, bekommt die Dauer des Termins vorgeschlagen.
async function stundenDialog(t) {
  let v;
  try { v = await api('GET', `/api/admin/termine/${t.id}/stunden`); } catch (e) { handleErr(e); return false; }
  const zeilen = v.zeilen.map((z) => {
    const std = h('input', { type: 'text', inputmode: 'decimal', class: 'std-in', value: z.stunden != null ? nfFlex.format(z.stunden) : '', placeholder: '–', 'aria-label': `Stunden ${z.name}`, title: 'Stunden (z. B. 2,5) oder Zeit von–bis (z. B. 9-11:30)' });
    const dabei = h('input', { type: 'checkbox', checked: z.stunden != null, 'aria-label': `${z.name} war dabei` });
    dabei.addEventListener('change', () => {
      if (dabei.checked && !std.value) std.value = v.dauer ? nfFlex.format(v.dauer) : '';
      if (!dabei.checked) std.value = '';
      summe();
    });
    std.addEventListener('input', () => { dabei.checked = std.value.trim() !== ''; summe(); });
    // »9-11:30« beim Verlassen des Feldes in Stunden umrechnen, damit man sieht, was gebucht wird
    std.addEventListener('blur', () => { const z = zeitspanneStunden(std.value); if (z != null) { std.value = nfFlex.format(z); summe(); } });
    const bisher = (z.gesamt || 0) - (z.stunden || 0);
    return { z, std, dabei, tr: h('tr', null, h('td', null, dabei), h('td', null, z.gartennr), h('td', null, z.name), h('td', null, std),
      h('td', { class: 'num klein' }, `${nfFlex.format(bisher)} h`, v.pflicht && bisher >= v.pflicht ? h('span', { class: 'badge ok', style: 'margin-left:6px' }, 'erfüllt') : null)) };
  });
  const summeEl = h('b', null);
  const summe = () => {
    let n = 0, s = 0;
    for (const x of zeilen) { const w = stundenWert(x.std.value); if (x.dabei.checked && w > 0) { n++; s += w; } }
    summeEl.textContent = `${n} Teilnehmer · ${nfFlex.format(Math.round(s * 100) / 100)} Stunden`;
  };
  summe();
  const jahrSel = h('select', null, v.jahre.map((j) => h('option', { value: j, selected: j === v.jahr }, String(j))));
  const alle = () => { for (const x of zeilen) { if (!x.dabei.checked) { x.dabei.checked = true; if (!x.std.value) x.std.value = v.dauer ? nfFlex.format(v.dauer) : ''; } } summe(); };
  const body = h('div', null,
    h('p', { class: 'hint' }, `${deDate(t.datum)} · ${terminZeit(t)}. Hake ab, wer dabei war – die Dauer des Termins${v.dauer ? ` (${nfFlex.format(v.dauer)} h)` : ''} wird vorgeschlagen. Wer früher gegangen oder später gekommen ist, bekommt seine eigenen Stunden: als Zahl (2,5) oder als Uhrzeit von–bis (9-11:30). Die Stunden werden auf die Gemeinschaftsstunden gebucht; spätere Korrekturen zählen nicht doppelt.`),
    h('div', { class: 'toolbar' }, h('label', null, 'Buchen auf Abrechnungsjahr'), jahrSel, h('button', { class: 'btn small', onclick: alle }, 'Alle waren dabei'), h('div', { class: 'spacer' }), summeEl),
    h('div', { class: 'tablewrap', style: 'max-height:380px;overflow:auto' }, h('table', { class: 'std-tab' },
      h('thead', null, h('tr', null, ['Dabei', 'Garten', 'Name', 'Stunden', 'Bisher im Jahr'].map((x) => h('th', null, x)))),
      h('tbody', null, zeilen.map((x) => x.tr)))));
  return breiterDialog(`Stunden: ${t.titel}`, body, [
    { label: 'Abbrechen', value: false },
    { label: 'Stunden buchen', cls: 'primary', value: true, action: async () => {
      const eintraege = [];
      for (const x of zeilen) {
        if (!x.dabei.checked) continue;
        const w = stundenWert(x.std.value);
        if (w == null || Number.isNaN(w) || w < 0 || w > 24) { toast(`Bitte für ${x.z.name} Stunden (z. B. 2,5) oder die Zeit (z. B. 9-11:30) eintragen.`, 'err'); x.std.focus(); return false; }
        eintraege.push({ paechterId: x.z.paechterId, stunden: w });
      }
      try {
        const r = await api('PUT', `/api/admin/termine/${t.id}/stunden`, { jahr: Number(jahrSel.value), eintraege });
        toast(`${r.teilnehmer} Teilnehmer, ${nfFlex.format(r.stunden)} Stunden gebucht`, 'ok');
      } catch (e) { handleErr(e); return false; }
    } }]);
}

function viewTermine() {
  const st = S.state;
  const istAdmin = !st.hasPassword || st.loggedIn;
  const heute = todayIso();
  const jahr = S.termineJahr || new Date().getFullYear();
  const jahre = new Set([new Date().getFullYear(), new Date().getFullYear() + 1, ...st.termine.map((t) => Number(t.datum.slice(0, 4)))]);
  const imJahr = st.termine.filter((t) => t.datum.startsWith(jahr + '-'));
  const vorjahr = st.termine.filter((t) => t.datum.startsWith((jahr - 1) + '-')).length;
  const jahrSel = h('select', { onchange: (e) => { S.termineJahr = Number(e.target.value); render(); } },
    [...jahre].sort().map((j) => h('option', { value: j, selected: j === jahr }, String(j))));
  const neu = async (vorlage) => { if (await terminDialog(vorlage)) await reload(); };
  const uebernehmen = async () => {
    try {
      const r = await api('POST', '/api/admin/termine-uebernehmen', { von: jahr - 1, nach: jahr });
      toast(r.uebernommen ? `${anz(r.uebernommen, 'Termin', 'Termine')} übernommen – bitte die Daten kurz prüfen.` : 'Alle Termine des Vorjahres sind schon eingetragen.', 'ok');
      await reload();
    } catch (e) { handleErr(e); }
  };
  const loeschen = async (t) => {
    try { const r = await api('DELETE', `/api/admin/termine/${t.id}`); await reload(); geloeschtToast(r, 'Termin', reload); } catch (e) { handleErr(e); }
  };
  const gruppen = [];
  for (const t of imJahr) {
    const m = Number(t.datum.slice(5, 7)) - 1;
    if (!gruppen.length || gruppen[gruppen.length - 1].m !== m) gruppen.push({ m, termine: [] });
    gruppen[gruppen.length - 1].termine.push(t);
  }
  const naechster = imJahr.find((t) => t.datum >= heute);
  return h('div', null,
    h('h2', null, `Vereinstermine ${jahr}`, ' ', seitenHilfe('termine')),
    h('p', { class: 'hint' }, 'Der Jahresplaner des Vereins: Arbeitseinsätze, Versammlungen, Ablesung, Feste. Als Aushang drucken oder als Kalenderdatei ins Handy übernehmen.'),
    h('div', { class: 'toolbar' },
      h('label', null, 'Jahr'), jahrSel,
      istAdmin ? h('button', { class: 'btn primary', onclick: () => neu({ datum: jahr === new Date().getFullYear() ? heute : `${jahr}-01-01` }) }, '+ Termin eintragen') : null,
      istAdmin && vorjahr ? h('button', { class: 'btn', title: `Alle Termine aus ${jahr - 1} auf denselben Wochentag in ${jahr} übertragen`, onclick: uebernehmen }, `Termine aus ${jahr - 1} übernehmen`) : null,
      istAdmin && imJahr.filter((t) => t.datum < heute).length > 1 ? h('button', { class: 'btn', title: 'Alle vergangenen Termine dieses Jahres in den Papierkorb legen (Rückgängig möglich)', onclick: async () => {
        const alt = imJahr.filter((t) => t.datum < heute);
        if (!(await confirmBox(`${alt.length} vergangene Termine aus ${jahr} in den Papierkorb legen?`, 'In den Papierkorb', true))) return;
        await sammelLoeschen(alt.map((t) => `/api/admin/termine/${t.id}`), anz(alt.length, 'Termin', 'Termine'), reload);
      } }, 'Vergangene aufräumen') : null,
      h('div', { class: 'spacer' }),
      h('button', { class: 'btn', disabled: !imJahr.length, onclick: () => jahresplanDrucken(jahr) }, 'Als Aushang drucken'),
      h('a', { class: 'btn', href: '/api/termine.ics', download: 'Vereinstermine.ics', title: 'Kalenderdatei für Outlook, Google-Kalender oder das Handy' }, 'In den Kalender')),
    imJahr.length ? h('div', { class: 'termine' }, gruppen.map((g) => h('div', { class: 'card termin-monat' },
      h('h3', null, MONATE[g.m]),
      h('ul', { class: 'termin-liste' }, g.termine.map((t) => {
        const d = isoDatum(t.datum);
        const vorbei = t.datum < heute;
        return h('li', { class: 'termin' + (vorbei ? ' vorbei' : '') + (t === naechster ? ' naechster' : '') },
          h('div', { class: 'termin-tag' }, h('b', null, String(d.getDate())), h('span', null, WOCHENTAGE[d.getDay()])),
          h('div', { class: 'termin-text' },
            h('b', null, t.titel), t === naechster ? h('span', { class: 'badge ok' }, 'als Nächstes') : null,
            h('div', { class: 'hint' }, [terminZeit(t), t.ort].filter(Boolean).join(' · ')),
            t.notiz ? h('div', { class: 'hint' }, t.notiz) : null),
          h('span', { class: 'chip termin-art' }, t.art),
          istAdmin ? terminAktionen(t, vorbei, neu, loeschen) : null);
      })))))
      : h('div', { class: 'card' }, leer('kalender', `Noch keine Termine für ${jahr}`,
        vorjahr && istAdmin ? `Aus ${jahr - 1} sind ${vorjahr} Termine eingetragen. Mit einem Klick übernimmst du sie auf dieselben Wochentage.` : 'Trag Arbeitseinsätze, Versammlungen und Feste ein. Daraus wird ein Aushang zum Ausdrucken und eine Kalenderdatei.',
        istAdmin ? (vorjahr ? `Termine aus ${jahr - 1} übernehmen` : '+ Termin eintragen') : null,
        vorjahr ? uebernehmen : () => neu({ datum: `${jahr}-01-01` }))));
}

// Knöpfe je Termin: die eine passende Hauptaktion sichtbar (vor dem Termin vorbereiten, danach nachbereiten),
// »Bearbeiten« und alles Weitere im Mehr-Menü – damit nicht sieben Knöpfe nebeneinanderstehen.
function terminAktionen(t, vorbei, neu, loeschen) {
  const sitzung = t.art === 'Versammlung' || t.art === 'Vorstand';
  const einsatz = t.art === 'Arbeitseinsatz';
  const a = {
    einladung: { label: 'Einladung drucken', titel: 'Einladung mit Tagesordnung als PDF', aktion: () => einladungDrucken(t) },
    anwesenheit: { label: 'Anwesenheitsliste drucken', aktion: () => anwesenheitDrucken(t) },
    paket: { label: 'Versammlungspaket (ZIP)', aktion: () => { location.href = `/api/admin/termine/${encodeURIComponent(t.id)}/paket`; } },
    protokoll: { label: 'Protokoll', titel: 'Protokoll schreiben – Beschlüsse landen im Beschlussbuch', aktion: () => sitzungDialog(t) },
    stundenliste: { label: 'Stundenliste drucken', titel: 'Liste zum Eintragen von Beginn und Ende', aktion: () => stundenlisteDrucken(t) },
    stunden: { label: 'Stunden eintragen', titel: 'Abhaken, wer dabei war, und die Stunden buchen', aktion: async () => { if (await stundenDialog(t)) await reload(); } },
  };
  let haupt = null; let weitere = [];
  if (sitzung) {
    haupt = vorbei ? a.protokoll : a.einladung;
    weitere = [vorbei ? a.einladung : a.protokoll, a.anwesenheit, t.art === 'Versammlung' ? a.paket : null];
  } else if (einsatz) {
    haupt = vorbei ? a.stunden : a.stundenliste;
    weitere = [vorbei ? a.stundenliste : a.stunden];
  }
  return h('div', { class: 'termin-aktionen' },
    haupt ? h('button', { class: 'btn small primary', title: haupt.titel || '', onclick: haupt.aktion }, haupt.label) : null,
    h('button', { class: 'btn small', onclick: () => neu(t) }, 'Bearbeiten'),
    mehrMenue([...weitere.filter(Boolean).map((x) => ({ label: x.label, aktion: x.aktion })), { label: 'Löschen', gefahr: true, aktion: () => loeschen(t) }]));
}

// Übersicht: die nächsten Termine der kommenden acht Wochen
function naechsteTermine(st) {
  const heute = todayIso();
  const bis = new Date(); bis.setDate(bis.getDate() + 56);
  const p = (n) => String(n).padStart(2, '0');
  const grenze = `${bis.getFullYear()}-${p(bis.getMonth() + 1)}-${p(bis.getDate())}`;
  const l = (st.termine || []).filter((t) => t.datum >= heute && t.datum <= grenze).slice(0, 3);
  if (!l.length) return null;
  return h('div', { class: 'card naechste-termine' },
    h('div', { class: 'fs-kopf' }, h('b', null, 'Nächste Termine'),
      h('button', { class: 'btn small', onclick: () => { S.tab = 'termine'; render(); } }, 'Alle Termine →')),
    h('ul', { class: 'termin-liste' }, l.map((t) => {
      const d = isoDatum(t.datum);
      return h('li', { class: 'termin' },
        h('div', { class: 'termin-tag' }, h('b', null, String(d.getDate())), h('span', null, `${WOCHENTAGE[d.getDay()]} · ${MONATE[d.getMonth()].slice(0, 3)}`)),
        h('div', { class: 'termin-text' }, h('b', null, t.titel), h('div', { class: 'hint' }, [terminZeit(t), t.ort].filter(Boolean).join(' · '))),
        h('span', { class: 'chip termin-art' }, t.art));
    })));
}

// ------------------------------------------------------------------ Gartenakte

const DOKUMENT_ARTEN = ['Bauantrag', 'Genehmigung', 'Foto', 'Pachtvertrag', 'Schriftverkehr', 'Protokoll', 'Sonstiges'];
const PERSONEN_ARTEN = new Set(['Pachtvertrag', 'Schriftverkehr']);
const groesseText = (n) => (n >= 1 << 20 ? `${nfFlex.format(Math.round(n / 104857.6) / 10)} MB` : `${Math.max(1, Math.round(n / 1024))} KB`);

// breiter Dialog (für Akte und Wertermittlung)
function breiterDialog(titel, body, knoepfe) {
  const p = modal(titel, body, knoepfe);
  const dlg = document.querySelector('dialog:last-of-type');
  if (dlg) dlg.classList.add('breit');
  return p;
}

async function gartenakteDialog(nr) {
  let ak;
  try { ak = await api('GET', `/api/admin/gartenakte?gartennr=${encodeURIComponent(nr)}`); } catch (e) { handleErr(e); return; }
  const inhalt = h('div', { class: 'akte' });
  const neuLaden = async () => {
    try { ak = await api('GET', `/api/admin/gartenakte?gartennr=${encodeURIComponent(nr)}`); } catch (e) { handleErr(e); return; }
    zeichnen();
  };
  const abschnitt = (ic, titel, knopf, ...kids) => h('section', { class: 'akte-abschnitt' },
    h('div', { class: 'akte-kopf' }, h('span', { class: 'akte-ico' }, icon(ic, 18)), h('b', null, titel), h('div', { class: 'spacer' }), knopf), ...kids);
  const zeichnen = () => {
    const p = ak.paechter;
    const docs = ak.dokumente.map((d) => {
      const bild = /\.(jpe?g|png|webp)$/i.test(d.datei);
      const url = `/api/admin/dokumente/${d.id}`;
      return h('div', { class: 'dok' },
        bild ? h('img', { class: 'dok-bild', src: url, alt: '', loading: 'lazy' }) : h('div', { class: 'dok-bild pdf' }, 'PDF'),
        h('div', { class: 'dok-text' }, h('b', { title: d.titel }, d.titel),
          h('span', { class: 'hint' }, `${d.art} · ${deDate(d.datum)} · ${groesseText(d.groesse)}`),
          d.paechterId ? h('span', { class: 'badge neu', title: 'Gehört zum Pächter und wird mit ihm endgültig gelöscht' }, 'zur Person') : null),
        h('div', { class: 'dok-aktionen' },
          h('a', { class: 'btn small', href: url, target: '_blank', rel: 'noopener' }, 'Öffnen'),
          h('button', { class: 'btn small danger', onclick: async () => {
            if (!(await confirmBox(`Dokument „${d.titel}“ löschen?`, 'Löschen', true))) return;
            try { await api('DELETE', url); await neuLaden(); } catch (e) { handleErr(e); }
          } }, 'Löschen')));
    });
    inhalt.replaceChildren(
      h('div', { class: 'akte-steckbrief' },
        h('div', { class: 'akte-nr' }, h('span', null, 'Garten'), h('b', null, nr)),
        h('div', null,
          p ? h('div', null, h('b', null, p.name), h('span', { class: 'hint' }, ` · Mitgl.-Nr. ${p.mitgliedsnr} · ${nfFlex.format(p.gartengroesse)} m²`)) : h('div', null, h('b', null, 'Zurzeit frei'), h('span', { class: 'hint' }, ' · kein Pächter')),
          p ? h('div', { class: 'hint' }, [p.strasse, p.plzOrt, p.telefon, p.email].filter(Boolean).join(' · ')) : null,
          h('div', { class: 'hint' }, `Zähler: Wasser ${p && p.wasserzaehlerNr || '–'}, Strom ${p && p.stromzaehlerNr || '–'}`))),
      abschnitt('akte', `Dokumente (${ak.dokumente.length})`, h('button', { class: 'btn small primary', onclick: () => dokumentHochladen(nr, !!p).then((ok) => ok && neuLaden()) }, '+ Dokument'),
        docs.length ? h('div', { class: 'dok-liste' }, docs) : h('p', { class: 'hint' }, 'Noch keine Dokumente. Lege Bauantrag, Genehmigungen, Fotos der Laube oder den Pachtvertrag hier ab – alles bleibt nur auf diesem Rechner.')),
      abschnitt('waage', `Wertermittlungen (${ak.wertermittlungen.length})`, h('button', { class: 'btn small primary', onclick: () => wertermittlungDialog(nr, null).then((ok) => ok && neuLaden()) }, '+ Wertermittlung'),
        ak.wertermittlungen.length ? h('table', { class: 'akte-tab' }, h('tbody', null, ak.wertermittlungen.map((x) => h('tr', null,
          h('td', null, deDate(x.datum)), h('td', null, x.anlass), h('td', { class: 'hint' }, x.wertermittler || ''), h('td', { class: 'num' }, h('b', null, eur(x.summe))),
          h('td', { class: 'r' },
            h('button', { class: 'btn small', onclick: () => wertermittlungDialog(nr, x).then((ok) => ok && neuLaden()) }, 'Bearbeiten'), ' ',
            h('button', { class: 'btn small', onclick: () => wertermittlungDrucken(x) }, 'Drucken'), ' ',
            h('button', { class: 'btn small danger', onclick: async () => {
              if (!(await confirmBox(`Wertermittlung vom ${deDate(x.datum)} löschen?`, 'Löschen', true))) return;
              try { await api('DELETE', `/api/admin/wertermittlungen/${x.id}`); await neuLaden(); } catch (e) { handleErr(e); }
            } }, 'Löschen'))))))
          : h('p', { class: 'hint' }, 'Vor einem Pächterwechsel den Wert von Laube, Anpflanzungen und Einrichtungen festhalten. Die Summe wird beim Wechsel als Ablösesumme vorgeschlagen.')),
      abschnitt('personen', 'Übergaben und Pächter', null,
        ak.uebergaben.length ? h('table', { class: 'akte-tab' }, h('tbody', null, ak.uebergaben.map((u) => h('tr', null,
          h('td', null, deDate(u.datum)), h('td', null, `${u.altName} → ${u.neuName}`), h('td', { class: 'num' }, u.abloese ? `Ablöse ${eur(u.abloese)}` : ''),
          h('td', { class: 'r' }, h('button', { class: 'btn small', onclick: () => protokollDrucken(u) }, 'Protokoll')))))) : null,
        ak.historie.length ? h('ul', { class: 'akte-verlauf' }, ak.historie.map((e) => h('li', null,
          h('b', null, e.name), ` (${e.mitgliedsnr || '–'}) · seit ${deDate(e.seit)}${e.bis ? ' bis ' + deDate(e.bis) : ' – heute'}`)))
          : h('p', { class: 'hint' }, 'Noch kein Eintrag im Gartenverlauf.')),
      abschnitt('uhr', 'Zeitleiste', null,
        ak.zeitleiste && ak.zeitleiste.length ? h('ul', { class: 'akte-verlauf' }, ak.zeitleiste.map((z) => h('li', null,
          h('b', null, deDate(z.datum)), ' · ', z.titel, z.text ? h('span', { class: 'hint' }, ` – ${z.text}`) : null)))
          : h('p', { class: 'hint' }, 'Noch keine Ereignisse.')));
  };
  zeichnen();
  await breiterDialog(`Gartenakte – Garten ${nr}`, inhalt, [
    { label: 'Exposé', value: false, action: () => { exposeDrucken(nr); return false; } },
    p_wechselKnopf(ak, nr),
    { label: 'Schließen', cls: 'primary', value: true }].filter(Boolean));
  await reload();
}

// Knopf »Pächterwechsel« in der Akte, wenn der Garten einen Pächter hat
const willkommenDrucken = (p) => pdfDrucken({
  url: `/api/admin/paechter/${p.id}/willkommen`, oeffnenUrl: `/api/admin/paechter/${p.id}/willkommen/oeffnen`,
  titel: `Willkommensmappe ${p.name}`, meldung: () => 'Die Willkommensmappe wurde im PDF-Programm geöffnet – dort drucken oder per E-Mail schicken.',
});
const exposeDrucken = (nr) => pdfDrucken({
  url: `/api/admin/expose?gartennr=${encodeURIComponent(nr)}`, oeffnenUrl: `/api/admin/expose/oeffnen?gartennr=${encodeURIComponent(nr)}`,
  titel: `Garten ${nr}`, meldung: () => 'Das Exposé wurde im PDF-Programm geöffnet – für Interessenten drucken oder verschicken.',
});

// Texte für Willkommensmappe und Exposé: Ansprechpartner und Hinweise
async function vereinsinfoDialog() {
  let v;
  try { v = await api('GET', '/api/admin/vereinsinfo'); } catch (e) { handleErr(e); return; }
  const ap = h('textarea', { rows: 4, style: 'width:100%', placeholder: 'z. B.\nVorsitz: Name, Telefon\nKasse: Name, E-Mail\nSprechstunde: samstags 10–12 Uhr' }, v.ansprechpartner || '');
  const hw = h('textarea', { rows: 6, style: 'width:100%', placeholder: 'Ein Hinweis pro Zeile, z. B.\nMittagsruhe 13–15 Uhr\nWasser wird im April angestellt' }, v.hinweise || '');
  await modal('Texte für Willkommensmappe und Exposé', h('div', { class: 'form' },
    h('div', { class: 'field wide' }, h('label', null, 'Ansprechpartner'), ap, h('span', { class: 'fhint' }, 'Erscheint in Willkommensmappe und Exposé – nur angeben, wer damit einverstanden ist.')),
    h('div', { class: 'field wide' }, h('label', null, 'Bitte beachten (Willkommensmappe)'), hw, h('span', { class: 'fhint' }, 'Kurz das Wichtigste aus der Gartenordnung, ein Punkt pro Zeile.'))),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    try { await api('PUT', '/api/admin/vereinsinfo', { ansprechpartner: ap.value, hinweise: hw.value }); toast('Gespeichert', 'ok'); } catch (e) { handleErr(e); return false; }
  } }]);
}

function p_wechselKnopf(ak, nr) {
  const p = ak.paechter && S.state.paechter.find((x) => x.id === ak.paechter.id && !x.ausgeschiedenAm);
  if (!p) return null;
  return { label: 'Pächterwechsel …', value: false, action: async () => { setTimeout(() => paechterwechselDialog(p), 0); return true; } };
}

async function dokumentHochladen(nr, hatPaechter) {
  const datei = h('input', { type: 'file', accept: '.pdf,.jpg,.jpeg,.png,.webp' });
  const titel = h('input', { type: 'text', maxlength: 120, placeholder: 'z. B. Baugenehmigung Laube' });
  const art = h('select', null, DOKUMENT_ARTEN.map((a) => h('option', { value: a }, a)));
  const datum = h('input', { type: 'date', value: todayIso() });
  const zurPerson = h('input', { type: 'checkbox' });
  const sync = () => { zurPerson.checked = hatPaechter && PERSONEN_ARTEN.has(art.value); };
  art.addEventListener('change', sync);
  datei.addEventListener('change', () => {
    const f = datei.files[0];
    if (f && !titel.value) titel.value = f.name.replace(/\.[^.]+$/, '');
    if (f && /\.(jpe?g|png|webp)$/i.test(f.name) && art.value === 'Bauantrag') { art.value = 'Foto'; sync(); }
  });
  sync();
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  return modal(`Dokument zu Garten ${nr}`, h('div', { class: 'form' },
    fld('Datei (PDF oder Foto, höchstens 12 MB) *', datei, null, true),
    fld('Bezeichnung', titel), fld('Art', art), fld('Datum des Dokuments', datum),
    hatPaechter ? h('div', { class: 'field wide' }, h('label', { class: 'check' }, zurPerson, ' Gehört zum jetzigen Pächter (z. B. Pachtvertrag, Schriftverkehr)'),
      h('span', { class: 'fhint' }, 'Solche Dokumente werden gelöscht, wenn der Pächter endgültig gelöscht wird. Fotos und Genehmigungen bleiben beim Garten.')) : null),
  [{ label: 'Abbrechen', value: false }, { label: 'Hochladen', cls: 'primary', value: true, action: async () => {
    if (!datei.files[0]) { toast('Bitte eine Datei auswählen.', 'err'); return false; }
    const fd = new FormData();
    fd.append('file', datei.files[0]); fd.append('gartennr', nr); fd.append('titel', titel.value); fd.append('art', art.value);
    fd.append('datum', datum.value); fd.append('zurPerson', zurPerson.checked ? '1' : '');
    try { await api('POST', '/api/admin/dokumente', fd, true); toast('Dokument gespeichert', 'ok'); } catch (e) { handleErr(e); return false; }
  } }]);
}

// ------------------------------------------------------------------ Wertermittlung

const WERT_VORLAGEN = [
  ['Baulichkeiten', 'Gartenlaube', 'm²', 0], ['Baulichkeiten', 'Gewächshaus', 'Stück', 0], ['Baulichkeiten', 'Geräteschuppen', 'Stück', 0],
  ['Baulichkeiten', 'Terrasse / Wege (befestigt)', 'm²', 0], ['Anpflanzungen', 'Obstbaum', 'Stück', 0], ['Anpflanzungen', 'Beerenstrauch', 'Stück', 0],
  ['Anpflanzungen', 'Hecke', 'm', 0], ['Anpflanzungen', 'Stauden und Beete', 'm²', 0], ['Einrichtungen', 'Wasserleitung im Garten', 'pauschal', 0],
  ['Einrichtungen', 'Elektroinstallation', 'pauschal', 0], ['Einrichtungen', 'Regentonne / Zisterne', 'Stück', 0], ['Einrichtungen', 'Komposter', 'Stück', 0],
  ['Abzug', 'Beseitigung von Mängeln', 'pauschal', 0], ['Abzug', 'Rückbau nicht zulässiger Bauten', 'pauschal', 0],
];
const WERT_KATEGORIEN = ['Baulichkeiten', 'Anpflanzungen', 'Einrichtungen', 'Abzug'];

async function wertermittlungDialog(nr, x) {
  const cur = x ? JSON.parse(JSON.stringify(x)) : { gartennr: nr, datum: todayIso(), anlass: 'Pächterwechsel', wertermittler: '', bemerkung: '', positionen: [] };
  const datum = h('input', { type: 'date', value: cur.datum });
  const anlass = h('select', null, ['Pächterwechsel', 'Kündigung', 'Gartenteilung', 'Sonstiges'].map((a) => h('option', { value: a, selected: a === cur.anlass }, a)));
  const wer = h('input', { type: 'text', value: cur.wertermittler || '', maxlength: 100, placeholder: 'Name der Wertermittlerin / des Wertermittlers' });
  const bem = h('textarea', { rows: 2, maxlength: 1500, style: 'width:100%' }, cur.bemerkung || '');
  const tbody = h('tbody');
  const summeEl = h('b', { class: 'wert-summe' });
  const zahl = (v) => { const el = h('input', { type: 'text', value: v ? numIn(v) : '', class: 'num-in' }); el.setAttribute('inputmode', 'decimal'); return el; };
  const zeilen = [];
  const rechnen = () => {
    let summe = 0;
    for (const z of zeilen) {
      const m = parseNum(z.menge.value) ?? 1, e = parseNum(z.wert.value) ?? 0, a = parseNum(z.abschlag.value) ?? 0;
      let w = Number.isNaN(m) || Number.isNaN(e) || Number.isNaN(a) ? 0 : Math.round(m * e * (1 - Math.min(100, a) / 100) * 100) / 100;
      if (z.kat.value === 'Abzug') w = -w;
      z.ergebnis.textContent = eur(w);
      z.tr.classList.toggle('abzug', z.kat.value === 'Abzug');
      summe += w;
    }
    summeEl.textContent = eur(Math.max(0, summe));
  };
  const zeile = (p) => {
    const z = {
      kat: h('select', null, WERT_KATEGORIEN.map((k) => h('option', { value: k, selected: k === p.kategorie }, k))),
      bez: h('input', { type: 'text', value: p.bezeichnung || '', maxlength: 120, placeholder: 'Bezeichnung' }),
      menge: zahl(p.menge || ''), einheit: h('input', { type: 'text', value: p.einheit || '', maxlength: 15, class: 'einheit-in', placeholder: 'm²' }),
      wert: zahl(p.einzelwert), abschlag: zahl(p.abschlag), ergebnis: h('span', { class: 'num' }),
    };
    z.tr = h('tr', null, h('td', null, z.kat), h('td', null, z.bez), h('td', null, z.menge), h('td', null, z.einheit), h('td', null, z.wert), h('td', null, z.abschlag),
      h('td', { class: 'num' }, z.ergebnis),
      h('td', null, h('button', { class: 'btn small', title: 'Zeile entfernen', onclick: () => { zeilen.splice(zeilen.indexOf(z), 1); z.tr.remove(); rechnen(); } }, '✕')));
    z.tr.addEventListener('input', rechnen); z.tr.addEventListener('change', rechnen);
    zeilen.push(z); tbody.append(z.tr); rechnen();
    return z;
  };
  for (const p of cur.positionen) zeile(p);
  if (!cur.positionen.length) zeile({ kategorie: 'Baulichkeiten', bezeichnung: 'Gartenlaube', einheit: 'm²' });
  const vorlagen = h('div', { class: 'chips' }, h('span', { class: 'hint' }, 'Schnell hinzufügen:'),
    WERT_VORLAGEN.map(([k, b, e]) => h('button', { class: 'chip chip-knopf', type: 'button', onclick: () => zeile({ kategorie: k, bezeichnung: b, einheit: e }).bez.focus() }, b)));
  const body = h('div', null,
    h('div', { class: 'form', style: 'margin-bottom:14px' },
      h('div', { class: 'field' }, h('label', null, 'Datum *'), datum), h('div', { class: 'field' }, h('label', null, 'Anlass'), anlass),
      h('div', { class: 'field' }, h('label', null, 'Wertermittlung durch'), wer)),
    h('div', { class: 'tablewrap' }, h('table', { class: 'wert-tab' },
      h('thead', null, h('tr', null, ['Art', 'Position', 'Menge', 'Einheit', 'Einzelwert €', 'Abschlag %', 'Wert', ''].map((t) => h('th', null, t)))), tbody)),
    h('div', { class: 'toolbar', style: 'margin-top:8px' }, h('button', { class: 'btn small', onclick: () => zeile({ kategorie: 'Einrichtungen' }).bez.focus() }, '+ Zeile'), h('div', { class: 'spacer' }),
      h('span', null, 'Ermittelter Wert: '), summeEl),
    vorlagen,
    h('div', { class: 'field', style: 'margin-top:12px' }, h('label', null, 'Bemerkung'), bem),
    h('p', { class: 'hint', style: 'margin-top:10px' }, 'Wert = Menge × Einzelwert, vermindert um den Abschlag für Alter und Zustand; Abzüge werden abgezogen. Maßgeblich sind die Richtlinien eures Landesverbands – hier wird das Ergebnis festgehalten und als Ablösesumme beim Pächterwechsel vorgeschlagen.'));
  return breiterDialog(x ? `Wertermittlung Garten ${nr} bearbeiten` : `Wertermittlung Garten ${nr}`, body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const positionen = zeilen.map((z) => ({ kategorie: z.kat.value, bezeichnung: z.bez.value, menge: parseNum(z.menge.value) ?? 0, einheit: z.einheit.value,
        einzelwert: parseNum(z.wert.value) ?? 0, abschlag: parseNum(z.abschlag.value) ?? 0 }));
      if (positionen.some((p) => [p.menge, p.einzelwert, p.abschlag].some((v) => Number.isNaN(v) || v < 0))) { toast('Menge, Einzelwert und Abschlag müssen Zahlen ab 0 sein.', 'err'); return false; }
      const daten = { gartennr: nr, datum: datum.value, anlass: anlass.value, wertermittler: wer.value, bemerkung: bem.value.trim(), positionen };
      try {
        if (x) await api('PUT', `/api/admin/wertermittlungen/${x.id}`, daten); else await api('POST', '/api/admin/wertermittlungen', daten);
        toast('Wertermittlung gespeichert', 'ok');
      } catch (e) { handleErr(e); return false; }
    } }]);
}

const wertermittlungDrucken = (x) => pdfDrucken({
  url: `/api/admin/wertermittlungen/${x.id}/pdf`, oeffnenUrl: `/api/admin/wertermittlungen/${x.id}/pdf/oeffnen`,
  titel: `Wertermittlung Garten ${x.gartennr}`, meldung: () => 'Die Wertermittlung wurde im PDF-Programm geöffnet – dort drucken.',
});

// ------------------------------------------------------------------ Zählerstand-Erinnerung

async function zaehlerErinnerungDialog(jahr) {
  let d;
  try { d = await api('GET', `/api/admin/zaehler-fehlen?year=${jahr}`); } catch (e) { handleErr(e); return; }
  const in14 = new Date(); in14.setDate(in14.getDate() + 14);
  const frist = h('input', { type: 'date', value: `${in14.getFullYear()}-${String(in14.getMonth() + 1).padStart(2, '0')}-${String(in14.getDate()).padStart(2, '0')}` });
  const mitMail = d.fehlend.filter((f) => f.email).length;
  const body = h('div', null,
    d.fehlend.length
      ? h('div', null,
        h('p', null, `Bei ${d.fehlend.length} von ${d.gesamt} Gärten mit Zählern fehlt noch ein aktueller Stand.`),
        h('div', { class: 'tablewrap', style: 'max-height:260px;overflow:auto' }, h('table', null,
          h('thead', null, h('tr', null, ['Garten', 'Name', 'Es fehlt', 'E-Mail'].map((t) => h('th', null, t)))),
          h('tbody', null, d.fehlend.map((f) => h('tr', null, h('td', null, f.gartennr), h('td', null, f.name), h('td', null, f.fehlt.join(' + ')),
            h('td', { class: 'klein' }, f.email || '–')))))),
        h('div', { class: 'field', style: 'margin-top:12px;max-width:16em' }, h('label', null, 'Bitte melden bis'), frist),
        h('p', { class: 'hint', style: 'margin-top:8px' }, `Der Aushang nennt nur die Gartennummern, keine Namen. Die E-Mail geht als Blindkopie (Bcc) an ${mitMail} Pächter mit hinterlegter Adresse – niemand sieht die Adressen der anderen.`))
      : leer('fertig', 'Alles gemeldet', `Für ${jahr} liegen von allen ${d.gesamt} Gärten mit Zählern aktuelle Stände vor.`));
  const knoepfe = [{ label: 'Schließen', value: false }];
  if (d.fehlend.length) {
    knoepfe.push({ label: 'Aushang drucken', value: true, action: async () => {
      pdfDrucken({ url: `/api/admin/zaehler-aushang?year=${jahr}&frist=${encodeURIComponent(frist.value)}`, oeffnenUrl: `/api/admin/zaehler-aushang/oeffnen?year=${jahr}&frist=${encodeURIComponent(frist.value)}`,
        titel: 'Zählerstände fehlen', meldung: () => 'Der Aushang wurde im PDF-Programm geöffnet – dort drucken.' });
      return false;
    } });
    if (mitMail) {
      knoepfe.push({ label: 'E-Mail vorbereiten', cls: 'primary', value: true, action: async () => {
        try {
          const r = await api('POST', `/api/admin/zaehler-mail?year=${jahr}&frist=${encodeURIComponent(frist.value)}`);
          toast(r.ohneMail ? `E-Mail an ${r.empfaenger} vorbereitet – ${r.ohneMail} ohne Adresse oder zu viele für einen Link.` : `E-Mail an ${r.empfaenger} vorbereitet.`, 'ok');
        } catch (e) { handleErr(e); }
        return false;
      } });
    }
  }
  return breiterDialog(`Fehlende Zählerstände ${jahr}`, body, knoepfe);
}

// ------------------------------------------------------------------ Berichte & Meldungen

async function loadMeldung(jahr) {
  if (S.meldungLoading) return;
  S.meldungLoading = true;
  try { S.meldung = await api('GET', `/api/admin/jahresmeldung?year=${jahr}`); } catch (e) { handleErr(e); S.meldung = false; }
  S.meldungLoading = false;
  render();
}

// Ablesung mit dem Handy: Ergebnisdatei vom Handy einlesen und zeigen, was übernommen wurde.
function handyErgebnisEinlesen() {
  const datei = h('input', { type: 'file', accept: '.json,application/json', style: 'display:none' });
  datei.addEventListener('change', async () => {
    if (!datei.files[0]) return;
    const fd = new FormData(); fd.append('file', datei.files[0]);
    try {
      const r = await api('POST', '/api/admin/ablesung-handy/import', fd, true);
      await reload();
      await modal('Zählerstände übernommen', h('div', null,
        h('p', null, `${r.uebernommen} Gärten wurden aktualisiert, ${r.unveraendert} waren schon so eingetragen.`),
        r.hinweise.length ? h('div', { class: 'banner warn' }, h('b', null, 'Bitte prüfen:'), h('ul', null, r.hinweise.map((x) => h('li', null, x)))) : h('p', { class: 'hint' }, 'Keine Auffälligkeiten.')),
      [{ label: 'Schließen', cls: 'primary', value: true }]);
    } catch (e) { handleErr(e); }
  });
  document.body.append(datei); datei.click();
  setTimeout(() => datei.remove(), 60000);
}

// Arbeitsstunden-Konto: Soll und Ist der Gemeinschaftsarbeit je Garten, mit der Ersatzzahlung, die sonst anfällt.
// Ehrenamts-Nachweis (Stunden und Ämter) als PDF; ohne Pächter: eine Seite je Mitglied mit Stunden oder Amt.
const ehrenamtDrucken = (jahr, paechterId) => pdfDrucken({
  url: `/api/admin/ehrenamt?year=${jahr}&paechterId=${encodeURIComponent(paechterId)}`,
  oeffnenUrl: `/api/admin/ehrenamt/oeffnen?year=${jahr}&paechterId=${encodeURIComponent(paechterId)}`,
  titel: 'Ehrenamt', meldung: () => 'Nachweis im PDF-Programm geöffnet – dort drucken.',
});

async function stundenkontoDialog(jahr) {
  let k;
  const laden = async () => { k = await api('GET', `/api/admin/stundenkonto?year=${jahr}`); };
  try { await laden(); } catch (e) { handleErr(e); return; }
  const inhalt = h('div');
  const zeichnen = () => {
    inhalt.replaceChildren(
      h('p', { class: 'hint' }, `Pflicht: ${nfFlex.format(k.pflicht)} Stunden je Garten, je fehlende Stunde ${eur(k.jeStunde)}. ${k.erfuellt} von ${k.zeilen.length} Gärten haben die Pflicht erfüllt, zusammen ${nfFlex.format(k.summe)} Stunden geleistet. Die Ersatzzahlung kommt automatisch in die Jahresrechnung.`),
      h('table', { class: 'tbl' },
        h('thead', null, h('tr', null, ['Garten', 'Name', 'Geleistet', 'Fehlt', 'Ersatzzahlung', ''].map((t, i) => h('th', { class: i > 1 && i < 5 ? 'r' : '' }, t)))),
        h('tbody', null, k.zeilen.map((z) => h('tr', null,
          h('td', null, z.gartennr), h('td', null, z.name),
          h('td', { class: 'r' }, nfFlex.format(z.ist), z.einsaetze ? h('span', { class: 'hint' }, ` (${z.einsaetze} Einsätze)`) : null),
          h('td', { class: 'r' }, z.erfuellt ? h('span', { class: 'badge ok' }, 'erfüllt') : nfFlex.format(z.fehlt)),
          h('td', { class: 'r' }, z.ersatz ? eur(z.ersatz) : '–'),
          h('td', { class: 'r' }, h('button', { class: 'btn small', onclick: () => nachtragen(z) }, '+ Stunden'), ' ',
            h('button', { class: 'btn small', title: 'Bescheinigung über die ehrenamtliche Tätigkeit (Stunden und Ämter)', onclick: () => ehrenamtDrucken(jahr, z.paechterId) }, 'Nachweis')))))),
      h('p', { class: 'hint' }, `Zu erwartende Ersatzzahlungen insgesamt: ${eur(k.ersatzSumme)}`));
  };
  const nachtragen = async (z) => {
    const std = h('input', { type: 'number', step: '0.5', min: '-24', max: '24', value: '', style: 'width:6em', placeholder: 'z. B. 3' });
    const ok = await modal(`Stunden nachtragen – Garten ${z.gartennr}`, h('div', null,
      h('p', { class: 'hint' }, `${z.name} hat bisher ${nfFlex.format(z.ist)} Stunden. Für Einsätze mit Termin im Jahresplaner trägst du die Stunden dort ein. Hier geht es um einzelne Stunden außerhalb davon. Mit einer negativen Zahl nimmst du Stunden zurück.`),
      h('label', null, 'Stunden ', std)), [{ label: 'Abbrechen', value: false }, { label: 'Eintragen', cls: 'primary', value: true }]);
    if (!ok) return;
    try {
      await api('POST', '/api/admin/stundenkonto/buchen', { paechterId: z.paechterId, jahr, stunden: Number(std.value) });
      await laden(); zeichnen(); await reload(); toast('Stunden eingetragen', 'ok');
    } catch (e) { handleErr(e); }
  };
  zeichnen();
  await breiterDialog(`Arbeitsstunden ${jahr}`, inhalt, [
    { label: 'Alle Nachweise (PDF)', value: 'nachweise', action: () => { ehrenamtDrucken(jahr, ''); return false; } },
    { label: 'Schließen', cls: 'primary', value: true }]);
}

// ------------------------------------------------------------------ Mitgliederbefragung (Berichte)

const BEFRAGUNG_ARTEN = { janein: 'Ja / Nein', skala: 'Skala 1–5', wahl: 'Auswahl', text: 'Freitext' };
const befragungOptionen = (f) => (f.art === 'janein' ? ['Ja', 'Nein', 'Enthaltung'] : f.art === 'skala' ? ['1 – trifft nicht zu', '2', '3', '4', '5 – trifft voll zu'] : f.art === 'wahl' ? (f.optionen || []) : []);

function befragungPdf(b, art, titel) {
  return pdfDrucken({ url: `/api/admin/befragungen/${encodeURIComponent(b.id)}/pdf?art=${art}`, oeffnenUrl: `/api/admin/befragungen/${encodeURIComponent(b.id)}/pdf/oeffnen?art=${art}`,
    titel, meldung: () => `${titel} im PDF-Programm geöffnet – dort drucken.` });
}

async function befragungenDialog() {
  let liste = [];
  const inhalt = h('div');
  const laden = async () => { liste = await api('GET', '/api/admin/befragungen'); };
  const zeichnen = () => {
    inhalt.replaceChildren(
      h('p', { class: 'hint' }, 'Drei Schritte: Fragebogen drucken und verteilen, ausgefüllte Bögen mit dem Erfassungsbogen auszählen, Zahlen eintragen – die Auswertung kommt ohne Namen in die Versammlungsunterlagen.'),
      liste.length ? h('div', null, liste.map((b) => h('div', { class: 'bericht-kachel' },
        h('div', null, h('b', null, b.titel), h('p', { class: 'hint' }, `${b.fragen.length} Fragen${b.datum ? ' · bis ' + deDate(b.datum) : ''} · ${b.rueckgabe ? b.rueckgabe + ' Bögen ausgezählt' : 'noch nicht ausgezählt'}`)),
        h('div', { class: 'row' },
          h('button', { class: 'btn small', onclick: () => befragungPdf(b, 'fragebogen', 'Fragebogen') }, 'Fragebogen'),
          h('button', { class: 'btn small', onclick: () => befragungPdf(b, 'erfassung', 'Erfassungsbogen') }, 'Erfassungsbogen'),
          h('button', { class: 'btn small', onclick: () => ergebnis(b) }, 'Ergebnis eintragen'),
          h('button', { class: 'btn small', disabled: !b.rueckgabe, onclick: () => befragungPdf(b, 'auswertung', 'Auswertung') }, 'Auswertung'),
          h('button', { class: 'btn small', onclick: () => bearbeiten(b) }, 'Bearbeiten'),
          h('button', { class: 'btn small', onclick: () => loeschen(b) }, 'Löschen'))))) : h('p', { class: 'hint' }, 'Noch keine Befragung angelegt.'));
  };
  const speichern = async (b) => { await api('POST', '/api/admin/befragungen', b); await laden(); zeichnen(); };
  const loeschen = async (b) => {
    if (!(await confirmBox(`Befragung »${b.titel}« mit allen Ergebnissen löschen?`, 'Löschen', true))) return;
    try { await api('DELETE', `/api/admin/befragungen/${b.id}`); await laden(); zeichnen(); } catch (e) { handleErr(e); }
  };
  const ergebnis = async (b) => {
    const bogen = h('input', { type: 'number', min: '0', max: '100000', value: b.rueckgabe || '', style: 'width:6em' });
    const felder = b.fragen.map((f) => {
      const opt = befragungOptionen(f);
      return { f, opt, zahlen: opt.map((_, k) => h('input', { type: 'number', min: '0', max: '100000', value: (f.zaehlung && f.zaehlung[k]) || '', style: 'width:5em' })),
        anm: h('textarea', { rows: 2, maxlength: 1500, placeholder: f.art === 'text' ? 'Zusammenfassung der Antworten' : 'Anmerkung (optional)' }, f.anmerkung || '') };
    });
    const ok = await breiterDialog(`Ergebnis: ${b.titel}`, h('div', { class: 'form' },
      h('div', { class: 'field wide' }, h('label', null, 'Anzahl ausgefüllter Bögen'), bogen),
      felder.map((x, i) => h('div', { class: 'field wide' }, h('label', null, `${i + 1}. ${x.f.text}`),
        x.opt.map((o, k) => h('div', { class: 'row' }, h('span', { style: 'min-width:12em' }, o), x.zahlen[k])), x.anm))),
    [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      try {
        await speichern({ ...b, rueckgabe: Number(bogen.value) || 0, fragen: b.fragen.map((f, i) => ({ ...f, zaehlung: felder[i].zahlen.map((z) => Number(z.value) || 0), anmerkung: felder[i].anm.value })) });
      } catch (e) { handleErr(e); return false; }
    } }]);
    if (ok) toast('Ergebnis gespeichert', 'ok');
  };
  const bearbeiten = async (b) => {
    const neu = !b;
    const f = {
      titel: h('input', { type: 'text', maxlength: 120, value: b ? b.titel : '', placeholder: 'z. B. Wünsche für die Anlage' }),
      datum: h('input', { type: 'date', value: b ? b.datum : '' }),
      einl: h('textarea', { rows: 3, maxlength: 1500, placeholder: 'Kurzer Text oben auf dem Fragebogen (optional)' }, b ? b.einleitung || '' : ''),
    };
    let fragen = b ? b.fragen.map((x) => ({ ...x })) : [{ text: '', art: 'janein' }];
    const reihen = h('div');
    const zeile = () => {
      reihen.replaceChildren(...fragen.map((q, i) => h('div', { class: 'row', style: 'align-items:flex-start;margin-bottom:.5em' },
        h('input', { type: 'text', maxlength: 300, value: q.text, placeholder: `Frage ${i + 1}`, style: 'flex:1 1 16em', oninput: (e) => { q.text = e.target.value; } }),
        h('select', { onchange: (e) => { q.art = e.target.value; zeile(); } }, Object.entries(BEFRAGUNG_ARTEN).map(([k, v]) => h('option', { value: k, selected: q.art === k }, v))),
        q.art === 'wahl' ? h('input', { type: 'text', value: (q.optionen || []).join('; '), placeholder: 'Antworten mit ; trennen', style: 'flex:1 1 14em', oninput: (e) => { q.optionen = e.target.value.split(';').map((s) => s.trim()).filter(Boolean); } }) : null,
        h('button', { type: 'button', class: 'btn small', title: 'Frage entfernen', disabled: fragen.length < 2, onclick: () => { fragen.splice(i, 1); zeile(); } }, 'Entfernen'))));
    };
    zeile();
    const ok = await breiterDialog(neu ? 'Neue Befragung' : 'Befragung bearbeiten', h('div', { class: 'form' },
      h('div', { class: 'field wide' }, h('label', null, 'Titel *'), f.titel),
      h('div', { class: 'field' }, h('label', null, 'Abgabe bis'), f.datum),
      h('div', { class: 'field wide' }, h('label', null, 'Einleitung'), f.einl),
      h('div', { class: 'field wide' }, h('label', null, 'Fragen'), reihen,
        h('button', { type: 'button', class: 'btn small', onclick: () => { if (fragen.length < 30) { fragen.push({ text: '', art: 'janein' }); zeile(); } } }, '+ Frage'),
        h('span', { class: 'fhint' }, 'Wer Fragen nach dem Auszählen ändert, sollte die Ergebnisse danach prüfen.'))),
    [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      try { await speichern({ id: b ? b.id : '', titel: f.titel.value, datum: f.datum.value, einleitung: f.einl.value, rueckgabe: b ? b.rueckgabe : 0, fragen }); } catch (e) { handleErr(e); return false; }
    } }]);
    if (ok) toast('Befragung gespeichert', 'ok');
  };
  try { await laden(); } catch (e) { handleErr(e); return; }
  zeichnen();
  const r = await breiterDialog('Mitgliederbefragung', inhalt, [{ label: '+ Neue Befragung', value: 'neu', action: () => { bearbeiten(null); return false; } }, { label: 'Schließen', cls: 'primary', value: true }]);
  return r;
}

// Statistik: wenige einfache Säulen-Diagramme über die Jahre, jedes mit einem Satz, was man daran sieht.
async function statistikDialog() {
  let r;
  try { r = await api('GET', '/api/admin/statistik'); } catch (e) { handleErr(e); return; }
  const v = r.kosten.verein;
  if (!v.length) { toast('Noch keine Jahre vorhanden', 'info'); return; }
  const karte = (titel, satz, daten, fmt) => h('div', { class: 'card' }, h('h4', null, titel), h('p', { class: 'hint' }, satz),
    barChart(daten, { valueFmt: fmt }));
  const eurK = (x) => eur(x);
  const prozent = (x) => `${nfFlex.format(x)} %`;
  const tage = (x) => `${nfFlex.format(x)} Tage`;
  const z = r.zahlung;
  const inhalt = h('div', { class: 'statistik' },
    h('p', { class: 'hint' }, 'So hat sich der Verein über die Jahre entwickelt. Jede Säule ist ein Jahr.'),
    h('div', { class: 'meldung-raster' },
      karte('Wasser', 'Wie viele Kubikmeter hat der ganze Verein verbraucht?', v.map((x) => ({ label: String(x.jahr), value: x.wasser || 0 })), (x) => `${nfFlex.format(x)} m³`),
      karte('Strom', 'Wie viele Kilowattstunden hat der ganze Verein verbraucht?', v.map((x) => ({ label: String(x.jahr), value: x.strom || 0 })), (x) => `${nfFlex.format(x)} kWh`),
      karte('Ausgaben', 'Was hat der Verein ausgegeben?', v.map((x) => ({ label: String(x.jahr), value: x.ausgaben || 0 })), eurK),
      karte('Rechnungen', 'Wie viel wurde den Pächtern insgesamt in Rechnung gestellt?', v.map((x) => ({ label: String(x.jahr), value: x.rechnungen || 0 })), eurK),
      z.length ? karte('Bezahlt', 'Wie viel Prozent der Rechnungen sind bezahlt? Je näher an 100, desto besser.', z.map((x) => ({ label: String(x.jahr), value: x.quote })), prozent) : null,
      z.some((x) => x.tage > 0) ? karte('Zahlungsdauer', 'Nach wie vielen Tagen wird im Durchschnitt bezahlt? Weniger ist besser.', z.filter((x) => x.tage > 0).map((x) => ({ label: String(x.jahr), value: x.tage })), tage) : null));
  await modal('Statistik', inhalt, [{ label: 'Schließen', cls: 'primary', value: true }]);
}

async function kostenvergleichDialog() {
  let k;
  try { k = await api('GET', '/api/admin/kostenvergleich'); } catch (e) { handleErr(e); return; }
  if (!k.verein.length) { toast('Noch keine Jahre vorhanden', 'info'); return; }
  const wert = (v, einheit) => (v == null ? '–' : `${nfFlex.format(v)}${einheit}`);
  const reihe = (feld, titel, einheit) => h('div', { class: 'card' },
    h('h4', null, titel),
    barChart(k.verein.map((v) => ({ label: String(v.jahr), value: v[feld] || 0 })), { unit: einheit }));
  const inhalt = h('div', null,
    k.hinweise.length ? h('div', { class: 'hinweis warn' }, h('b', null, 'Auffällig im Verein: '), k.hinweise.join(' · ')) : null,
    h('p', { class: 'hint' }, 'Verbrauch und Ausgaben der letzten Jahre. Auffällig ist ein Garten, dessen Verbrauch gegenüber dem Vorjahr um die Hälfte gestiegen ist (und über der Mindestabweichung liegt) – oft ein Rohrbruch, tropfender Hahn oder defekter Zähler.'),
    h('table', { class: 'tbl' },
      h('thead', null, h('tr', null, ['Jahr', 'Wasser', 'Strom', 'Ausgaben', 'Rechnungen'].map((t, i) => h('th', { class: i ? 'r' : '' }, t)))),
      h('tbody', null, k.verein.slice().reverse().map((v) => h('tr', null,
        h('td', null, String(v.jahr)), h('td', { class: 'r' }, wert(v.wasser, ' m³')), h('td', { class: 'r' }, wert(v.strom, ' kWh')),
        h('td', { class: 'r' }, eur(v.ausgaben)), h('td', { class: 'r' }, eur(v.rechnungen)))))),
    h('div', { class: 'toolbar' }, reihe('wasser', 'Wasser gesamt (m³)', ' m³'), reihe('ausgaben', 'Ausgaben (€)', ' €')),
    h('h4', null, 'Verbrauch je Garten'),
    h('table', { class: 'tbl' },
      h('thead', null, h('tr', null, h('th', null, 'Garten'), h('th', null, 'Pächter'),
        ...k.jahre.map((y) => h('th', { class: 'r' }, String(y))), h('th', null, 'Hinweis'))),
      h('tbody', null, k.gaerten.map((g) => h('tr', null,
        h('td', null, g.gartennr), h('td', null, g.name),
        ...g.werte.map((w) => h('td', { class: 'r' }, w.wasser == null && w.strom == null ? '–' : `${w.wasser == null ? '–' : nfFlex.format(w.wasser)} / ${w.strom == null ? '–' : nfFlex.format(w.strom)}`)),
        h('td', null, g.hinweise.length ? h('span', { class: 'badge warn' }, g.hinweise.join('; ')) : ''))))),
    h('p', { class: 'hint' }, 'Spalten: Wasser (m³) / Strom (kWh).'));
  await breiterDialog('Kostenvergleich über die Jahre', inhalt, [{ label: 'Schließen', cls: 'primary', value: true }]);
}

async function papierkorbDialog() {
  let liste, pliste;
  const laden = async () => {
    liste = await api('GET', '/api/admin/papierkorb');
    pliste = await api('GET', '/api/admin/paechter-papierkorb');
  };
  try { await laden(); } catch (e) { handleErr(e); return; }
  const inhalt = h('div');
  const zeichnen = () => {
    inhalt.replaceChildren(
      h('p', { class: 'hint' }, 'Gelöschte Mängel, Termine, Beschlüsse und Verträge bleiben 30 Tage hier liegen und lassen sich zurückholen. Gelöschte Pächter bleiben, bis du sie unter »Pächter« endgültig löschst. Die Warteliste hat aus Datenschutzgründen keinen Papierkorb.'),
      !liste.length && !pliste.length ? leer('Der Papierkorb ist leer.') : h('table', { class: 'tbl' },
        h('thead', null, h('tr', null, ['Art', 'Eintrag', 'Gelöscht', 'Noch', ''].map((t) => h('th', null, t)))),
        h('tbody', null,
          pliste.map((p) => h('tr', null,
            h('td', null, 'Pächter'), h('td', null, `${p.name}${p.gartennr ? ' (Garten ' + p.gartennr + ')' : ''}`), h('td', null, deDate(p.geloeschtAm)), h('td', null, 'unbegrenzt'),
            h('td', { class: 'r' }, h('button', { class: 'btn small', onclick: () => paechterZurueck(p) }, 'Zurückholen')))),
          liste.map((z) => h('tr', null,
            h('td', null, z.artText), h('td', null, z.titel), h('td', null, deDate(z.geloeschtAm)), h('td', null, `${z.nochTage} Tage`),
            h('td', { class: 'r' }, h('button', { class: 'btn small', onclick: () => zurueckholen(z) }, 'Zurückholen')))))));
  };
  const zurueckholen = async (z) => {
    try {
      await api('POST', `/api/admin/papierkorb/${encodeURIComponent(z.id)}/wiederherstellen`);
      await laden(); zeichnen(); await reload(); toast(`${z.artText} zurückgeholt`, 'ok');
    } catch (e) { handleErr(e); }
  };
  const paechterZurueck = async (p) => {
    try {
      await api('POST', `/api/admin/paechter/${encodeURIComponent(p.id)}/wiederherstellen`);
      S.papierkorb = null; await laden(); zeichnen(); await reload(); toast('Pächter zurückgeholt', 'ok');
    } catch (e) { handleErr(e); }
  };
  const leeren = async () => {
    if (!liste.length || !(await confirmBox('Mängel, Termine, Beschlüsse und Verträge im Papierkorb werden endgültig gelöscht. Pächter bleiben unberührt.', 'Papierkorb leeren', true))) return;
    try { await api('DELETE', '/api/admin/papierkorb'); await laden(); zeichnen(); } catch (e) { handleErr(e); }
  };
  zeichnen();
  const ok = await breiterDialog('Papierkorb', inhalt, [{ label: 'Papierkorb leeren', value: 'leeren' }, { label: 'Schließen', cls: 'primary', value: true }]);
  if (ok === 'leeren') { await leeren(); }
}

// Alle druckbaren Vordrucke an einer Stelle.
async function vordruckeDialog() {
  const st = S.state;
  const zeile = (titel, text, ...knoepfe) => h('div', { class: 'vordruck' }, h('div', null, h('b', null, titel), h('div', { class: 'hint' }, text)), h('div', { class: 'row' }, ...knoepfe));
  const knopf = (label, fn) => h('button', { class: 'btn', onclick: fn }, label);
  const body = h('div', { class: 'vordrucke' },
    h('h4', null, 'Für den Rundgang'),
    zeile(`Ablesebogen ${st.currentYear}`, 'Alle Gärten mit Zählern und Vorjahresständen.', knopf('Drucken', () => ablesebogenDrucken(st.currentYear))),
    zeile('Saison-Liste Wasser', 'Frühjahr (Wasser an) oder Herbst (winterfest).', knopf('Frühjahr', () => saisonlisteDrucken('fruehjahr')), knopf('Herbst', () => saisonlisteDrucken('herbst'))),
    zeile('Begehungsliste', 'Mit den noch offenen Mängeln je Garten.', knopf('Drucken', begehungslisteDrucken)),
    h('h4', null, 'Für Schaukasten und Vorstand'),
    zeile('Aushang', 'Kurze Mitteilung mit großer Überschrift, mit Vorlagen.', knopf('Erstellen', aushangDialog)),
    zeile(`Vereinstermine ${new Date().getFullYear()}`, 'Jahresplaner als Aushang.', knopf('Drucken', () => jahresplanDrucken(new Date().getFullYear()))),
    zeile('Lageplan', 'Als Aushang ohne Namen oder für den Vorstand mit Namen.', knopf('Drucken', () => lageplanDrucken(true))),
    zeile('Pächterliste', 'Mit Anschrift, Kontakt und Zählern (vertraulich).', knopf('Drucken', paechterlisteDrucken)));
  await breiterDialog('Vordrucke und Aushänge', body, [{ label: 'Schließen', cls: 'primary', value: true }]);
}

function adminBerichte() {
  const st = S.state;
  const jahr = S.berichtJahr || st.currentYear;
  if (S.meldung == null || (S.meldung && S.meldung.jahr !== jahr)) { if (!S.meldungLoading) loadMeldung(jahr); }
  const m = S.meldung && S.meldung.jahr === jahr ? S.meldung : null;
  const jahrSel = h('select', { onchange: (e) => { S.berichtJahr = Number(e.target.value); S.meldung = null; render(); } },
    st.years.map((y) => h('option', { value: y, selected: y === jahr }, String(y))));
  const kachel = (ic, titel, text, knopf) => h('div', { class: 'bericht-kachel' },
    h('span', { class: 'qlico' }, icon(ic, 24)), h('div', null, h('b', null, titel), h('p', { class: 'hint' }, text)), knopf);
  const fmt = (z) => (z.einheit === '€' ? eur(z.wert) : `${nfFlex.format(z.wert)}${z.einheit ? ' ' + z.einheit : ''}`);
  // PDF-Knopf für Kacheln, die sonst nur einen Dialog oder eine Excel-Datei haben
  const pdfKnopf = (pfad, titel, query = '') => h('button', { class: 'btn', onclick: () => pdfDrucken({ url: `/api/admin/${pfad}/pdf${query}`, oeffnenUrl: `/api/admin/${pfad}/pdf/oeffnen${query}`, titel, meldung: () => `${titel} im PDF-Programm geöffnet – dort drucken.` }) }, 'Als PDF');
  const gruppen = [];
  for (const z of (m ? m.zeilen : [])) {
    if (!gruppen.length || gruppen[gruppen.length - 1].g !== z.gruppe) gruppen.push({ g: z.gruppe, z: [] });
    gruppen[gruppen.length - 1].z.push(z);
  }
  return h('div', null,
    h('p', { class: 'hint' }, 'Alles, was der Vorstand im Jahr vorlegen oder melden muss, an einer Stelle.'),
    h('div', { class: 'toolbar' }, h('label', null, 'Jahr'), jahrSel),
    h('h3', { class: 'bericht-gruppe' }, 'Für Versammlung und Verband'),
    h('p', { class: 'hint' }, 'Zahlen und Unterlagen, die der Vorstand vorlegt oder meldet.'),
    h('div', { class: 'bericht-raster' },
      kachel('bericht', `Jahresbericht ${jahr}`, 'Für die Mitgliederversammlung: Kennzahlen mit Vorjahresvergleich, Kasse, Ausgaben, Verlauf, Termine – ohne Namen.',
        h('button', { class: 'btn primary', onclick: () => jahresberichtDrucken(jahr) }, 'Als PDF')),
      kachel('bericht', `Jahresabrechnung ${jahr}`, 'Alle ausgestellten Rechnungen des Jahres in einer Datei: vorn eine Übersicht mit Summen, danach je Pächter eine Seite.',
        pdfKnopf('jahresabrechnung', `Jahresabrechnung ${jahr}`, `?year=${jahr}`)),
      kachel('waage', `Jahresmeldung ${jahr}`, 'Zahlen für den Landes- oder Regionalverband: Parzellen, Flächen, Wechsel, Warteliste, Stunden, Abgaben.',
        h('div', { class: 'row' }, h('a', { class: 'btn primary', href: `/api/admin/jahresmeldung?year=${jahr}&format=xlsx` }, 'Als Excel'), pdfKnopf('jahresmeldung', `Jahresmeldung ${jahr}`, `?year=${jahr}`))),
      kachel('uhr', `Arbeitsstunden ${jahr}`, 'Soll und Ist der Gemeinschaftsarbeit je Garten, mit der Ersatzzahlung, die sonst anfällt. Einzelne Stunden lassen sich nachtragen.',
        h('div', { class: 'row' }, h('button', { class: 'btn', onclick: () => stundenkontoDialog(jahr) }, 'Ansehen'), pdfKnopf('stundenkonto', `Arbeitsstunden ${jahr}`, `?year=${jahr}`))),
      kachel('bericht', 'Statistik', 'Wasser, Strom, Ausgaben, Rechnungen und Zahlungsmoral über die Jahre als einfache Diagramme.',
        h('button', { class: 'btn', onclick: statistikDialog }, 'Ansehen')),
      kachel('bericht', 'Kostenvergleich', 'Wasser, Strom und Ausgaben über die Jahre, je Garten mit Hinweis auf auffälligen Mehrverbrauch.',
        h('div', { class: 'row' }, h('button', { class: 'btn', onclick: kostenvergleichDialog }, 'Ansehen'), pdfKnopf('kostenvergleich', 'Kostenvergleich'))),
      kachel('personen', 'Mitgliederbefragung', 'Fragebogen zum Ausdrucken, Erfassungsbogen zum Auszählen und anonyme Auswertung für die Versammlung.',
        h('button', { class: 'btn', onclick: befragungenDialog }, 'Öffnen'))
    ),
    h('h3', { class: 'bericht-gruppe' }, 'Rundgang, Zähler und Vordrucke'),
    h('p', { class: 'hint' }, 'Alles zum Ausdrucken und für den Gang durch die Anlage.'),
    h('div', { class: 'bericht-raster' },
      kachel('bericht', 'Vordrucke und Aushänge', 'Ablesebogen, Saison-Liste, Begehungsliste, Pächterliste, Lageplan, Vereinstermine und Aushang – alles Druckbare an einer Stelle.',
        h('button', { class: 'btn primary', onclick: vordruckeDialog }, 'Öffnen')),
      kachel('zaehler', `Ablesung mit dem Handy ${st.currentYear}`, 'Eine Datei fürs Handy, die ohne Internet funktioniert: Zählerstände beim Rundgang eintippen, das Ergebnis hier wieder einlesen. Die Datei enthält Namen – vertraulich behandeln.',
        h('div', { class: 'toolbar-gruppe' }, h('a', { class: 'btn', href: '/api/admin/ablesung-handy', download: '' }, 'Datei fürs Handy'), ' ',
          h('button', { class: 'btn', onclick: handyErgebnisEinlesen }, 'Ergebnis einlesen'))),
      kachel('zaehler', `Fehlende Zählerstände ${st.currentYear}`, 'Wer hat noch keinen Stand gemeldet? Mit Aushang ohne Namen und E-Mail-Erinnerung.',
        h('button', { class: 'btn', onclick: () => zaehlerErinnerungDialog(st.currentYear) }, 'Prüfen')),
      kachel('personen', 'Willkommensmappe & Exposé', 'Texte für die Mappe neuer Pächter und den Steckbrief freier Gärten: Ansprechpartner und das Wichtigste aus der Gartenordnung.',
        h('button', { class: 'btn', onclick: vereinsinfoDialog }, 'Texte bearbeiten'))
    ),
    h('h3', { class: 'bericht-gruppe' }, 'Sicherheit und Übergabe'),
    h('p', { class: 'hint' }, 'Prüfen, zurückholen, an den nächsten Vorstand weitergeben.'),
    h('div', { class: 'bericht-raster' },
      kachel('daten', 'Wartungs-Check', 'Ist alles in Ordnung? Datenordner, Sicherungen, zweiter Ordner, Passwort und Programmversion auf einen Blick.',
        h('div', { class: 'row' }, h('button', { class: 'btn', onclick: wartungsCheck }, 'Prüfen'), pdfKnopf('wartung', 'Wartungs-Check'))),
      kachel('daten', 'Daten aufräumen', 'Findet doppelte Pächter, doppelte Nummern, fehlende Anschriften und ungültige IBAN – und führt mit einem Klick zur Stelle.',
        h('button', { class: 'btn', onclick: datencheckDialog }, 'Prüfen')),
      kachel('daten', 'Abgleich zwischen zwei Rechnern', 'Kassier und Schriftführer arbeiten getrennt? Per USB-Stick bringst du beide Stände zusammen – es kommen nur neue Einträge dazu.',
        h('button', { class: 'btn', onclick: abgleichDialog }, 'Öffnen')),
      kachel('daten', 'Papierkorb', 'Gelöschte Mängel, Termine, Beschlüsse und Verträge 30 Tage lang zurückholen.',
        h('button', { class: 'btn', onclick: papierkorbDialog }, 'Öffnen')),
      kachel('daten', 'Amtsübergabe', 'Vereinsakte für den neuen Vorstand: alle Daten und Dateien plus »Lies mich zuerst« mit Zahlen, Regeln und offenen Punkten.',
        h('button', { class: 'btn', onclick: vereinsakteErstellen }, 'Erstellen'))
    ),
    h('h3', null, `Jahresmeldung ${jahr} – Übersicht`),
    !m ? h('p', { class: 'hint' }, S.meldung === false ? 'Konnte nicht geladen werden.' : 'Wird geladen …')
      : h('div', { class: 'meldung-raster' }, gruppen.map((g) => h('div', { class: 'card' },
        h('h4', null, g.g),
        h('table', { class: 'meldung' }, h('tbody', null, g.z.map((z) => h('tr', null, h('td', null, z.angabe), h('td', { class: 'num' }, h('b', null, fmt(z)))))))))),
    m ? h('p', { class: 'hint', style: 'margin-top:12px' }, 'Neu vergebene Gärten zählen nur Vergaben über Pächterwechsel oder Warteliste, nicht das erstmalige Eintragen langjähriger Pächter. Das Formular eures Verbands kann andere Angaben verlangen – die Excel-Datei lässt sich frei ergänzen.') : null);
}
