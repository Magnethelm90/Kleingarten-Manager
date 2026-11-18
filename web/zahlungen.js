'use strict';
/* Kleingarten-Manager – Oberfläche, Teil: Zahlungen und Spenden. Gemeinsamer Gültigkeitsbereich mit den anderen Dateien (siehe index.html für die Reihenfolge). */

// ------------------------------------------------------------------ Tab: Zahlungen

const deDate = (iso) => (iso ? iso.split('-').reverse().join('.') : '');

// volle Jahre seit einem Datum (JJJJ-MM-TT), Geburtstags-genau gerechnet
function jahreSeit(iso) {
  const seit = new Date(iso + 'T00:00:00');
  const now = new Date();
  let jahre = now.getFullYear() - seit.getFullYear();
  const schonGehabt = now.getMonth() > seit.getMonth() || (now.getMonth() === seit.getMonth() && now.getDate() >= seit.getDate());
  if (!schonGehabt) jahre--;
  return Math.max(0, jahre);
}
// isoInTagen: heutiges Datum (Ortszeit) plus n Tage als JJJJ-MM-TT.
const isoInTagen = (n) => { const d = new Date(); d.setDate(d.getDate() + n); const p = (x) => String(x).padStart(2, '0'); return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`; };
const todayIso = () => isoInTagen(0);
const payTotal = (e) => Math.round((Math.abs(e.gesamt) - (e.gutschriften || []).reduce((a, g) => a + g.betrag, 0)) * 100) / 100;
const payPaid = (e) => (e.bezahltAm ? (e.bezahltBetrag != null ? e.bezahltBetrag : payTotal(e)) : 0);
const payOpen = (e) => Math.round((payTotal(e) - payPaid(e)) * 100) / 100;
const payIsOpen = (e) => payOpen(e) > 0.004;

async function savePayment(e, patch, opts) {
  const vorher = { bezahltAm: e.bezahltAm || '', bezahltBetrag: e.bezahltBetrag ?? null, notiz: e.notiz || '' };
  const body = { bezahltAm: e.bezahltAm || '', bezahltBetrag: e.bezahltBetrag ?? null, notiz: e.notiz || '', ...patch };
  await api('PUT', `/api/payment/${e.id}`, body);
  Object.assign(e, { bezahltAm: body.bezahltAm, bezahltBetrag: body.bezahltAm ? body.bezahltBetrag : null, notiz: body.notiz });
  // volle Zahlung: Betrag nicht separat führen
  if (e.bezahltBetrag != null && Math.round(e.bezahltBetrag * 100) === Math.round(payTotal(e) * 100)) e.bezahltBetrag = null;
  if (e.ratenplan && e.ratenplan.length) e.faellig = ratenFaellig(e);
  if (e.bezahltAm && payOpen(e) < -0.004) toast(`Hinweis: ${eur(-payOpen(e))} mehr als der Rechnungsbetrag eingetragen.`, 'ok');
  else if (!(opts && opts.stumm)) {
    const text = !e.bezahltAm ? 'Zahlung zurückgenommen' : payIsOpen(e) ? `Teilzahlung vermerkt – offen bleiben ${eur(payOpen(e))}` : e.gesamt < 0 ? 'Auszahlung vermerkt' : 'Rechnung als bezahlt vermerkt';
    toast(text, 'ok', { label: 'Rückgängig', fn: async () => {
      try { await savePayment(e, vorher, { stumm: true }); toast('Zurückgenommen', 'ok'); await reload(); } catch (er) { handleErr(er); }
    } });
  }
  // war das die letzte offene Rechnung des Jahres?
  const st = S.state, forderungen = (st.archive || []).filter((x) => x.status === 'gueltig' && x.gesamt > 0);
  S.erfolgGezeigt = S.erfolgGezeigt || {};
  if (e.bezahltAm && forderungen.length >= 3 && !forderungen.some(payIsOpen) && !S.erfolgGezeigt[st.year]) {
    S.erfolgGezeigt[st.year] = true;
    erfolgsHinweis(`Alle Rechnungen ${st.year} sind bezahlt`, `${forderungen.length} Rechnungen, zusammen ${eur(forderungen.reduce((a, x) => a + payTotal(x), 0))}. Gute Arbeit!`);
  }
}

// Gutschriften zu einer Rechnung: Liste mit PDF-Knopf; nur die jüngste lässt sich zurücknehmen.
function gutschriftListe(e) {
  const l = e.gutschriften || [];
  if (!l.length) return null;
  return h('div', { class: 'gutschriften' }, h('div', { class: 'hint' }, `Ursprünglicher Rechnungsbetrag ${eur(Math.abs(e.gesamt))}, abzüglich Gutschriften`),
    ...l.map((g, i) => h('div', { class: 'zeile' },
      h('span', { class: 'mono' }, g.nummer), h('span', null, `${deDate(g.datum)} · −${eur(g.betrag)} · ${g.grund}`),
      h('span', { class: 'aktion' },
        h('button', { class: 'btn small', onclick: () => pdfDrucken({ url: `/api/admin/rechnungen/${e.id}/gutschrift?nr=${i + 1}`, oeffnenUrl: `/api/admin/rechnungen/${e.id}/gutschrift/oeffnen?nr=${i + 1}`,
          titel: g.nummer, meldung: () => 'Gutschrift im PDF-Programm geöffnet – dort drucken.' }) }, 'PDF'),
        i === l.length - 1 ? h('button', { class: 'btn small', title: 'Jüngste Gutschrift zurücknehmen', onclick: async (ev) => {
          if (!(await confirmBox(`Gutschrift ${g.nummer} zurücknehmen? Der Rechnungsbetrag steigt wieder um ${eur(g.betrag)}.`, 'Zurücknehmen'))) return;
          const dlg = ev.target.closest('dialog');
          try {
            await api('DELETE', `/api/admin/rechnungen/${e.id}/gutschrift`);
            await reload();
            toast('Gutschrift zurückgenommen.', 'ok');
            // Zahlungsdialog schließen: seine Angaben (Betrag, Liste) stammen noch von vorher
            if (dlg && dlg.isConnected) dlg.dispatchEvent(new Event('cancel', { cancelable: true }));
          } catch (er) { handleErr(er); }
        } }, 'Zurücknehmen') : null))));
}

function gutschriftPanel(e) {
  const rest = payTotal(e);
  const betrag = h('input', { type: 'text', inputmode: 'decimal', style: 'width:120px' });
  const datum = h('input', { type: 'date', value: todayIso() });
  const grund = h('input', { type: 'text', maxlength: 300, style: 'width:100%', placeholder: 'z. B. Wasserzähler falsch abgelesen' });
  const body = h('div', { class: 'form' },
    h('p', { class: 'hint' }, `Rechnung ${e.nummer} (${e.name}), aktueller Betrag ${eur(rest)}. Die Rechnung selbst bleibt unverändert; es entsteht ein eigenes Schreiben mit eigener Nummer. War sie schon bezahlt, ergibt sich ein Betrag zur Erstattung; diesen trägst du nach der Überweisung im Kassenbericht als Ausgabe ein.`),
    h('div', { class: 'field' }, h('label', null, 'Gutschrift über'), h('div', { class: 'inputunit' }, betrag, h('span', null, '€'))),
    h('div', { class: 'field' }, h('label', null, 'Datum'), datum),
    h('div', { class: 'field wide' }, h('label', null, 'Grund (erscheint auf dem Schreiben)'), grund));
  return { el: body, knopf: 'Gutschrift ausstellen', save: async () => {
      const v = parseNum(betrag.value);
      if (v === null || Number.isNaN(v) || v <= 0) { toast('Bitte einen Betrag über 0 angeben.', 'err'); return false; }
      if (v > rest + 0.004) { toast(`Höchstens ${eur(rest)} möglich.`, 'err'); return false; }
      if (!grund.value.trim()) { toast('Bitte den Grund angeben.', 'err'); return false; }
      try {
        const r = await api('POST', `/api/admin/rechnungen/${e.id}/gutschrift`, { betrag: v, datum: datum.value, grund: grund.value.trim() });
        await reload();
        toast(`Gutschrift ${r.nummer} ausgestellt.`, 'ok');
      } catch (er) { handleErr(er); return false; }
      return 'neu';
  } };
}

// Ratenplan: gleiche Rechnung, aber in Teilbeträgen. Die Zahlungen laufen wie immer über »Zahlung vermerken«.
function ratenFaellig(e) {
  const rs = e.ratenplan || [];
  if (!rs.length) return e.faellig;
  const gezahlt = payPaid(e);
  let summe = 0;
  for (const r of rs) { summe += r.betrag; if (summe > gezahlt + 0.005) return r.faellig; }
  return rs[rs.length - 1].faellig;
}
// »Rate 2 von 3« unter dem Fälligkeitsdatum, solange noch etwas offen ist.
function ratenHinweis(e) {
  const rs = e.ratenplan || [];
  if (!rs.length || !payIsOpen(e)) return null;
  const gezahlt = payPaid(e);
  let summe = 0, nr = rs.length;
  for (let i = 0; i < rs.length; i++) { summe += rs[i].betrag; if (summe > gezahlt + 0.005) { nr = i + 1; break; } }
  return h('div', { class: 'hint', title: 'Ratenplan' }, `Rate ${nr} von ${rs.length}`);
}
function monatPlus(iso, n) {
  const [j, m, t] = iso.split('-').map(Number);
  const d = new Date(j, m - 1 + n, 1);
  const letzter = new Date(d.getFullYear(), d.getMonth() + 1, 0).getDate();
  d.setDate(Math.min(t, letzter));
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}
function ratenPanel(e, ergebnis, schliessen) {
  const total = Math.round(payTotal(e) * 100) / 100;
  let raten = (e.ratenplan || []).map((r) => ({ ...r }));
  const anzahl = h('select', null, ...[2, 3, 4, 5, 6, 8, 10, 12].map((n) => h('option', { value: n, selected: n === (raten.length || 3) }, `${n} Raten`)));
  const start = h('input', { type: 'date', value: raten.length ? raten[0].faellig : isoInTagen(30) });
  const liste = h('div', { class: 'form' });
  const summe = h('p', { class: 'hint' });
  const pruefe = () => {
    const s = Math.round(raten.reduce((a, r) => a + (Number.isFinite(r.betrag) ? r.betrag : 0), 0) * 100) / 100;
    summe.textContent = Math.abs(s - total) < 0.005 ? `Zusammen ${eur(s)}. Das passt zur Rechnung.` : `Zusammen ${eur(s)}, die Rechnung lautet aber auf ${eur(total)}. Bitte anpassen.`;
    summe.className = Math.abs(s - total) < 0.005 ? 'hint' : 'hint err';
  };
  const zeichne = () => {
    liste.replaceChildren(...raten.map((r, i) => {
      const d = h('input', { type: 'date', value: r.faellig, onchange: (ev) => { r.faellig = ev.target.value; } });
      const b = h('input', { type: 'text', inputmode: 'decimal', value: numIn(r.betrag), style: 'width:110px', oninput: (ev) => { const v = parseNum(ev.target.value); r.betrag = v === null || Number.isNaN(v) ? NaN : v; pruefe(); } });
      return h('div', { class: 'field' }, h('label', null, `Rate ${i + 1}`), h('div', { class: 'inputunit' }, d, b, h('span', null, '€')));
    }));
    pruefe();
  };
  const erzeuge = () => {
    const n = Number(anzahl.value);
    if (!start.value) { toast('Bitte das Datum der ersten Rate angeben.', 'err'); return; }
    const cent = Math.round(total * 100), basis = Math.floor(cent / n);
    raten = Array.from({ length: n }, (_, i) => ({ faellig: monatPlus(start.value, i), betrag: (i === n - 1 ? cent - basis * (n - 1) : basis) / 100 }));
    zeichne();
  };
  anzahl.onchange = erzeuge; start.onchange = erzeuge;
  if (raten.length) zeichne(); else erzeuge();
  const body = h('div', { class: 'form' },
    h('p', null, `${e.mitgliedsnr} – ${e.name}: Rechnungsbetrag ${eur(total)}. Die Raten sind je einen Monat auseinander. Du kannst jedes Datum und jeden Betrag ändern.`),
    h('div', { class: 'field' }, h('label', null, 'Wie viele Raten?'), anzahl),
    h('div', { class: 'field' }, h('label', null, 'Erste Rate am'), start),
    liste, summe,
    h('p', { class: 'hint' }, 'Gezahlt wird wie immer unter »Zahlung vermerken« (Betrag = alles bisher Gezahlte). Das Programm erinnert erst, wenn eine Rate fehlt.'));
  if ((e.ratenplan || []).length) body.append(h('div', { class: 'toolbar' }, h('button', { class: 'btn danger small', onclick: async () => {
    try { await api('PUT', `/api/admin/rechnungen/${e.id}/ratenplan`, { raten: [] }); e.ratenplan = null; toast('Ratenplan gelöscht', 'ok'); await reload(); ergebnis.wert = 'neu'; schliessen(); } catch (er) { handleErr(er); }
  } }, 'Ratenplan löschen')));
  return { el: body, knopf: 'Ratenplan speichern', save: async () => {
      if (raten.some((r) => !r.faellig || !Number.isFinite(r.betrag) || r.betrag <= 0)) { toast('Bitte bei jeder Rate Datum und Betrag angeben.', 'err'); return false; }
      try { await api('PUT', `/api/admin/rechnungen/${e.id}/ratenplan`, { raten }); e.ratenplan = raten; e.faellig = ratenFaellig(e); } catch (er) { handleErr(er); return false; }
      await reload();
      return 'neu';
  } };
}

function zahlungsPanel(e, ergebnis, schliessen) {
  const date = h('input', { type: 'date', value: e.bezahltAm || todayIso() });
  const amt = h('input', { type: 'text', inputmode: 'decimal', value: numIn(e.bezahltAm ? payPaid(e) : payTotal(e)), style: 'width:120px' });
  const note = h('input', { type: 'text', maxlength: 200, value: e.notiz || '', style: 'width:100%' });
  const body = h('div', { class: 'form' },
    h('p', null, `${e.mitgliedsnr} – ${e.name}: Rechnungsbetrag ${eur(payTotal(e))}${e.gesamt < 0 ? ' (Guthaben, wird an den Pächter ausgezahlt)' : ''}`),
    gutschriftListe(e),
    (e.ratenplan || []).length ? h('p', { class: 'hint' }, `Ratenplan: ${e.ratenplan.map((r) => `${eur(r.betrag)} bis ${deDate(r.faellig)}`).join(', ')}. Als Betrag unten bitte alles bisher Gezahlte eintragen.`) : null,
    h('div', { class: 'field' }, h('label', null, 'Bezahlt am'), date),
    h('div', { class: 'field' }, h('label', null, 'Bezahlter Betrag (bei Teilzahlung anpassen)'), h('div', { class: 'inputunit' }, amt, h('span', null, '€'))),
    h('div', { class: 'field wide' }, h('label', null, 'Notiz (z. B. bar, Überweisung, Ratenzahlung)'), note),
    e.bezahltAm ? h('div', { class: 'toolbar' }, h('button', { class: 'btn danger small', onclick: async () => {
      try { await savePayment(e, { bezahltAm: '', bezahltBetrag: null }); ergebnis.wert = 'undo'; schliessen(); } catch (er) { handleErr(er); }
    } }, 'Zahlung zurücknehmen')) : null);
  return { el: body, knopf: 'Speichern', save: async () => {
    const v = parseNum(amt.value);
    if (!date.value) { toast('Bitte ein Datum angeben.', 'err'); return false; }
    if (v === null || Number.isNaN(v) || v < 0) { toast('Bitte einen gültigen Betrag angeben.', 'err'); return false; }
    try { await savePayment(e, { bezahltAm: date.value, bezahltBetrag: v, notiz: note.value.trim() }); } catch (er) { handleErr(er); return false; }
    return true;
  } };
}

// Ein Dialog für alles rund um eine Rechnung: Zahlung, Ratenplan, Gutschrift (Reiter). Liefert false (nichts
// geändert), true/'undo' (Zahlung geändert, Liste neu zeichnen) oder 'neu' (Seite wurde schon neu geladen).
async function paymentDialog(e, start) {
  const ergebnis = { wert: null };
  const kannMehr = e.gesamt > 0 && e.status === 'gueltig';
  const schliessen = () => { const k = document.querySelector('dialog[open] .dfoot .btn'); if (k) k.click(); };
  const panels = { zahlung: zahlungsPanel(e, ergebnis, schliessen) };
  if (kannMehr) {
    if (payIsOpen(e)) panels.raten = ratenPanel(e, ergebnis, schliessen);
    panels.gutschrift = gutschriftPanel(e);
  }
  let aktiv = panels[start] ? start : 'zahlung';
  const titel = { zahlung: 'Zahlung', raten: (e.ratenplan || []).length ? 'Ratenplan ändern' : 'Ratenplan', gutschrift: 'Gutschrift' };
  const kopf = h('div'), flaeche = h('div');
  const zeichne = () => {
    kopf.replaceChildren(Object.keys(panels).length > 1 ? reiterLeiste(Object.keys(panels).map((k) => ({ label: titel[k], aktiv: k === aktiv, geh: () => { aktiv = k; zeichne(); } })), 'Rechnung') : h('span'));
    flaeche.replaceChildren(panels[aktiv].el);
    const pk = document.querySelector('dialog[open] .dfoot .btn.primary');
    if (pk) pk.textContent = panels[aktiv].knopf;
  };
  zeichne();
  const versprechen = modal(`Rechnung ${e.nummer}`, h('div', null, kopf, flaeche), [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const r = await panels[aktiv].save();
      if (r === false) return false;
      ergebnis.wert = r;
    } },
  ]);
  if (aktiv !== 'zahlung') zeichne(); // Knopfbeschriftung des gewählten Reiters
  const erg = await versprechen;
  return ergebnis.wert || erg;
}

const MAHN_TITEL = ['', 'Zahlungserinnerung', 'Mahnung', 'Letzte Mahnung'];

// Mahn-Vorschlag: wer ist überfällig, welche Stufe wäre dran, und ist die letzte Frist schon abgelaufen?
// Nichts passiert von selbst – der Vorstand wählt aus und erzeugt die Briefe.
async function mahnVorschlag(ueberfaellig) {
  const heute = todayIso();
  const zeilen = ueberfaellig.map((e) => {
    const m = e.mahnungen || [];
    const letzte = m.length ? m[m.length - 1] : null;
    const wartet = letzte && letzte.frist && letzte.frist >= heute;
    return { e, stufe: Math.min(m.length + 1, 3), wartet, letzte, max: m.length >= 3 };
  }).sort((a, b) => a.e.mitgliedsnr.localeCompare(b.e.mitgliedsnr, 'de', { numeric: true }));
  const dran = zeilen.filter((z) => !z.wartet);
  const haken = new Map(dran.map((z) => [z.e.id, h('input', { type: 'checkbox', checked: true })]));
  const info = (z) => z.wartet ? `Frist läuft noch bis ${deDate(z.letzte.frist)} – noch warten`
    : z.max ? 'Schon dreimal gemahnt – nächster Schritt liegt beim Vorstand (z. B. Kündigung prüfen)'
      : `${MAHN_TITEL[z.stufe]} wäre dran${z.letzte ? ` (letzte Mahnung am ${deDate(z.letzte.datum)})` : ''}`;
  const ok = await modal('Mahn-Vorschlag', h('div', null,
    h('p', { class: 'hint' }, 'So sieht es aus. Wer noch eine laufende Frist hat, wird nicht vorgeschlagen. Nimm weg, wen du nicht anschreiben willst.'),
    h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['', 'Pächter', 'Offen', 'Fällig seit', 'Vorschlag'].map((t) => h('th', null, t)))),
      h('tbody', null, zeilen.map((z) => h('tr', null, h('td', null, haken.has(z.e.id) ? haken.get(z.e.id) : null),
        h('td', { class: 'wrap' }, `${z.e.mitgliedsnr} ${z.e.name}`), h('td', { class: 'r' }, eur(payOpen(z.e))),
        h('td', null, deDate(z.e.faellig)), h('td', { class: 'wrap' }, info(z)))))))),
  [{ label: 'Abbrechen', value: false }, { label: 'Weiter zu den Briefen', cls: 'primary', value: true }]);
  if (!ok) return;
  const ids = dran.filter((z) => haken.get(z.e.id).checked).map((z) => z.e.id);
  if (!ids.length) { toast('Niemand ausgewählt.', 'err'); return; }
  await mahnungErzeugen(ids);
}
async function mahnungErzeugen(ids) {
  if (!ids.length) { toast('Bitte mindestens eine offene Rechnung auswählen.', 'err'); return; }
  const archiv = new Map((S.state.archive || []).map((e) => [e.id, e]));
  const stufen = [0, 0, 0, 0];
  for (const id of ids) { const e = archiv.get(id); stufen[Math.min(((e && e.mahnungen) ? e.mahnungen.length : 0) + 1, 3)]++; }
  const frist = h('input', { type: 'date', value: isoInTagen(14) });
  const vermerken = h('input', { type: 'checkbox', checked: true });
  const text = [1, 2, 3].filter((i) => stufen[i]).map((i) => `${stufen[i]} × ${MAHN_TITEL[i]}`).join(', ');
  if (!(await modal('Mahnung erzeugen', h('div', { class: 'form' },
    h('p', { class: 'hint' }, `Es entstehen: ${text}. Die Stufe richtet sich danach, wie oft die Rechnung schon gemahnt wurde.`),
    h('div', { class: 'field' }, h('label', null, 'Zahlen bis'), frist),
    h('label', { class: 'inline' }, vermerken, ' Als verschickt vermerken (nächste Stufe, Erinnerung im Cockpit)')),
  [{ label: 'Abbrechen', value: false }, { label: 'Erzeugen', cls: 'primary', value: true }]))) return;
  try {
    const r = await fetch('/api/admin/mahnung', { method: 'POST', headers: { 'X-GA-Request': '1', 'Content-Type': 'application/json' }, body: JSON.stringify({ ids, frist: frist.value, vermerken: vermerken.checked }) });
    if (!r.ok) { const d = await r.json().catch(() => null); throw new Error((d && d.error) || `Fehler ${r.status}`); }
    const blob = await r.blob();
    const url = URL.createObjectURL(blob);
    if ((r.headers.get('content-type') || '').includes('pdf')) openDocument(url, 'Mahnung');
    else { const a = h('a', { href: url, download: 'Mahnungen.zip' }); document.body.append(a); a.click(); a.remove(); }
    setTimeout(() => URL.revokeObjectURL(url), 30000);
    if (vermerken.checked) { S.mahnSel.clear(); await reload(); }
  } catch (e) { handleErr(e); }
}

function viewZahlungen() {
  const st = S.state;
  const all = (st.archive || []).filter((e) => e.status === 'gueltig');
  const today = todayIso();
  const summary = h('div', { class: 'cards' });
  const tbody = h('tbody');
  const filters = h('div', { class: 'subtabs' });
  const mahnBar = h('div', { class: 'toolbar' });
  S.mahnSel = S.mahnSel instanceof Set ? S.mahnSel : new Set();

  const draw = () => {
    const sum = (arr, f) => arr.reduce((a, e) => a + f(e), 0);
    const claims = all.filter((e) => e.gesamt >= 0), credits = all.filter((e) => e.gesamt < 0);
    const openClaims = claims.filter(payIsOpen), overdue = openClaims.filter((e) => e.faellig && e.faellig < today);
    const card = (t, big, small, cls, ic) => h('div', { class: 'card stat ' + (cls || '') }, ic ? h('div', { class: 'statico' }, icon(ic, 22)) : null,
    h('div', null, h('div', { class: 'hint' }, t), h('div', { class: 'big' }, big), h('div', { class: 'hint' }, small)));
    summary.replaceChildren(...[
      card('Forderungen gesamt', eur(sum(claims, payTotal)), `${claims.length} Rechnungen`),
      card('Bereits bezahlt', eur(sum(claims, payPaid)), `${claims.filter((e) => !payIsOpen(e)).length} vollständig bezahlt`, 'ok'),
      card('Noch offen', eur(sum(openClaims, payOpen)), `${openClaims.length} Rechnungen`, openClaims.length ? 'warn' : 'ok'),
      card('Überfällig', String(overdue.length), overdue.length ? eur(sum(overdue, payOpen)) : 'nichts überfällig', overdue.length ? 'err' : 'ok'),
      credits.length ? card('Guthaben auszuzahlen', eur(sum(credits.filter(payIsOpen), payOpen)), `${credits.filter(payIsOpen).length} von ${credits.length} noch offen`) : null].filter(Boolean));

    const ueberfaellig = overdue.filter((e) => e.gesamt >= 0);
    const alleGewaehlt = ueberfaellig.length > 0 && ueberfaellig.every((e) => S.mahnSel.has(e.id));
    mahnBar.replaceChildren(
      h('button', { class: 'btn', disabled: !S.mahnSel.size, onclick: () => mahnungErzeugen([...S.mahnSel]) },
        `Mahnung erzeugen${S.mahnSel.size ? ` (${S.mahnSel.size})` : ''}`),
      h('button', { class: 'btn', disabled: !ueberfaellig.length, title: 'Setzt den Haken bei allen Rechnungen, die über das Zahlungsziel hinaus offen sind',
        onclick: () => { for (const e of ueberfaellig) { if (alleGewaehlt) S.mahnSel.delete(e.id); else S.mahnSel.add(e.id); } draw(); } },
      alleGewaehlt ? 'Auswahl aufheben' : `Alle überfälligen auswählen${ueberfaellig.length ? ` (${ueberfaellig.length})` : ''}`),
      h('button', { class: 'btn', disabled: !ueberfaellig.length, title: 'Das Programm schlägt vor, wen du jetzt erinnern oder mahnen solltest – du entscheidest',
        onclick: () => mahnVorschlag(ueberfaellig) }, 'Mahn-Vorschlag'),
      h('span', { class: 'hint' }, 'Haken bei den gewünschten offenen Rechnungen setzen, dann »Mahnung erzeugen«. Es wird nie automatisch gemahnt.'));

    const nOpen = all.filter(payIsOpen).length;
    filters.replaceChildren(
      ...[['offen', `Noch offen (${nOpen})`], ['bezahlt', `Bezahlt (${all.length - nOpen})`], ['alle', `Alle (${all.length})`]].map(([k, t]) =>
        h('button', { class: S.payFilter === k ? 'active' : '', onclick: () => { S.payFilter = k; draw(); } }, t)));

    const q = S.payQ.trim().toLowerCase();
    const shown = all.filter((e) => (S.payFilter === 'alle' || (S.payFilter === 'offen') === payIsOpen(e)) &&
      (!q || [e.mitgliedsnr, e.name, e.nummer].some((x) => (x || '').toLowerCase().includes(q))))
      .sort((a, b) => a.mitgliedsnr.localeCompare(b.mitgliedsnr, 'de', { numeric: true }));
    tbody.replaceChildren();
    for (const e of shown) {
      const open = payIsOpen(e);
      const late = open && e.gesamt >= 0 && e.faellig && e.faellig < today;
      const partial = e.bezahltAm && open;
      const chk = h('input', { type: 'checkbox', checked: !!e.bezahltAm && !open, title: 'als bezahlt markieren',
        onchange: async (ev) => {
          try {
            if (ev.target.checked) await savePayment(e, { bezahltAm: todayIso(), bezahltBetrag: null });
            else await savePayment(e, { bezahltAm: '', bezahltBetrag: null });
            draw();
          } catch (er) { ev.target.checked = !ev.target.checked; handleErr(er); }
        } });
      const date = h('input', { type: 'date', value: e.bezahltAm || '', disabled: !e.bezahltAm, title: 'Zahldatum',
        onchange: async (ev) => {
          if (!ev.target.value) { ev.target.value = e.bezahltAm; return; }
          try { await savePayment(e, { bezahltAm: ev.target.value }); draw(); } catch (er) { handleErr(er); }
        } });
      // Teilzahlung direkt in der Zeile: Betrag eintragen, Enter – ohne den Details-Dialog
      const teil = open && e.gesamt >= 0 ? h('input', { type: 'text', inputmode: 'decimal', class: 'teilbetrag', autocomplete: 'off',
        placeholder: partial ? `bisher ${numIn(payPaid(e))}` : 'Teilbetrag €', title: 'Nur ein Teil wurde gezahlt? Betrag eintragen und Enter drücken. Der Rest bleibt offen.',
        'aria-label': `Teilbetrag für ${e.name}`,
        onkeydown: (ev) => { if (ev.key === 'Enter') { ev.preventDefault(); ev.target.blur(); } },
        onchange: async (ev) => {
          if (!ev.target.value.trim()) return;
          const v = parseNum(ev.target.value);
          if (v == null || Number.isNaN(v) || v <= 0) { toast('Bitte einen Betrag größer 0 eintragen, z. B. 50,00.', 'err'); ev.target.classList.add('err'); ev.target.focus(); return; }
          try {
            await savePayment(e, { bezahltAm: e.bezahltAm || todayIso(), bezahltBetrag: v });
            draw();
          } catch (er) { handleErr(er); }
        } }) : null;
      const mahnbar = open && e.gesamt >= 0;
      const mchk = h('input', { type: 'checkbox', checked: S.mahnSel.has(e.id), disabled: !mahnbar, title: mahnbar ? 'für Mahnung auswählen' : 'nicht offen oder ein Guthaben',
        onchange: (ev) => { if (ev.target.checked) S.mahnSel.add(e.id); else S.mahnSel.delete(e.id); draw(); } });
      tbody.append(h('tr', { class: late ? 'late' : '' },
        h('td', null, mchk),
        h('td', null, e.mitgliedsnr), h('td', null, e.name), h('td', { class: 'mono' }, e.nummer),
        h('td', { class: 'r' }, (e.gesamt < 0 ? '−' : '') + eur(payTotal(e)),
          e.gutschriften && e.gutschriften.length ? h('div', { class: 'hint', title: e.gutschriften.map((g) => g.nummer).join(', ') }, `nach Gutschrift (${eur(Math.abs(e.gesamt))})`) : null),
        h('td', null, deDate(e.faellig), ratenHinweis(e)),
        h('td', null, h('span', { class: 'badge ' + (partial || (e.gutschriften && e.gutschriften.length && payOpen(e) < -0.004) ? 'warn' : !open ? 'ok' : late ? 'err' : 'neu') },
          partial ? `Teilzahlung, offen ${eur(payOpen(e))}` : !open && e.gesamt >= 0 && payOpen(e) < -0.004 && e.gutschriften && e.gutschriften.length ? `zu erstatten ${eur(-payOpen(e))}` : !open ? (e.gesamt < 0 ? 'ausgezahlt' : 'bezahlt') : late ? 'überfällig' : e.gesamt < 0 ? 'Guthaben offen' : 'offen'),
          open && e.mahnungen && e.mahnungen.length ? h('div', { class: 'hint', title: 'Letzte Mahnung' }, `${MAHN_TITEL[e.mahnungen[e.mahnungen.length - 1].stufe]} ${deDate(e.mahnungen[e.mahnungen.length - 1].datum)}, Frist ${deDate(e.mahnungen[e.mahnungen.length - 1].frist)}`) : null),
        h('td', { class: 'paycell' }, chk, date, teil),
        h('td', { class: 'note', title: e.notiz || '' }, e.notiz || ''),
        h('td', null, h('button', { class: 'btn small', onclick: async () => { const r = await paymentDialog(e); if (r === 'neu') return; if (r !== false && r !== undefined) draw(); } }, 'Details'))));
    }
    if (!shown.length) tbody.append(h('tr', null, h('td', { colspan: 10, class: 'empty' },
      !all.length ? leer('euro', 'Noch nichts zu kassieren', `Für ${st.year} wurde noch keine Rechnung ausgestellt. Sobald das passiert ist, hakst du hier die Zahlungen ab.`, 'Zu den Rechnungen', () => { S.tab = 'rechnungen'; render(); })
        : S.payFilter === 'offen' && !q ? leer('fertig', 'Alles bezahlt', 'Für dieses Jahr ist nichts mehr offen.') : 'Keine Treffer.')));
  };
  draw();

  const notIssued = st.paechter.length - Object.keys(st.issued || {}).length;
  return h('div', null,
    h('h2', null, `Zahlungen ${st.year}`, ' ', seitenHilfe(S.tab === 'zahlungen' ? 'zahlungen' : S.adminTab)),
    reiterKopf(),
    h('p', { class: 'hint' }, 'Hier siehst du, welche Rechnungen bezahlt sind und welche noch offen. Haken setzen = bezahlt heute (das Datum kannst du ändern). Über »Details« trägst du Teilzahlungen oder eine Notiz ein. Rechnungen mit Guthaben erscheinen hier auch, damit du siehst, was noch an Pächter auszuzahlen ist.'),
    notIssued > 0 ? h('div', { class: 'banner info' }, `${notIssued} Pächter ${notIssued === 1 ? 'hat' : 'haben'} in ${st.year} noch keine ausgestellte Rechnung und ${notIssued === 1 ? 'taucht' : 'tauchen'} deshalb hier noch nicht auf.`) : null,
    anderenJahrenOffen(st),
    summary,
    h('div', { class: 'toolbar' },
      h('input', { type: 'search', placeholder: 'Suchen (Nummer oder Name)', value: S.payQ, style: 'width:240px', oninput: (e) => { S.payQ = e.target.value; draw(); } }),
      !st.hasPassword || st.loggedIn ? h('button', { class: 'btn primary', title: 'CSV-Datei aus dem Online-Banking: Eingänge werden den Rechnungen zugeordnet', onclick: kontoauszugEinlesen }, 'Kontoauszug einlesen') : null,
      h('div', { class: 'spacer' }),
      mehrMenue([
        { label: 'Offene Posten als Excel', aktion: () => { location.href = `/api/export-payments?year=${st.year}&nur=offen`; } },
        { label: 'Alle Zahlungen als Excel', aktion: () => { location.href = `/api/export-payments?year=${st.year}`; } },
        { label: 'Fälligkeiten als Kalenderdatei', title: 'Fälligkeitstermine der offenen Rechnungen zum Import in den eigenen Kalender', aktion: () => { location.href = `/api/export-ics?year=${st.year}`; } }], 'Exportieren ▾')),
    filters,
    mahnBar,
    h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['', 'Nr.', 'Name', 'Rechnungs-Nr.', 'Betrag', 'Fällig am', 'Status', 'Bezahlt / am', 'Notiz', ''].map((t) => h('th', null, t)))),
      tbody)));
}

// ------------------------------------------------------------------ Spenden und Zuwendungsbestätigungen

async function loadSpenden() {
  if (S.spendenLoading) return;
  S.spendenLoading = true;
  try { S.spenden = await api('GET', '/api/admin/spenden'); } catch (e) { handleErr(e); S.spenden = { spenden: [], gemeinnuetzigkeit: {} }; }
  S.spendenLoading = false;
  render();
}

// Mehrere Spenden nacheinander erfassen (»Speichern und nächste«); liefert true, wenn etwas gespeichert wurde.
async function spendeDialog(x) {
  let vorlage = null, r, gespeichert = false;
  do {
    r = await spendeDialogEinmal(x, vorlage);
    if (r) gespeichert = true;
    vorlage = r === 'weiter' ? { datum: spendeDialogEinmal.letzt } : null;
  } while (r === 'weiter');
  return gespeichert;
}

function spendeDialogEinmal(x, vorlage) {
  const st = S.state;
  const isNew = !x || !x.id;
  const cur = { datum: (vorlage && vorlage.datum) || todayIso(), betrag: '', name: '', strasse: '', plzOrt: '', paechterId: '', verzicht: false, notiz: '', ...(x || {}) };
  const gesperrt = !!cur.bestaetigtAm;
  const f = {
    paechter: h('select', { disabled: gesperrt }, [h('option', { value: '' }, '– kein Pächter / andere Person –'),
      ...st.paechter.filter((p) => !p.geloescht).map((p) => h('option', { value: p.id, selected: p.id === cur.paechterId }, `${p.gartennr ? 'Garten ' + p.gartennr + ' · ' : ''}${p.name}`))]),
    datum: h('input', { type: 'date', value: cur.datum, disabled: gesperrt }),
    betrag: h('input', { type: 'text', inputmode: 'decimal', value: cur.betrag === '' ? '' : numIn(cur.betrag), disabled: gesperrt, placeholder: '0,00' }),
    name: h('input', { type: 'text', value: cur.name, maxlength: 100, disabled: gesperrt }),
    strasse: h('input', { type: 'text', value: cur.strasse, maxlength: 100 }),
    plzOrt: h('input', { type: 'text', value: cur.plzOrt, maxlength: 80 }),
    verzicht: h('input', { type: 'checkbox', checked: !!cur.verzicht, disabled: gesperrt }),
    notiz: h('input', { type: 'text', value: cur.notiz || '', maxlength: 200 }),
  };
  f.paechter.addEventListener('change', () => {
    const p = st.paechter.find((q) => q.id === f.paechter.value);
    if (p) { f.name.value = p.name; f.strasse.value = p.strasse || ''; f.plzOrt.value = p.plzOrt || ''; }
  });
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const body = h('div', null,
    gesperrt ? h('div', { class: 'banner info' }, `Für diese Spende wurde am ${deDate(cur.bestaetigtAm)} eine Bestätigung ausgestellt. Betrag, Datum und Spender lassen sich deshalb nicht mehr ändern.`) : null,
    h('div', { class: 'form' },
      fld('Spender ist Pächter', f.paechter, 'übernimmt Name und Anschrift', true),
      fld('Tag der Zuwendung *', f.datum), fld('Betrag (€) *', f.betrag),
      fld('Name *', f.name, null, true), fld('Straße', f.strasse), fld('PLZ und Ort', f.plzOrt),
      h('div', { class: 'field wide' }, h('label', { class: 'check' }, f.verzicht, ' Verzicht auf Erstattung von Aufwendungen (Aufwandsspende)')),
      fld('Notiz', f.notiz, null, true)));
  const speichern = async () => {
      const betrag = parseNum(f.betrag.value);
      if (betrag == null || Number.isNaN(betrag) || betrag <= 0) return feldFehler(f.betrag, 'Bitte einen Betrag größer als 0 eintragen, z. B. 50,00.');
      if (!f.name.value.trim()) return feldFehler(f.name, 'Bitte den Namen des Spenders eintragen.');
      const daten = { ...cur, paechterId: f.paechter.value, datum: f.datum.value, betrag, name: f.name.value, strasse: f.strasse.value, plzOrt: f.plzOrt.value, verzicht: f.verzicht.checked, notiz: f.notiz.value };
      try {
        if (isNew) await api('POST', '/api/admin/spenden', daten); else await api('PUT', `/api/admin/spenden/${cur.id}`, daten);
        toast('Gespeichert', 'ok');
        spendeDialogEinmal.letzt = f.datum.value;
      } catch (e) { handleErr(e); return false; }
  };
  return modal(isNew ? 'Spende erfassen' : 'Spende bearbeiten', body, [
    { label: 'Abbrechen', value: false },
    isNew ? { label: 'Speichern und nächste', value: 'weiter', action: speichern } : null,
    { label: 'Speichern', cls: 'primary', value: true, action: speichern }].filter(Boolean));
}

async function bestaetigungAusstellen(liste) {
  const schon = liste.filter((x) => x.bestaetigtAm);
  if (schon.length && !(await confirmBox(`Für ${schon.length === 1 ? 'diese Spende' : schon.length + ' dieser Spenden'} wurde am ${deDate(schon[0].bestaetigtAm)} schon eine Bestätigung ausgestellt. Nur erneut ausstellen, wenn die erste nicht verwendet wird (z. B. verloren oder fehlerhaft).`, 'Erneut ausstellen'))) return;
  const ids = liste.map((x) => x.id);
  try { await api('POST', '/api/admin/spenden/bestaetigen', { ids }); } catch (e) { handleErr(e); return; }
  const q = ids.map(encodeURIComponent).join(',');
  await pdfDrucken({ url: `/api/admin/zuwendungsbestaetigung?ids=${q}`, oeffnenUrl: `/api/admin/zuwendungsbestaetigung/oeffnen?ids=${q}`,
    titel: 'Zuwendungsbestätigung', meldung: () => 'Die Bestätigung wurde im PDF-Programm geöffnet – ausdrucken, unterschreiben und ein Doppel aufbewahren.' });
  S.spenden = null; render();
}

// Alle offenen Spenden eines Jahres: ZIP mit einer Bestätigung je Spender, danach auf Wunsch als ausgestellt vermerken.
async function spendenJahrespaket(jahr) {
  let d;
  try { d = await api('GET', `/api/admin/spenden/jahr?jahr=${jahr}`); } catch (e) { handleErr(e); return; }
  if (!d.gruppen.length) { toast(`Für ${jahr} gibt es keine offenen Spenden mit Anschrift.`, 'err'); return; }
  const link = h('a', { class: 'btn primary', href: `/api/admin/spenden/jahrespaket?jahr=${jahr}`, download: '' }, 'ZIP herunterladen');
  const ok = await modal(`Zuwendungsbestätigungen ${jahr}`, h('div', null,
    h('p', null, `${d.gruppen.length} Spender mit noch nicht bestätigten Spenden – je Spender eine Einzel- oder Sammelbestätigung als PDF in einer ZIP-Datei.`),
    h('ul', null, d.gruppen.map((g) => h('li', null, `${g.name}${g.ids.length > 1 ? ` (${g.ids.length} Spenden, Sammelbestätigung)` : ''}`))),
    d.fehlt.length ? h('div', { class: 'banner warn' }, h('b', null, 'Nicht enthalten: '), d.fehlt.join(' · ')) : h('span'),
    h('p', null, link),
    h('p', { class: 'hint' }, 'Erst ausdrucken und unterschreiben. Danach hier vermerken, dass die Bestätigungen ausgestellt sind – sonst erscheinen die Spenden weiter als offen.')),
  [{ label: 'Schließen', value: false }, { label: 'Als ausgestellt vermerken', cls: 'primary', value: true }]);
  if (!ok) return;
  try {
    for (const g of d.gruppen) await api('POST', '/api/admin/spenden/bestaetigen', { ids: g.ids });
    toast(`${d.gruppen.length} Bestätigungen als ausgestellt vermerkt`, 'ok');
  } catch (e) { handleErr(e); }
  S.spenden = null; render();
}

function gemeinnuetzigkeitKarte(g, offen) {
  const f = {
    anschrift: h('textarea', { rows: 2, style: 'width:100%', placeholder: 'Straße Hausnummer\nPLZ Ort' }, g.anschrift || ''),
    zweck: h('input', { type: 'text', value: g.zweck || 'der Kleingärtnerei', maxlength: 200 }),
    finanzamt: h('input', { type: 'text', value: g.finanzamt || '', maxlength: 80, placeholder: 'z. B. Musterstadt' }),
    steuernummer: h('input', { type: 'text', value: g.steuernummer || '', maxlength: 30 }),
    bescheid: h('select', null, [['freistellung', 'Freistellungsbescheid (bzw. Anlage zum KSt-Bescheid)'], ['feststellung', 'Feststellungsbescheid nach § 60a AO (neu gegründet)']]
      .map(([v, t]) => h('option', { value: v, selected: (g.bescheid || 'freistellung') === v }, t))),
    datum: h('input', { type: 'date', value: g.bescheidDatum || '' }),
    veranlagung: h('input', { type: 'text', value: g.veranlagung || '', maxlength: 30, placeholder: 'z. B. 2021–2023' }),
  };
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const speichern = async () => {
    try {
      await api('PUT', '/api/admin/spenden/gemeinnuetzigkeit', { anschrift: f.anschrift.value, zweck: f.zweck.value, finanzamt: f.finanzamt.value, steuernummer: f.steuernummer.value,
        bescheid: f.bescheid.value, bescheidDatum: f.datum.value, veranlagung: f.veranlagung.value });
      toast('Gespeichert', 'ok'); S.spenden = null; render();
    } catch (e) { handleErr(e); }
  };
  return h('details', { class: 'card', open: offen },
    h('summary', null, h('b', null, 'Angaben zur Gemeinnützigkeit'), g.steuernummer ? h('span', { class: 'hint' }, ` · Finanzamt ${g.finanzamt}, StNr. ${g.steuernummer}`) : null),
    h('p', { class: 'hint', style: 'margin-top:10px' }, 'Steht im Bescheid des Finanzamts. Ohne diese Angaben lässt sich keine Zuwendungsbestätigung ausstellen.'),
    h('div', { class: 'form' },
      fld('Anschrift des Vereins', f.anschrift, null, true),
      fld('Begünstigter Zweck', f.zweck, '»Wir sind wegen Förderung … befreit« – meist »der Kleingärtnerei«', true),
      fld('Finanzamt', f.finanzamt), fld('Steuernummer', f.steuernummer),
      fld('Art des Bescheids', f.bescheid, null, true), fld('Datum des Bescheids', f.datum, 'Freistellung höchstens 5, Feststellung höchstens 3 Jahre alt'),
      fld('Letzter Veranlagungszeitraum', f.veranlagung, 'nur beim Freistellungsbescheid')),
    h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: speichern }, 'Speichern')));
}

function adminSpenden() {
  if (S.spenden == null) {
    if (!S.spendenLoading) loadSpenden();
    return h('p', { class: 'hint' }, 'Wird geladen …');
  }
  const d = S.spenden;
  const jahre = [...new Set([new Date().getFullYear(), ...d.spenden.map((x) => Number(x.datum.slice(0, 4)))])].sort((a, b) => b - a);
  const neuestes = d.spenden.length ? Math.max(...d.spenden.map((x) => Number(x.datum.slice(0, 4)))) : jahre[0];
  const jahr = S.spendenJahr && jahre.includes(S.spendenJahr) ? S.spendenJahr : neuestes;
  const l = d.spenden.filter((x) => x.datum.startsWith(jahr + '-'));
  S.spendenAuswahl = S.spendenAuswahl instanceof Set ? S.spendenAuswahl : new Set();
  for (const id of [...S.spendenAuswahl]) if (!l.some((x) => x.id === id)) S.spendenAuswahl.delete(id);
  const neu = async () => { if (await spendeDialog(null)) { S.spenden = null; render(); } };
  const loeschen = async (x) => {
    if (!(await confirmBox(`Spende von ${x.name} über ${eur(x.betrag)} löschen?`, 'Löschen', true))) return;
    try { await api('DELETE', `/api/admin/spenden/${x.id}`); S.spenden = null; render(); } catch (e) { handleErr(e); }
  };
  const auswahl = l.filter((x) => S.spendenAuswahl.has(x.id));
  const summe = l.reduce((a, x) => a + x.betrag, 0);
  const jahrSel = h('select', { onchange: (e) => { S.spendenJahr = Number(e.target.value); S.spendenAuswahl.clear(); render(); } },
    jahre.map((j) => h('option', { value: j, selected: j === jahr }, String(j))));
  return h('div', null,
    h('p', { class: 'hint' }, 'Geldspenden an den Verein erfassen und die Zuwendungsbestätigung nach dem amtlichen Muster ausstellen – einzeln oder als Sammelbestätigung für ein Jahr. Spenden zählen im Kassenbericht als Einnahme.'),
    d.hinweis ? h('div', { class: 'banner warn' }, d.hinweis) : null,
    gemeinnuetzigkeitKarte(d.gemeinnuetzigkeit || {}, !!d.hinweis),
    h('div', { class: 'toolbar', style: 'margin-top:14px' },
      h('button', { class: 'btn primary', onclick: neu }, '+ Spende erfassen'),
      h('label', null, 'Jahr'), jahrSel,
      h('button', { class: 'btn', disabled: !auswahl.length || !!d.hinweis, title: 'Eine Spende: Einzelbestätigung; mehrere desselben Spenders: Sammelbestätigung', onclick: () => bestaetigungAusstellen(auswahl) },
        auswahl.length > 1 ? `Sammelbestätigung (${auswahl.length})` : 'Bestätigung für Auswahl'),
      h('button', { class: 'btn', disabled: !!d.hinweis || !l.some((x) => !x.bestaetigtAm && !x.bereinigt), title: 'Alle noch offenen Spenden des Jahres, je Spender eine Bestätigung', onclick: () => spendenJahrespaket(jahr) }, 'Alle offenen des Jahres …'),
      h('div', { class: 'spacer' }), h('span', { class: 'stat' }, `${anz(l.length, 'Spende', 'Spenden')} · ${eur(summe)}`)),
    l.length ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['', 'Datum', 'Spender', 'Betrag', 'Bestätigung', ''].map((x) => h('th', null, x)))),
      h('tbody', null, l.map((x) => h('tr', null,
        h('td', null, h('input', { type: 'checkbox', checked: S.spendenAuswahl.has(x.id), disabled: x.bereinigt, 'aria-label': 'auswählen',
          onchange: (e) => { if (e.target.checked) S.spendenAuswahl.add(x.id); else S.spendenAuswahl.delete(x.id); render(); } })),
        h('td', null, deDate(x.datum)),
        h('td', { class: 'wrap' }, h('b', null, x.name), h('div', { class: 'hint' }, [x.strasse, x.plzOrt].filter(Boolean).join(', ') || 'Anschrift fehlt'),
          x.verzicht ? h('span', { class: 'badge neu' }, 'Aufwandsspende') : null),
        h('td', { class: 'num' }, eur(x.betrag)),
        h('td', null, x.bestaetigtAm ? h('span', { class: 'badge ok' }, `ausgestellt ${deDate(x.bestaetigtAm)}`) : h('span', { class: 'badge warn' }, 'offen')),
        h('td', { class: 'aktionen' },
          h('button', { class: 'btn small', disabled: !!d.hinweis || x.bereinigt, onclick: () => bestaetigungAusstellen([x]) }, 'Bestätigung'), ' ',
          h('button', { class: 'btn small', disabled: x.bereinigt, onclick: async () => { if (await spendeDialog(x)) { S.spenden = null; render(); } } }, 'Bearbeiten'), ' ',
          mehrMenue([{ label: 'Löschen', gefahr: true, aktion: () => loeschen(x) }])))))))
      : h('div', { class: 'card' }, leer('stern', `Noch keine Spenden in ${jahr}`, 'Erfasse Spenden hier oder übernimm sie beim Einlesen des Kontoauszugs. Daraus entsteht die Zuwendungsbestätigung für das Finanzamt.', '+ Spende erfassen', neu)),
    h('p', { class: 'hint', style: 'margin-top:14px' }, 'Gut zu wissen: Mitgliedsbeiträge an Kleingartenvereine sind steuerlich nicht abziehbar, deshalb werden hier nur Spenden bestätigt. Bis 300 € je Spende genügt dem Spender meist der Kontoauszug als Nachweis. Ein Doppel jeder Bestätigung muss der Verein aufbewahren. Im Zweifel bitte mit dem Steuerberater klären.'));
}
