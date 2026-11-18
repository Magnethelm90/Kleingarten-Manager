'use strict';
/* Kleingarten-Manager – Oberfläche, Teil: Einführungsrundgang und Übersicht. Gemeinsamer Gültigkeitsbereich mit den anderen Dateien (siehe index.html für die Reihenfolge). */

// ------------------------------------------------------------------ Einführungsrundgang

// Jeder Schritt wechselt selbst zur passenden Stelle im Programm und markiert den
// zugehörigen Reiter, die Seite dahinter bleibt bedienbar.
const TOUR = [
  { titel: 'Willkommen im Kleingarten-Manager',
    text: 'In 5 kurzen Schritten siehst du, was wohin gehört. Die Leiste links führt dich von oben nach unten durch ein Abrechnungsjahr, und ich zeige dir jede Stelle direkt im Programm. Alles wird sofort gespeichert, und vor heiklen Schritten legt das Programm automatisch eine Sicherung an.',
    ziel: 'uebersicht', gehe: () => { S.tab = 'uebersicht'; } },
  { titel: '1. Einmalig einrichten',
    text: 'Unter »Pächter« legst du zuerst die Pächter an (einzeln oder per Excel/CSV-Import) und trägst bei »Preise & Einstellungen« Vereinsname, Bankverbindung, Preise und Rechnungsdatum ein. Das bleibt in allen Folgejahren erhalten.',
    ziel: 'admin', gehe: () => { adminTabWaehlen('paechter'); } },
  { titel: '2. Zählerstände eintragen',
    text: 'Das ist die Arbeit jedes Jahr: Nummer eintippen, Enter drücken, neue Zählerstände und Arbeitsstunden eingeben. Name, Größe und Vorjahresstände sind schon da, gespeichert wird automatisch.',
    ziel: 'eingabe', gehe: () => { S.tab = 'eingabe'; S.eingabeMode = 'schnell'; } },
  { titel: '3. Rechnungen ausstellen',
    text: 'Hier prüfst du jede Rechnung und stellst sie aus. Sie wird als PDF gespeichert und archiviert, spätere Änderungen verändern sie nicht mehr. »Alle offenen ausstellen« erledigt alle auf einmal.',
    ziel: 'rechnungen', gehe: () => { S.tab = 'rechnungen'; S.invTab = 'pruefen'; } },
  { titel: '4. Zahlungen verfolgen',
    text: 'Setze den Haken, sobald eine Rechnung bezahlt ist (Teilzahlungen gehen über »Details«). Für überfällige Rechnungen erzeugst du hier die Mahnung, es wird nie automatisch gemahnt.',
    ziel: 'zahlungen', gehe: () => { S.tab = 'zahlungen'; } },
  { titel: '5. Jahresabschluss & Sicherheit',
    text: 'Am Jahresende schließt »Jahreswechsel« das Jahr ab, mit Checkliste. Deine Daten werden automatisch gesichert; unter »Einstellungen → Passwort & Sicherheit« schützt du die Verwaltung mit einem Passwort. Das Suchfeld oben findet jeden Pächter, und diese Einführung erreichst du jederzeit über den runden »?«-Knopf oben rechts.',
    ziel: 'hilfe', gehe: () => { adminTabWaehlen('jahreswechsel'); } },
];

// Willkommen: fünf Bilder mit kurzem Text, danach auf Wunsch der Rundgang durchs Programm.
const GESCHICHTE = [
  { bild: 'garten', titel: 'Willkommen im Kleingarten-Manager', text: 'Das Vereinsbüro für euren Kleingartenverein: Abrechnung, Pächter, Kasse und alles drumherum. Alles liegt auf diesem Rechner – ohne Cloud, ohne Anmeldung.' },
  { bild: 'zaehler', titel: 'Zählerstände in Minuten', text: 'Gartennummer eintippen, Wasser und Strom eingeben, fertig. Vorjahresstände kennt das Programm schon, gerechnet wird automatisch – auch Pacht, Beiträge und Stunden.' },
  { bild: 'rechnung', titel: 'Rechnungen, die bezahlt werden', text: 'Jede Rechnung als PDF mit QR-Code für die Banking-App, per Post oder E-Mail. Zahlungen hakst du ab oder liest sie aus dem Kontoauszug ein, Mahnungen gibt es auf Knopfdruck.' },
  { bild: 'laube', titel: 'Der ganze Verein an einem Ort', text: 'Lageplan, Warteliste, Kündigungen, Termine, Rundschreiben, Beschlüsse und Verträge. Was gerade ansteht, sagt dir die Übersicht von selbst.' },
  { bild: 'liste', titel: 'Bereit?', text: 'Der kurze Rundgang zeigt dir in fünf Schritten, wo was ist. Oder du schaust dich selbst um – ausprobieren geht gefahrlos mit den Beispieldaten.' },
];

function geschichteZeigen() {
  if (document.querySelector('.geschichte')) return;
  S.tour = null; render();
  let i = 0;
  const dlg = h('dialog', { class: 'geschichte', 'aria-label': 'Willkommen' });
  const buehne = h('div', { class: 'gs-buehne' });
  const punkte = h('div', { class: 'gs-punkte' });
  const zurueck = h('button', { class: 'btn' }, 'Zurück');
  const weiter = h('button', { class: 'btn primary' });
  const selbst = h('button', { class: 'btn' }, 'Selbst umschauen');
  const ende = (rundgang) => { dlg.close(); dlg.remove(); if (rundgang) tourGo(1); else tourEnd(); };
  const zeigen = (neu, richtung) => {
    i = neu;
    const g = GESCHICHTE[i];
    const folie = h('div', { class: 'gs-folie' + (richtung ? ' rein-' + richtung : '') },
      h('div', { class: 'gs-bild' }, h('img', { src: `/bilder/${g.bild}.svg`, alt: '' })),
      h('div', { class: 'gs-text' }, h('span', { class: 'gs-schritt' }, `${i + 1} von ${GESCHICHTE.length}`), h('h2', null, g.titel), h('p', null, g.text)));
    const alt = buehne.firstChild;
    if (alt && richtung && !bewegungReduziert()) { alt.classList.add('raus-' + richtung); setTimeout(() => alt.remove(), 320); } else if (alt) alt.remove();
    buehne.append(folie);
    punkte.replaceChildren(...GESCHICHTE.map((_, k) => h('button', { class: k === i ? 'an' : '', 'aria-label': `Bild ${k + 1}`, onclick: () => k !== i && zeigen(k, k > i ? 'rechts' : 'links') })));
    zurueck.style.visibility = i ? 'visible' : 'hidden';
    const letzte = i === GESCHICHTE.length - 1;
    weiter.textContent = letzte ? 'Rundgang starten' : 'Weiter';
    selbst.style.display = letzte ? '' : 'none';
  };
  zurueck.addEventListener('click', () => i > 0 && zeigen(i - 1, 'links'));
  weiter.addEventListener('click', () => (i === GESCHICHTE.length - 1 ? ende(true) : zeigen(i + 1, 'rechts')));
  selbst.addEventListener('click', () => ende(false));
  dlg.addEventListener('cancel', (e) => { if (e.target !== dlg) return; e.preventDefault(); ende(false); });
  dlg.addEventListener('keydown', (e) => {
    if (e.key === 'ArrowRight' && i < GESCHICHTE.length - 1) zeigen(i + 1, 'rechts');
    if (e.key === 'ArrowLeft' && i > 0) zeigen(i - 1, 'links');
  });
  dlg.append(h('button', { class: 'gs-zu', 'aria-label': 'Überspringen', title: 'Überspringen (Esc)', onclick: () => ende(false) }, 'Überspringen ✕'),
    buehne, h('div', { class: 'gs-fuss' }, zurueck, punkte, h('div', { class: 'gs-knoepfe' }, selbst, weiter)));
  document.body.append(dlg);
  zeigen(0, null);
  dlg.showModal();
  weiter.focus();
}

async function tourGo(i) {
  S.tour = { i };
  const step = TOUR[i];
  if (step.gehe) step.gehe();
  if ((S.tab === 'admin' || S.tab === 'uebersicht') && S.year !== S.state.currentYear) S.year = S.state.currentYear;
  await reload();
}

function tourEnd() {
  S.tour = null;
  let ersterStart = false;
  if (S.state && !S.state.tutorialGesehen) {
    S.state.tutorialGesehen = true;
    ersterStart = true;
    api('POST', '/api/tutorial').catch(() => { /* beim nächsten Start erneut anbieten */ });
  }
  render();
  if (ersterStart && S.state.leer) setTimeout(einrichtungsAssistent, 150);
}

function tourCard() {
  if (!S.tour) return null;
  const i = S.tour.i, step = TOUR[i], last = i === TOUR.length - 1;
  return h('div', { class: 'tour', role: 'dialog', 'aria-label': 'Einführung' },
    h('button', { class: 'skip', onclick: tourEnd, title: 'Einführung beenden (Esc)' }, last ? 'Schließen ✕' : 'Überspringen ✕'),
    h('div', { class: 'tstep' }, i === 0 ? 'Einführung' : `Schritt ${i} von ${TOUR.length - 1}`),
    h('h3', null, step.titel),
    h('p', null, step.text),
    h('div', { class: 'tfoot' },
      h('div', { class: 'dots' }, TOUR.map((_, k) => h('i', { class: k === i ? 'on' : '' }))),
      i > 0 ? h('button', { class: 'btn small', onclick: () => tourGo(i - 1) }, 'Zurück') : null,
      h('button', { class: 'btn small primary', onclick: () => (last ? tourEnd() : tourGo(i + 1)) }, last ? 'Fertig' : 'Weiter')));
}

function tourMark() {
  document.querySelectorAll('.tour-hl').forEach((el) => el.classList.remove('tour-hl'));
  if (!S.tour) return;
  const el = document.querySelector(`[data-tour="${TOUR[S.tour.i].ziel}"]`);
  if (el) el.classList.add('tour-hl');
}

document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape' && S.tour && !document.querySelector('dialog[open]')) tourEnd();
});

// Globale Suche im Kopfbereich: springt von jeder Seite direkt zur
// Schnellansicht eines Pächters (Nummer, Gartennummer, Name oder Straße).
// Suche tolerant: Umlaute ausgeschrieben (Mueller = Müller), Wörter ab 5 Buchstaben dürfen einen Tippfehler haben.
const sucheNorm = (s) => (s || '').toLowerCase().replace(/ä/g, 'ae').replace(/ö/g, 'oe').replace(/ü/g, 'ue').replace(/ß/g, 'ss').replace(/[^\p{L}\p{N}\s]/gu, '');
function einFehler(a, b) {
  if (a.length < b.length) [a, b] = [b, a];
  if (a.length - b.length > 1) return false;
  let i = 0;
  while (i < b.length && a[i] === b[i]) i++;
  if (i === b.length) return true;
  if (a.length === b.length) return a.slice(i + 1) === b.slice(i + 1) || (a[i] === b[i + 1] && a[i + 1] === b[i] && a.slice(i + 2) === b.slice(i + 2));
  return a.slice(i + 1) === b.slice(i);
}
function sucheTrifft(felder, anfrage) {
  const heu = sucheNorm(felder.join(' '));
  const teile = heu.split(/\s+/);
  return sucheNorm(anfrage).split(/\s+/).filter(Boolean).every((w) => heu.includes(w) || (w.length >= 5 && teile.some((t) => einFehler(t, w))));
}

function globalSearchBox(st) {
  const list = st.paechter || [];
  const find = (q) => {
    q = q.trim();
    if (!q) return [];
    return list.filter((p) => sucheTrifft([p.mitgliedsnr, p.gartennr, p.name, p.strasse], q)).sort(cmpNr).slice(0, 8);
  };
  const results = h('div', { class: 'globalsearch-results' });
  const goTo = (p) => {
    S.tab = 'eingabe';
    S.eingabeMode = 'schnell';
    S.quickId = p.id;
    S.globalSearchQ = '';
    render();
  };
  const istAdmin = !st.hasPassword || st.loggedIn;
  let weitere = [], weitereFuer = '', timer = null;
  const gehZu = (t) => {
    S.globalSearchQ = '';
    if (t.garten) { gartenakteDialog(t.garten); render(); return; }
    zielOeffnen(t.ziel);
  };
  const draw = () => {
    results.replaceChildren();
    const m = find(S.globalSearchQ);
    if (!S.globalSearchQ.trim()) { results.classList.remove('open'); return; }
    results.classList.add('open');
    const treffer = weitereFuer === S.globalSearchQ.trim() ? weitere : [];
    if (!m.length && !treffer.length) { results.append(h('div', { class: 'gs-empty' }, 'Nichts gefunden.')); return; }
    for (const p of m) {
      // Hauptklick: Zählerstände; daneben die häufigsten Ziele für diesen Pächter
      const ziel = (fn) => (e) => { e.stopPropagation(); S.globalSearchQ = ''; fn(); };
      const knoepfe = [
        { t: 'Rechnung', fn: () => { S.tab = 'rechnungen'; S.invTab = 'pruefen'; S.invView = 'archiv'; S.selInvoice = p.id; render(); } },
        istAdmin ? { t: 'Zahlung', fn: () => { S.tab = 'zahlungen'; S.payQ = p.mitgliedsnr; S.payFilter = 'alle'; render(); } } : null,
        istAdmin && p.gartennr ? { t: 'Akte', fn: () => { gartenakteDialog(p.gartennr); render(); } } : null].filter(Boolean);
      results.append(h('div', { class: 'gs-zeile' },
        h('button', { class: 'gs-item', type: 'button', onclick: () => goTo(p) },
          h('b', null, p.gartennr || p.mitgliedsnr), ' ', p.name, h('span', { class: 'hint' }, p.mitgliedsnr)),
        h('span', { class: 'gs-aktionen' }, knoepfe.map((k) => h('button', { class: 'gs-chip', type: 'button', title: `${k.t} von ${p.name}`, onclick: ziel(k.fn) }, k.t)))));
    }
    for (const t of treffer)
      results.append(h('button', { class: 'gs-item', type: 'button', onclick: () => gehZu(t) },
        h('b', null, t.art), ' ', t.titel, t.text ? h('span', { class: 'hint' }, t.text) : null));
  };
  const weitereSuchen = () => {
    clearTimeout(timer);
    const q = S.globalSearchQ.trim();
    if (!istAdmin || q.length < 2) return;
    timer = setTimeout(async () => {
      try { weitere = await api('GET', `/api/admin/suche?q=${encodeURIComponent(q)}`); weitereFuer = q; if (S.globalSearchQ.trim() === q) draw(); } catch (e) { /* Suche ist nur eine Hilfe */ }
    }, 200);
  };
  const input = h('input', {
    type: 'search', placeholder: 'Suchen', title: 'Suchen: Pächter, Mängel, Beschlüsse, Verträge, Termine, Dokumente …', class: 'gs-input', autocomplete: 'off',
    value: S.globalSearchQ,
    oninput: (e) => { S.globalSearchQ = e.target.value; draw(); weitereSuchen(); },
    onkeydown: (e) => {
      if (e.key === 'Escape') { S.globalSearchQ = ''; input.value = ''; draw(); input.blur(); }
      if (e.key !== 'Enter') return;
      const m = find(S.globalSearchQ);
      if (m.length === 1) goTo(m[0]);
    },
    onblur: () => setTimeout(() => { results.classList.remove('open'); }, 150),
    onfocus: () => draw(),
  });
  draw();
  return h('div', { class: 'globalsearch' }, input, results);
}

// ---- Schriftgröße (Anzeige), nur in diesem Programm; wird im Browser-Speicher gemerkt
const ZOOM_STUFEN = [0.9, 1, 1.15, 1.3, 1.5];
function zoomLaden() {
  try { const v = Number(localStorage.getItem('kgm-zoom')); return ZOOM_STUFEN.includes(v) ? v : 1; } catch (e) { return 1; }
}
function zoomSetzen(v) {
  document.documentElement.style.zoom = v === 1 ? '' : String(v);
  try { localStorage.setItem('kgm-zoom', String(v)); } catch (e) { /* ohne Speicher nur für diese Sitzung */ }
}
function zoomKnoepfe() {
  const i = ZOOM_STUFEN.indexOf(zoomLaden());
  const knopf = (label, titel, ziel, aus) => h('button', { class: 'zoombtn', title: titel, disabled: aus, onclick: () => { zoomSetzen(ziel); render(); } }, label);
  return h('span', { class: 'zoom' }, 'Anzeige: ',
    knopf('A−', 'Schrift kleiner', ZOOM_STUFEN[Math.max(0, i - 1)], i <= 0), ' ',
    knopf('A', 'Normale Größe', 1, i === 1), ' ',
    knopf('A+', 'Schrift größer', ZOOM_STUFEN[Math.min(ZOOM_STUFEN.length - 1, i + 1)], i >= ZOOM_STUFEN.length - 1), ' ',
    h('button', { class: 'zoombtn' + (lesbarLaden() ? ' an' : ''), 'aria-pressed': String(lesbarLaden()),
      title: 'Kräftigere Schrift, stärkere Kontraste und größere Knöpfe', onclick: () => { lesbarSetzen(!lesbarLaden()); render(); } },
    lesbarLaden() ? '✓ Gut lesbar' : 'Gut lesbar'));
}
zoomSetzen(zoomLaden());

// »Gut lesbar«: kräftigere Schrift, mehr Kontrast, größere Bedienelemente (je Rechner gemerkt).
function lesbarLaden() {
  try { return localStorage.getItem('kgm-lesbar') === '1'; } catch (e) { return document.documentElement.classList.contains('gutlesbar'); }
}
function lesbarSetzen(an) {
  document.documentElement.classList.toggle('gutlesbar', an);
  try { localStorage.setItem('kgm-lesbar', an ? '1' : '0'); } catch (e) { /* nur für diese Sitzung */ }
}
lesbarSetzen(lesbarLaden());

// Erinnerung an offene Rechnungen des laufenden Jahres, direkt nach dem Öffnen sichtbar.
// Bleibt bis zum nächsten Programmstart ausgeblendet, sobald sie einmal weggeklickt wurde.
// Warnt, falls die jüngste Sicherung beim Programmstart nicht lesbar war
// (z. B. Festplattenfehler, abgebrochener Schreibvorgang). Sehr selten, aber
// sicherheitsrelevant genug, um auf jedem Tab zu erscheinen.
function sicherungBanner(st) {
  if (S.sicherungDismissed || !st.sicherungFehler) return null;
  return h('div', { class: 'banner err' },
    `${st.sicherungFehler}.`, ' ',
    h('button', { class: 'btn small', onclick: () => { S.sicherungDismissed = true; render(); } }, 'Ausblenden'));
}

// Offene Posten des laufenden Jahres plus aller Vorjahre (diese liefert der Server getrennt), damit nichts
// nach dem Jahreswechsel aus dem Blick gerät.
function offenePosten(st) {
  const jetzt = (st.archive || []).filter((e) => e.status === 'gueltig' && e.gesamt >= 0).filter(payIsOpen);
  const vor = (st.offeneVorjahre || []).filter(payIsOpen);
  return { jetzt, vor, alle: [...vor, ...jetzt] };
}

// Hinweis in der Zahlungsübersicht: in anderen Jahren sind noch Rechnungen offen (Sprung dorthin)
function anderenJahrenOffen(st) {
  const proJahr = {};
  for (const e of (st.offeneVorjahre || []).filter(payIsOpen)) proJahr[e.jahr] = (proJahr[e.jahr] || 0) + 1;
  const jahre = Object.keys(proJahr).map(Number).sort();
  if (!jahre.length) return null;
  return h('div', { class: 'banner warn' }, 'In anderen Jahren sind noch Rechnungen offen: ',
    jahre.map((j) => [h('button', { class: 'btn small', onclick: async () => { S.year = j; await reload(); } }, `${j} (${proJahr[j]})`), ' ']));
}

function openRechnungenBanner(st) {
  if (S.remindDismissed || !['rechnungen', 'eingabe'].includes(S.tab) || st.year !== st.currentYear) return null;
  const { jetzt, vor, alle } = offenePosten(st);
  if (!alle.length) return null;
  const today = todayIso();
  const overdue = alle.filter((e) => e.faellig && e.faellig < today);
  const vorTxt = vor.length ? ` (davon ${vor.length} aus früheren Jahren, ältestes ${vor[0].jahr})` : '';
  return h('div', { class: 'banner warn' },
    `${alle.length} Rechnung${alle.length === 1 ? '' : 'en'} noch offen${vorTxt}${overdue.length ? `, davon ${overdue.length} überfällig` : ''}.`, ' ',
    h('button', { class: 'btn small', onclick: async () => {
      S.tab = 'zahlungen';
      S.year = jetzt.length ? st.currentYear : vor[0].jahr;
      await reload();
    } }, 'Zu den Zahlungen'), ' ',
    h('button', { class: 'btn small', onclick: () => { S.remindDismissed = true; render(); } }, 'Ausblenden'));
}

async function quitApp() {
  if (!(await confirmBox('Soll das Programm beendet werden? Alle Eingaben sind bereits gespeichert.', 'Beenden'))) return;
  try { await api('POST', '/api/quit'); } catch (e) { /* Programm ist weg */ }
  document.getElementById('app').replaceChildren(
    h('div', { class: 'center card' }, h('h2', null, 'Programm beendet'),
      h('p', null, 'Das Fenster schließt sich gleich von selbst. Zum erneuten Starten den Kleingarten-Manager wieder öffnen.')));
}

// ------------------------------------------------------------------ Tab: Übersicht

async function loadDashKasse(year) {
  if (S.dashKasseLoading) return;
  S.dashKasseLoading = true;
  try { S.dashKasse = await api('GET', `/api/admin/kassenbericht?year=${year}`); } catch (e) { S.dashKasse = false; }
  S.dashKasseLoading = false;
  render();
}

async function loadDashJubilaeen() {
  if (S.dashJubilaeenLoading) return;
  S.dashJubilaeenLoading = true;
  try { S.dashJubilaeen = await api('GET', '/api/garten-historie'); } catch (e) { S.dashJubilaeen = false; }
  S.dashJubilaeenLoading = false;
  render();
}

// Hinweis auf der Übersicht, welche aktiven Mitglieder in diesem Kalenderjahr
// ein rundes Jubiläum (5, 10, 15, ...) haben – aus der Gartenhistorie.
function jubilaeumBanner(st, istAdmin) {
  if (S.jubilaeumDismissed || !istAdmin || !modulAn('jubilaeen')) return null;
  if (S.dashJubilaeen == null) { if (!S.dashJubilaeenLoading) loadDashJubilaeen(); return null; }
  if (!S.dashJubilaeen) return null;
  const seitByPaechter = {};
  for (const e of S.dashJubilaeen) {
    if (!seitByPaechter[e.paechterId] || e.seit < seitByPaechter[e.paechterId]) seitByPaechter[e.paechterId] = e.seit;
  }
  const jahr = new Date().getFullYear();
  const treffer = [];
  for (const p of st.paechter) {
    const seit = seitByPaechter[p.id];
    if (!seit || p.ausgeschiedenAm) continue;
    const jahre = jahr - Number(seit.slice(0, 4));
    if (jahre > 0 && jahre % 5 === 0) treffer.push(`${p.name} (${jahre} Jahre)`);
  }
  if (!treffer.length) return null;
  return h('div', { class: 'banner info' },
    `${anz(treffer.length, 'Mitglied hat', 'Mitglieder haben')} ${jahr} ein rundes Jubiläum: ${treffer.join(', ')}.`, ' ',
    h('button', { class: 'btn small', onclick: () => { adminTabWaehlen('jubilaeen'); render(); } }, 'Zu den Jubiläen'), ' ',
    h('button', { class: 'btn small', onclick: () => { S.jubilaeumDismissed = true; render(); } }, 'Ausblenden'));
}

// Beispieldaten: in einen leeren Datenbestand laden und mit einem Klick wieder entfernen.
async function beispieldatenLaden() {
  try {
    await api('POST', '/api/admin/beispieldaten');
    toast('Beispieldaten geladen. Schau dich in Ruhe um, du kannst nichts kaputt machen.', 'ok');
    S.dashKasse = null; S.dashJubilaeen = null;
    await reload();
  } catch (e) { handleErr(e); }
}

async function beispieldatenEntfernen() {
  const ok = await confirmBox('Alle Beispieldaten entfernen und mit einem leeren Programm beginnen? Auch alles, was du in der Zwischenzeit selbst eingegeben oder ausgestellt hast, wird entfernt. Ein Admin-Passwort und der zweite Sicherungsordner bleiben erhalten.', 'Beispieldaten entfernen', true);
  if (!ok) return;
  try {
    await api('DELETE', '/api/admin/beispieldaten');
    toast('Beispieldaten entfernt. Jetzt kannst du deinen Verein eintragen.', 'ok');
    S.dashKasse = null; S.dashJubilaeen = null; S.tab = 'uebersicht';
    await reload();
  } catch (e) { handleErr(e); }
}

// »3 Rechnungen« / »1 Rechnung« statt »3 Rechnung(en)«
function anz(n, eins, viele) { return `${n} ${n === 1 ? eins : viele}`; }

function demoBanner(st) {
  if (!st.demo) return null;
  const istAdmin = !st.hasPassword || st.loggedIn;
  return h('div', { class: 'banner info' },
    'Du siehst Beispieldaten: ein erfundener Verein mit erfundenen Namen und Zahlen zum Ausprobieren.',
    ' ', h('button', { class: 'btn small', onclick: probierDialog }, 'Probier es aus'),
    istAdmin ? [' ', h('button', { class: 'btn small', onclick: beispieldatenEntfernen }, 'Beispieldaten entfernen und eigene Daten anlegen')] : null);
}

// Drei kleine Aufgaben mit den Beispieldaten: jede führt direkt an die richtige Stelle.
async function probierDialog() {
  const AUFGABEN = [
    ['Einen Zählerstand eintragen', 'Wähle einen Pächter, gib Wasser und Strom ein. Gespeichert wird automatisch.', 'Zur Schnelleingabe', () => { S.tab = 'eingabe'; S.eingabeMode = 'schnell'; }],
    ['Eine Rechnung ansehen und ausstellen', 'Prüfe eine Rechnung samt Vorschau und stelle sie aus – mit Beispieldaten ganz ohne Folgen.', 'Zu den Rechnungen', () => { S.tab = 'rechnungen'; S.invTab = 'pruefen'; }],
    ['Eine Zahlung abhaken', 'Setze den Haken bei einer ausgestellten Rechnung; ein Teilbetrag geht direkt in der Zeile.', 'Zu den Zahlungen', () => { S.tab = 'zahlungen'; }],
  ];
  const wahl = await modal('Probier es aus', h('div', null,
    h('p', { class: 'hint' }, 'Drei kurze Aufgaben, um das Programm kennenzulernen. Die Beispieldaten kannst du jederzeit entfernen.'),
    h('ol', { class: 'probier' }, AUFGABEN.map((a) => h('li', null, h('strong', null, a[0]), h('div', { class: 'hint' }, a[1]))))),
    [{ label: 'Schließen' }, ...AUFGABEN.map((a, i) => ({ label: a[2], value: String(i), primary: i === 0 }))]);
  const n = Number(wahl);
  if (wahl !== undefined && wahl !== null && wahl !== '' && AUFGABEN[n]) { AUFGABEN[n][3](); await reload(); }
}

function beispieldatenKarte(st, istAdmin) {
  if (!st.leer || !istAdmin) return null;
  return h('div', { class: 'card', style: 'margin-bottom:14px' },
    h('h3', { style: 'margin-top:0' }, 'Wie möchtest du anfangen?'),
    h('p', { class: 'hint' }, 'Der Einrichtungs-Assistent fragt in vier kurzen Schritten Verein, Preise und die gebrauchten Bereiche ab und hilft beim Eintragen der Pächter. Oder lade erst einen erfundenen Verein mit 12 Pächtern zum Ausprobieren – die Beispieldaten lassen sich danach mit einem Klick wieder entfernen.'),
    h('div', { class: 'actions' },
      h('button', { class: 'btn primary', onclick: einrichtungsAssistent }, 'Programm einrichten'),
      h('button', { class: 'btn', onclick: beispieldatenLaden }, 'Mit Beispieldaten ausprobieren')));
}

// Erinnerung an die Datensicherung außer Haus: ohne zweiten Sicherungsordner (USB-Stick, Netzlaufwerk)
// liegen alle Sicherungen auf derselben Festplatte wie die Daten.
async function sicherungJetzt() {
  try {
    const z = await api('POST', '/api/admin/sicherung-jetzt');
    if (!z.eingerichtet) toast('Sicherung angelegt (nur auf diesem Rechner, kein zweiter Ordner eingerichtet).', 'ok');
    else if (z.fehler) toast(`${z.fehler}. Ist der USB-Stick eingesteckt bzw. das Netzlaufwerk verbunden?`, 'err');
    else toast('Sicherung angelegt und in den zweiten Sicherungsordner kopiert.', 'ok');
    await reload();
  } catch (e) { handleErr(e); }
}

// Wiederherstellungs-Probe: liest die jüngsten Sicherungen probeweise ein und zeigt, was dabei herauskam.
async function wiederherstellungsProbe() {
  let r;
  try { r = await api('POST', '/api/admin/wiederherstellungsprobe'); } catch (e) { handleErr(e); return; }
  const zeilen = r.zeilen.map((z) => h('div', { class: 'banner ' + (z.stufe === 'ok' ? 'info' : z.stufe), style: 'margin:6px 0' },
    h('b', null, z.titel), h('div', null, z.text)));
  await modal(r.ok ? 'Probe bestanden' : 'Probe: Handlungsbedarf',
    h('div', null, h('p', { class: 'hint' }, 'Es wurde nichts verändert. Das Programm hat die Sicherungen so eingelesen, wie es beim Wiederherstellen geschähe.'), ...zeilen),
    [{ label: 'Schließen', cls: 'primary', value: true }]);
  await reload();
}

async function wartungsCheck() {
  let r;
  try { r = await api('GET', '/api/admin/wartung'); } catch (e) { handleErr(e); return; }
  const zeilen = r.zeilen.map((z) => h('div', { class: 'banner ' + (z.stufe === 'ok' ? 'info' : z.stufe), style: 'margin:6px 0' },
    h('b', null, (z.stufe === 'ok' ? '✓ ' : z.stufe === 'warn' ? '! ' : '✗ ') + z.titel), h('div', null, z.text)));
  await modal(r.stufe === 'ok' ? 'Alles in Ordnung' : 'Wartungs-Check: Hinweise', h('div', null,
    h('p', { class: 'hint' }, 'Prüft den Rechner, auf dem das Programm läuft: Datenordner, Sicherungen, Zugriffsschutz. Es wird nichts verändert.'), ...zeilen),
  [{ label: 'Schließen', cls: 'primary', value: true }]);
}

// Abgleich zwischen zwei Rechnern: Datei erstellen (USB-Stick) und vom anderen Rechner einlesen. Es kommen nur neue Einträge dazu.
async function abgleichDialog() {
  const datei = h('input', { type: 'file', accept: '.json,application/json', style: 'display:none' });
  const ergebnisBox = h('div');
  datei.addEventListener('change', async () => {
    const f = datei.files[0];
    if (!f) return;
    ergebnisBox.replaceChildren(h('p', { class: 'hint' }, 'Ich schaue mir die Datei an …'));
    let inhalt;
    try { inhalt = JSON.parse(await f.text()); } catch (e) { ergebnisBox.replaceChildren(h('div', { class: 'banner err' }, 'Die Datei lässt sich nicht lesen. Ist es die Abgleich-Datei vom anderen Rechner?')); return; }
    let v;
    try { v = await api('POST', '/api/admin/abgleich', inhalt); } catch (e) { ergebnisBox.replaceChildren(h('div', { class: 'banner err' }, e.message)); return; }
    if (!v.neu) { ergebnisBox.replaceChildren(h('div', { class: 'banner info' }, 'Hier ist schon alles vorhanden – es gibt nichts Neues zu übernehmen.')); return; }
    ergebnisBox.replaceChildren(
      h('div', { class: 'banner info' }, `Neu in dieser Datei: ${anz(v.neu, 'Eintrag', 'Einträge')}. Vorhandenes bleibt unverändert.`),
      h('table', { class: 'tbl' }, h('thead', null, h('tr', null, h('th', null, 'Bereich'), h('th', { class: 'r' }, 'Neu'), h('th', { class: 'r' }, 'Schon vorhanden'))),
        h('tbody', null, v.zeilen.map((z) => h('tr', null, h('td', null, z.bereich), h('td', { class: 'r' }, h('b', null, String(z.neu))), h('td', { class: 'r' }, String(z.schonDa)))))),
      h('div', { class: 'actions', style: 'margin-top:10px' }, h('button', { class: 'btn primary', onclick: async (ev) => {
        ev.target.disabled = true;
        try {
          const e = await api('POST', '/api/admin/abgleich?anwenden=1', inhalt);
          ergebnisBox.replaceChildren(h('div', { class: 'banner info' }, `Fertig: ${anz(e.neu, 'neuer Eintrag', 'neue Einträge')} übernommen.`));
          toast('Abgleich übernommen', 'ok');
          await reload();
        } catch (er) { handleErr(er); ev.target.disabled = false; }
      } }, `${anz(v.neu, 'Eintrag', 'Einträge')} übernehmen`)));
  });
  await modal('Abgleich zwischen zwei Rechnern', h('div', null,
    h('p', { class: 'hint' }, 'Zwei Personen arbeiten auf zwei Rechnern, zum Beispiel Kassier und Schriftführer? Mit dem USB-Stick bringst du beide wieder zusammen. Es kommen nur neue Einträge dazu, nichts wird überschrieben oder gelöscht.'),
    h('ol', { class: 'probier' },
      h('li', null, h('b', null, 'Auf dem einen Rechner: '), 'Datei erstellen und auf den USB-Stick legen.', h('div', null, h('a', { class: 'btn small', href: '/api/admin/abgleich', download: '' }, 'Abgleich-Datei erstellen'))),
      h('li', null, h('b', null, 'Auf dem anderen Rechner: '), 'die Datei hier einlesen. Du siehst zuerst, was neu ist, und bestätigst dann.', h('div', null, h('button', { class: 'btn small', onclick: () => datei.click() }, 'Abgleich-Datei einlesen')))),
    h('p', { class: 'hint' }, 'Dabei sind Ausgaben, Spenden, Termine, Beschlüsse, Mängel, Einsatzstunden und Zählerstände. Belege, Fotos und das Bar-Kassenbuch nicht. Die Datei enthält Namen und Beträge – nur auf dem USB-Stick weitergeben und danach löschen.'),
    datei, ergebnisBox), [{ label: 'Schließen', cls: 'primary', value: true }]);
}

// Daten aufräumen: zeigt Dubletten und Lücken im Bestand, jeder Fund mit Knopf zum betroffenen Pächter.
async function datencheckDialog() {
  const lade = async () => {
    try { return await api('GET', '/api/admin/datencheck'); } catch (e) { handleErr(e); return null; }
  };
  let r = await lade();
  if (!r) return;
  for (;;) {
    const GRUPPEN = [['doppelt', 'Doppelt vorhanden'], ['luecke', 'Angaben fehlen'], ['iban', 'Bankverbindung'], ['garten', 'Gärten']];
    const inhalt = h('div', null,
      h('p', { class: 'hint' }, `Ich habe ${anz(r.geprueft, 'Pächter', 'Pächter')} angeschaut und suche nach Doppelten und fehlenden Angaben. Es wird nichts verändert – du entscheidest.`));
    if (!r.funde.length) inhalt.append(leer('fertig', 'Alles in Ordnung', 'Keine doppelten Einträge und keine fehlenden Angaben gefunden.'));
    let wahl = null;
    for (const [art, titel] of GRUPPEN) {
      const liste = r.funde.filter((f) => f.art === art);
      if (!liste.length) continue;
      inhalt.append(h('h4', null, `${titel} (${liste.length})`), ...liste.map((f) => h('div', { class: 'banner ' + (f.stufe === 'warn' ? 'warn' : 'info'), style: 'margin:6px 0' },
        h('div', null, f.text), h('div', { class: 'hint' }, f.tipp),
        ...(f.paechterIds || []).map((id) => {
          const p = (S.state.paechter || []).find((x) => x.id === id);
          return p ? h('button', { class: 'btn small', style: 'margin:4px 6px 0 0', onclick: (e) => { wahl = id; e.target.closest('dialog').querySelector('.dfoot .btn').click(); } }, `${p.name} öffnen`) : null;
        }).filter(Boolean))));
    }
    await modal('Daten aufräumen', inhalt, [{ label: 'Schließen', cls: 'primary', value: 'zu' }]);
    if (!wahl) return;
    const p = (S.state.paechter || []).find((x) => x.id === wahl);
    if (p && await paechterDialog(p)) await reload();
    r = await lade();
    if (!r) return;
  }
}

function zweiteSicherungBanner(st, istAdmin) {
  const z = st.zweiteSicherung || {};
  if (S.zweiteDismissed || !istAdmin || !st.paechter.length) return null;
  const knoepfe = (mitJetzt) => [' ',
    mitJetzt ? h('button', { class: 'btn small', onclick: sicherungJetzt }, 'Jetzt sichern') : null, mitJetzt ? ' ' : null,
    h('button', { class: 'btn small', onclick: () => { adminTabWaehlen('daten'); render(); } }, 'Zu den Einstellungen'), ' ',
    h('button', { class: 'btn small', onclick: () => { S.zweiteDismissed = true; render(); } }, 'Ausblenden')];
  if (!z.eingerichtet) {
    return h('div', { class: 'banner info' },
      'Alle Sicherungen liegen bisher nur auf diesem Rechner. Geht die Festplatte kaputt, sind auch sie weg. Richte einen zweiten Sicherungsordner ein, zum Beispiel auf einem USB-Stick.', ...knoepfe(false));
  }
  if (z.fehler) return h('div', { class: 'banner warn' }, `${z.fehler}. Ist der USB-Stick eingesteckt?`, ...knoepfe(true));
  const tage = z.zuletzt ? Math.floor((Date.parse(todayIso()) - Date.parse(z.zuletzt)) / 86400000) : null;
  if (tage === null || tage > 30) {
    return h('div', { class: 'banner warn' },
      `${tage === null ? 'Im zweiten Sicherungsordner liegt noch keine Kopie.' : `Die letzte Kopie im zweiten Sicherungsordner ist vom ${deDate(z.zuletzt)} (vor ${tage} Tagen).`} Einfach USB-Stick einstecken und »Jetzt sichern« wählen.`, ...knoepfe(true));
  }
  return null;
}

// Hilfe: Rundgang, Wörterbuch und (bei Beispieldaten) die kleinen Aufgaben an einer Stelle.
async function hilfeMenue() {
  const wahl = await modal('Wobei brauchst du Hilfe?', h('div', null,
    h('p', { class: 'hint' }, 'Such dir aus, was dir weiterhilft. Du kannst nichts kaputt machen.')),
  [{ label: 'Schließen', value: false },
    S.state && S.state.demo ? { label: 'Probier es aus', value: 'probier' } : null,
    { label: 'Wörter erklärt', value: 'woerter' },
    { label: 'Kurzer Rundgang', value: 'rundgang' },
    { label: 'Anleitung', cls: 'primary', value: 'anleitung' }].filter(Boolean));
  if (wahl === 'rundgang') geschichteZeigen();
  else if (wahl === 'woerter') woerterbuchDialog();
  else if (wahl === 'anleitung') anleitungDialog();
  else if (wahl === 'probier') probierDialog();
}

// Wörter, die im Verein jeder benutzt, aber nicht jeder kennt – in einfachen Sätzen.
const WOERTER = [
  ['Pächter', 'Wer einen Garten vom Verein gepachtet hat und ihn nutzt.'],
  ['Parzelle', 'Ein einzelner Garten in der Anlage. Er hat eine Nummer.'],
  ['Zählerstand', 'Die Zahl, die am Wasser- oder Stromzähler steht. Wir schreiben sie bei der Ablesung auf.'],
  ['Verbrauch', 'Neuer Zählerstand minus alter Zählerstand. So viel Wasser oder Strom wurde benutzt.'],
  ['Ablesung', 'Der Rundgang, bei dem man alle Zähler ansieht und die Zahlen notiert.'],
  ['Rechnung ausstellen', 'Die Rechnung fertig machen und als PDF speichern. Danach ändert sie sich nicht mehr.'],
  ['Zahlungsziel', 'Bis zu diesem Tag soll das Geld da sein.'],
  ['Mahnung', 'Ein Brief: Du hast noch nicht bezahlt, bitte zahle bis zu diesem Tag.'],
  ['Gutschrift', 'Geld, das ein Pächter zurückbekommt, zum Beispiel weil eine Rechnung zu hoch war.'],
  ['Vorauszahlung', 'Geld, das man schon vor der Rechnung bezahlt, oft in Raten.'],
  ['Lastschrift', 'Die Bank holt das Geld vom Konto ab. Dafür muss der Pächter vorher erlaubt haben (Mandat).'],
  ['Mandat', 'Das unterschriebene Blatt, auf dem der Pächter die Lastschrift erlaubt.'],
  ['Umlage', 'Ein Betrag, den alle Pächter zusammen für gemeinsame Kosten tragen, zum Beispiel für Wege.'],
  ['Arbeitsstunden', 'Zeit, die jeder für die Gemeinschaft arbeitet. Wer weniger arbeitet, zahlt für die fehlenden Stunden.'],
  ['Eichfrist', 'Zähler dürfen nur eine bestimmte Zeit benutzt werden. Dann müssen sie getauscht werden.'],
  ['Jahreswechsel', 'Das alte Jahr abschließen und das neue beginnen. Die Zahlen werden mitgenommen.'],
  ['Kassenbericht', 'Eine Übersicht: Wie viel Geld ist reingekommen, wie viel wurde ausgegeben?'],
  ['Kassenprüfung', 'Zwei Mitglieder prüfen, ob die Kasse stimmt, und schreiben das auf.'],
  ['Beschluss', 'Etwas, worüber abgestimmt wurde. Das Beschlussbuch merkt es sich.'],
  ['Warteliste', 'Wer einen Garten möchte, aber noch keinen hat, steht auf dieser Liste.'],
  ['Papierkorb', 'Gelöschtes bleibt hier 30 Tage liegen und lässt sich zurückholen.'],
  ['Sicherung', 'Eine Kopie aller Daten, falls einmal etwas kaputtgeht. Sie entsteht von selbst.'],
  ['Abgleich', 'Zwei Rechner haben unterschiedliche Einträge. Der Abgleich bringt beide zusammen.'],
];

async function woerterbuchDialog() {
  const suche = h('input', { type: 'search', placeholder: 'Wort suchen …', 'aria-label': 'Wort suchen', style: 'width:100%' });
  const liste = h('div', { class: 'woerter' });
  const zeichne = () => {
    const q = suche.value.trim().toLowerCase();
    const treffer = WOERTER.filter(([w, t]) => !q || w.toLowerCase().includes(q) || t.toLowerCase().includes(q));
    liste.replaceChildren(...(treffer.length ? treffer.map(([w, t]) => h('div', { class: 'wort' }, h('b', null, w), h('div', null, t)))
      : [h('p', { class: 'hint' }, 'Dazu habe ich nichts gefunden.')]));
  };
  suche.addEventListener('input', zeichne);
  zeichne();
  await modal('Wörter erklärt', h('div', null, suche, liste), [{ label: 'Schließen', cls: 'primary', value: true }]);
}

function viewUebersicht() {
  const st = S.state;
  const istAdmin = !st.hasPassword || st.loggedIn;
  const op = offenePosten(st);
  const open = op.alle;
  const today = todayIso();
  const overdue = open.filter((e) => e.faellig && e.faellig < today);

  const card = (t, big, small, cls, ic) => h('div', { class: 'card stat ' + (cls || '') }, ic ? h('div', { class: 'statico' }, icon(ic, 22)) : null,
    h('div', null, h('div', { class: 'hint' }, t), h('div', { class: 'big' }, big), h('div', { class: 'hint' }, small)));
  const cards = [
    card('Offene Rechnungen', String(open.length), open.length ? `${eur(open.reduce((a, e) => a + payOpen(e), 0))} · ${overdue.length} überfällig${op.vor.length ? ` · ${op.vor.length} aus Vorjahren` : ''}` : 'alles bezahlt', open.length ? 'warn' : 'ok', 'rechnung'),
    card('Letzte Sicherung', st.letzteSicherung ? deDate(st.letzteSicherung) : 'noch keine',
      st.sicherungFehler ? 'beschädigt!' : 'automatisch bei jeder Änderung',
      st.sicherungFehler ? 'err' : st.letzteSicherung ? 'ok' : 'warn', 'sicherung'),
  ];
  if (istAdmin) {
    if (S.dashKasse === null) { if (!S.dashKasseLoading) loadDashKasse(st.currentYear); }
    else if (S.dashKasse !== false) {
      cards.unshift(card('Kassenbestand', eur(S.dashKasse.kassenbestand), `Stand ${st.currentYear}`, S.dashKasse.kassenbestand >= 0 ? 'ok' : 'err', 'kasse'));
    }
  }

  return h('div', null,
    heroKarte(st),
    st.paechter.length ? jahresFortschritt(st, istAdmin) : null,
    beispieldatenKarte(st, istAdmin),
    jubilaeumBanner(st, istAdmin),
    st.demo ? null : zweiteSicherungBanner(st, istAdmin),
    anstehendKarte(st, istAdmin),
    h('div', { class: 'zwei-spalten' }, naechsteTermine(st), besichtigungenKarte(st, istAdmin)),
    h('div', { class: 'cards' }, cards));
}

// Wo steht das Abrechnungsjahr? Vier Stationen mit Balken und dem nächsten sinnvollen Schritt.
// Begrüßung oben auf der Übersicht: Tageszeit, Verein, Datum, ein paar Eckdaten und die Gartenillustration.
function heroKarte(st) {
  const jetzt = new Date();
  const std = jetzt.getHours();
  const gruss = std < 11 ? 'Guten Morgen' : std < 18 ? 'Guten Tag' : 'Guten Abend';
  const datum = jetzt.toLocaleDateString('de-DE', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
  const gaerten = new Set(st.paechter.filter((p) => !p.ausgeschiedenAm && (p.gartennr || '').trim()).map((p) => p.gartennr.trim())).size;
  const frei = Object.keys(st.lageplan || {}).filter((nr) => !st.paechter.some((p) => !p.ausgeschiedenAm && (p.gartennr || '').trim() === nr)).length;
  const fakt = (zahl, text) => h('div', { class: 'hero-fakt' }, h('b', null, zahl), h('span', null, text));
  const bild = h('img', { class: 'hero-bild', src: '/bilder/garten.svg', alt: '', onerror: (e) => e.target.remove() });
  return h('section', { class: 'hero' },
    h('div', { class: 'hero-text' },
      h('span', { class: 'hero-gruss' }, `${gruss} · ${datum}`),
      h('h2', null, st.settings.vereinName),
      h('p', null, `Abrechnungsjahr ${st.currentYear}. Hier siehst du auf einen Blick, wo der Verein steht und was als Nächstes ansteht.`),
      h('div', { class: 'hero-fakten' },
        fakt(String(gaerten), gaerten === 1 ? 'Garten verpachtet' : 'Gärten verpachtet'),
        frei ? fakt(String(frei), frei === 1 ? 'Garten frei' : 'Gärten frei') : null,
        st.wartelisteAnzahl && modulAn('warteliste') ? fakt(String(st.wartelisteAnzahl), 'auf der Warteliste') : null)),
    bild);
}

function jahresFortschritt(st, istAdmin) {
  const n = st.paechter.length;
  const vollst = st.paechter.filter((p) => st.results[p.id] && st.results[p.id].vollstaendig).length;
  const ausgestellt = st.paechter.filter((p) => (st.issued || {})[p.id]).length;
  const forderungen = (st.archive || []).filter((e) => e.status === 'gueltig' && e.gesamt > 0);
  const bezahlt = forderungen.filter((e) => !payIsOpen(e)).length;
  const gueltig = (st.archive || []).filter((e) => e.status === 'gueltig' && e.jahr === st.currentYear);
  const verschickt = gueltig.filter((e) => e.emailAm || e.postAm).length;
  const go = (tab, extra) => () => { S.tab = tab; Object.assign(S, extra || {}); render(); };
  const stationen = [
    // führt gleich zum ersten Pächter, bei dem noch etwas fehlt (Schnelleingabe)
    { titel: 'Zählerstände', wert: vollst, von: n, text: `${vollst} von ${n} vollständig`, los: 'Zählerstände eintragen',
      go: go('eingabe', { eingabeMode: 'schnell', quickId: ([...st.paechter].sort(cmpNr).find((p) => !(st.results[p.id] && st.results[p.id].vollstaendig)) || {}).id || S.quickId }) },
    { titel: 'Rechnungen', wert: ausgestellt, von: vollst || n, text: `${ausgestellt} von ${vollst} ausgestellt`, los: 'Rechnungen ausstellen', go: go('rechnungen', { invTab: 'pruefen' }) },
    // per Post gedruckt oder E-Mail vorbereitet
    { titel: 'Verschicken', wert: verschickt, von: gueltig.length, text: gueltig.length ? `${verschickt} von ${gueltig.length} verschickt` : 'noch keine Rechnung', los: 'Rechnungen verschicken', go: go('rechnungen', { invTab: 'pruefen' }) },
    // nur Guthaben ausgestellt: es gibt nichts einzusammeln, die Station gilt als erledigt
    forderungen.length || !ausgestellt
      ? { titel: 'Zahlungen', wert: bezahlt, von: forderungen.length, text: forderungen.length ? `${bezahlt} von ${forderungen.length} bezahlt` : 'noch keine Rechnung', los: 'Zahlungen abhaken', go: go('zahlungen') }
      : { titel: 'Zahlungen', wert: 1, von: 1, text: 'nichts offen', los: 'Zahlungen ansehen', go: go('zahlungen') },
    { titel: 'Jahresabschluss', wert: 0, von: 1, text: `${st.currentYear} abschließen`, los: 'Zum Jahreswechsel', go: go('admin', { adminTab: 'jahreswechsel' }), nurAdmin: true },
  ];
  // die erste Station, die noch nicht fertig ist, ist der nächste Schritt
  let naechste = stationen.findIndex((x) => x.von === 0 || x.wert < x.von);
  if (naechste < 0) naechste = stationen.length - 1;
  if (stationen[naechste].nurAdmin && !istAdmin) naechste = -1;
  const fertig = (x) => x.von > 0 && x.wert >= x.von && !x.nurAdmin;
  return h('div', { class: 'card fortschritt' },
    h('div', { class: 'fs-kopf' },
      h('div', null, h('b', null, `Abrechnungsjahr ${st.currentYear}`), h('span', { class: 'hint' }, ' – wo stehen wir?')),
      naechste >= 0 ? h('button', { class: 'btn primary small', onclick: stationen[naechste].go }, `Weiter: ${stationen[naechste].los} →`) : null),
    h('ol', { class: 'fs-stationen', 'aria-label': 'Schritte im Abrechnungsjahr' }, stationen.map((x, i) => {
      const anteil = x.von > 0 ? Math.min(1, x.wert / x.von) : 0;
      const cls = fertig(x) ? 'fertig' : i === naechste ? 'aktiv' : '';
      return h('li', { class: cls, onclick: x.nurAdmin && !istAdmin ? null : x.go, title: x.los },
        h('span', { class: 'fs-punkt' }, fertig(x) ? '✓' : String(i + 1)),
        h('span', { class: 'fs-text' }, h('b', null, x.titel), h('span', null, x.text)),
        h('span', { class: 'fs-balken' }, h('i', { style: `width:${Math.round(anteil * 100)}%` })));
    })),
    naechste >= 0 ? h('p', { class: 'fs-handy hint' }, h('b', null, `Schritt ${naechste + 1} von ${stationen.length}: ${stationen[naechste].titel}`), ` – ${stationen[naechste].text}`) : null);
}

