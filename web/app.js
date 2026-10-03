'use strict';
/* Kleingarten-Manager – Oberfläche. Alle Texte aus Daten werden nur über textContent/DOM-Knoten
   eingefügt (kein innerHTML), damit Namen o. Ä. nie als Code ausgeführt werden können. */

const S = { state: null, year: null, tab: 'uebersicht', adminTab: 'paechter', filter: '', selInvoice: null, invTab: 'pruefen', invView: 'archiv', eingabeMode: 'schnell', quickId: null, quickQ: '', payFilter: 'offen', payQ: '', importPreview: null, kasse: null, kasseLoading: false, kasseYear: null, kasseNurOhneBeleg: false, kasseVerlauf: null, kasseVerlaufLoading: false, remindDismissed: false, nurUnvollstaendig: false, archivSuche: '', archivAlle: null, archivAlleLoading: false, papierkorb: null, papierkorbLoading: false, papierkorbOffen: false, dashKasse: null, dashKasseLoading: false,
  mahnSel: new Set(), pruefLoginMode: false, protokoll: null, protokollLoading: false, updateCheck: null, updateChecking: false,
  sicherungDismissed: false, abschlussCheck: null, abschlussLoading: false, jubilaeen: null, jubilaeenLoading: false, datenschutz: null, datenschutzLoading: false,
  dashJubilaeen: null, dashJubilaeenLoading: false, jubilaeumDismissed: false, globalSearchQ: '', tour: null, tourAutoChecked: false };

// ------------------------------------------------------------------ Hilfsfunktionen

function h(tag, props, ...kids) {
  const el = document.createElement(tag);
  let value;
  for (const [k, v] of Object.entries(props || {})) {
    if (v === false || v == null) continue;
    if (k === 'class') el.className = v;
    else if (k === 'value') value = v;
    else if (k === 'checked') el.checked = !!v;
    else if (k.startsWith('on') && typeof v === 'function') el.addEventListener(k.slice(2), v);
    else el.setAttribute(k, v === true ? '' : String(v));
  }
  for (const kid of kids.flat(Infinity)) {
    if (kid == null || kid === false) continue;
    el.append(kid instanceof Node ? kid : document.createTextNode(String(kid)));
  }
  if (value !== undefined) el.value = value;
  return el;
}

function svgEl(tag, attrs, ...kids) {
  const el = document.createElementNS('http://www.w3.org/2000/svg', tag);
  for (const [k, v] of Object.entries(attrs || {})) if (v != null) el.setAttribute(k, v);
  for (const kid of kids.flat(Infinity)) {
    if (kid == null || kid === false) continue;
    el.append(kid instanceof Node ? kid : document.createTextNode(String(kid)));
  }
  return el;
}

// Einfaches Balkendiagramm ohne externe Bibliothek (reines inline-SVG, keine
// Fremdskripte nötig). data: [{label, value}]. valueFmt formatiert die Zahl
// über den Balken, negative Werte werden rot dargestellt.
function barChart(data, opts) {
  const { height = 150, valueFmt = (v) => nfFlex.format(v) } = opts || {};
  if (!data.length) return h('p', { class: 'hint' }, 'Noch keine Werte vorhanden.');
  const width = Math.max(260, data.length * 70);
  const padL = 6, padB = 32, padT = 18, padR = 6;
  const w = width - padL - padR, hgt = height - padT - padB;
  const max = Math.max(1, ...data.map((d) => Math.abs(d.value)));
  const bw = w / data.length;
  const svg = svgEl('svg', { viewBox: `0 0 ${width} ${height}`, width: '100%', height, class: 'barchart' });
  svg.append(svgEl('line', { x1: padL, y1: padT + hgt, x2: padL + w, y2: padT + hgt, class: 'axis' }));
  data.forEach((d, i) => {
    const bh = (Math.abs(d.value) / max) * hgt;
    const x = padL + i * bw + bw * 0.18;
    const y = padT + hgt - bh;
    svg.append(svgEl('rect', { x, y, width: bw * 0.64, height: Math.max(bh, 1), rx: 2, class: 'bar' + (d.value < 0 ? ' neg' : '') }));
    svg.append(svgEl('text', { x: x + bw * 0.32, y: y - 4, class: 'barval' }, valueFmt(d.value)));
    svg.append(svgEl('text', { x: x + bw * 0.32, y: height - 10, class: 'barlabel' }, d.label));
  });
  return h('div', { class: 'chartwrap' }, svg);
}

const nf2 = new Intl.NumberFormat('de-DE', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const nfFlex = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 2 });
const nfIn = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 4, useGrouping: false });
const eur = (x) => nf2.format(x) + ' €';
const numIn = (v) => (v == null ? '' : nfIn.format(v));

// Zahlen deutsch oder englisch schreibbar: "2,5" "2.5" "1.234,5"; leer -> null; Unsinn -> NaN
function parseNum(s) {
  s = String(s ?? '').trim().replace(/[€\s ]/g, '');
  if (s === '') return null;
  if (s.includes(',')) s = s.replace(/\./g, '').replace(',', '.');
  else if ((s.match(/\./g) || []).length > 1) s = s.replace(/\./g, '');
  if (!/^-?\d*\.?\d+$|^-?\d+\.$/.test(s)) return NaN;
  return Number(s);
}

const cmpNr = (a, b) => a.mitgliedsnr.localeCompare(b.mitgliedsnr, 'de', { numeric: true });

async function api(method, url, body, isForm) {
  const opt = { method, headers: { 'X-GA-Request': '1' } };
  if (isForm) opt.body = body;
  else if (body !== undefined) {
    opt.headers['Content-Type'] = 'application/json';
    opt.body = JSON.stringify(body);
  }
  let r;
  try {
    r = await fetch(url, opt);
  } catch (e) {
    const err = new Error('Keine Verbindung zum Programm. Läuft das Fenster mit dem Kleingarten-Manager noch?');
    err.network = true;
    throw err;
  }
  const isJson = (r.headers.get('content-type') || '').includes('json');
  const data = isJson ? await r.json() : null;
  if (!r.ok) {
    const err = new Error((data && data.error) || `Fehler ${r.status}`);
    err.status = r.status;
    throw err;
  }
  return data;
}

function toast(msg, kind) {
  const box = document.getElementById('toasts');
  const t = h('div', { class: 'toast ' + (kind || '') }, msg);
  box.append(t);
  setTimeout(() => t.remove(), kind === 'err' ? 7000 : 3500);
}

function handleErr(e) {
  if (e.status === 401) {
    S.state.loggedIn = false;
    render();
  }
  toast(e.message, 'err');
}

function modal(title, body, buttons) {
  return new Promise((resolve) => {
    const dlg = h('dialog');
    const close = (val) => { dlg.close(); dlg.remove(); resolve(val); };
    const foot = h('div', { class: 'dfoot' });
    for (const b of buttons) {
      foot.append(h('button', {
        class: 'btn ' + (b.cls || ''), type: 'button',
        onclick: async () => {
          if (b.action && (await b.action()) === false) return;
          close(b.value);
        },
      }, b.label));
    }
    dlg.append(h('div', { class: 'dhead' }, title), h('div', { class: 'dbody' }, body), foot);
    dlg.addEventListener('cancel', (e) => { e.preventDefault(); close(undefined); });
    document.body.append(dlg);
    dlg.showModal();
  });
}

// Im eigenen Programmfenster (Go stellt kgmOpenExternal bereit) gibt es keine Browser-Tabs:
// target=_blank und window.open würden dort je nach Plattform nichts tun oder das App-Fenster
// verlassen. Dokumente (PDF, Belege) werden deshalb in einem Vorschau-Dialog angezeigt.
const inNativeWindow = () => typeof window.kgmOpenExternal === 'function';

function openViewer(url, title) {
  const frame = h('iframe', { src: url, title: title || 'Vorschau' });
  const done = modal(title || 'Vorschau', frame, [{ label: 'Schließen', value: true }]);
  const dlg = document.querySelector('dialog:last-of-type');
  if (dlg) dlg.classList.add('viewer');
  return done;
}

function openDocument(url, title) {
  if (inNativeWindow()) return openViewer(url, title);
  window.open(url, '_blank');
  return Promise.resolve();
}

document.addEventListener('click', (e) => {
  if (!inNativeWindow() || e.defaultPrevented) return;
  const a = e.target.closest && e.target.closest('a[target="_blank"]');
  if (!a || a.hasAttribute('download')) return;
  const u = new URL(a.href, location.href);
  if (u.origin !== location.origin) return;
  e.preventDefault();
  openViewer(a.href, a.textContent.trim() || 'Vorschau');
});

const confirmBox = (msg, okLabel, danger) =>
  modal('Bitte bestätigen', h('p', null, msg), [
    { label: 'Abbrechen', value: false },
    { label: okLabel || 'OK', cls: danger ? 'solid-danger' : 'primary', value: true },
  ]);

// ------------------------------------------------------------------ Laden

async function load() {
  const q = S.year ? `?year=${S.year}` : '';
  S.state = await api('GET', '/api/state' + q);
  S.year = S.state.year;
}

async function reload() {
  try { await load(); render(); } catch (e) { handleErr(e); }
}

// ------------------------------------------------------------------ Grundgerüst

function render() {
  const st = S.state;
  const app = document.getElementById('app');
  app.replaceChildren();

  const istVollAdmin = !st.hasPassword || st.loggedIn;
  if (S.pruefLoginMode && !istVollAdmin) { app.append(viewPruefLogin()); return; }
  if (st.pruefLoggedIn && !istVollAdmin) { app.append(viewPruefShell()); return; }

  const yearSel = h('select', {
    onchange: async (e) => { S.year = Number(e.target.value); await reload(); },
    title: 'Abrechnungsjahr',
  }, st.years.map((y) => h('option', { value: y, selected: y === st.year },
    y === st.currentYear ? `${y} (aktuell)` : `${y} (abgeschlossen)`)));
  if (S.tab === 'admin' || S.tab === 'uebersicht') yearSel.disabled = true;

  const tabBtn = (id, label) => h('button', {
    class: S.tab === id ? 'active' : '', 'data-tour': id,
    onclick: async () => {
      S.tab = id;
      if ((id === 'admin' || id === 'uebersicht') && S.year !== st.currentYear) S.year = st.currentYear;
      await reload();
    },
  }, label);

  if (!S.tourAutoChecked && istVollAdmin) {
    S.tourAutoChecked = true;
    if (!st.tutorialGesehen) S.tour = { i: 0 };
  }

  app.append(
    h('header', { class: 'top' },
      h('div', { class: 'brand' }, h('b', null, 'Kleingarten-Manager'), h('span', null, st.settings.vereinName)),
      h('nav', { class: 'tabs' }, tabBtn('uebersicht', 'Übersicht'), tabBtn('eingabe', 'Zählerstände'), tabBtn('lageplan', 'Lageplan'), tabBtn('rechnungen', 'Rechnungen'), tabBtn('zahlungen', 'Zahlungen'), tabBtn('admin', 'Admin')),
      h('div', { class: 'spacer' }),
      globalSearchBox(st),
      h('div', null, h('label', null, 'Jahr'), yearSel),
      h('button', { class: 'quit help', 'data-tour': 'hilfe', onclick: () => tourGo(0), title: 'Hilfe: kurze Einführung, was wohin gehört', 'aria-label': 'Hilfe' }, '?'),
      h('button', { class: 'quit', onclick: quitApp, title: 'Programm beenden' }, 'Beenden'),
    ),
    h('main', null,
      sicherungBanner(st),
      openRechnungenBanner(st),
      st.readOnly && S.tab !== 'admin' && S.tab !== 'zahlungen'
        ? h('div', { class: 'banner info' }, `Das Jahr ${st.year} ist abgeschlossen. Du kannst es ansehen und Rechnungen neu ausdrucken, aber nichts mehr ändern.`)
        : null,
      S.tab === 'uebersicht' ? viewUebersicht() : S.tab === 'eingabe' ? viewEingabe() : S.tab === 'lageplan' ? viewLageplan()
        : S.tab === 'rechnungen' ? viewRechnungen() : S.tab === 'zahlungen' ? viewZahlungen() : viewAdmin(),
      h('div', { class: 'footer' }, `Kleingarten-Manager ${st.version} · Copyright © ${new Date().getFullYear()} ${st.autor || ''} · Daten liegen in: `, h('span', { class: 'mono' }, st.dataDir)),
    ),
  );
  const tour = tourCard();
  if (tour) app.append(tour);
  tourMark();
}

// ------------------------------------------------------------------ Einführungsrundgang

// Jeder Schritt wechselt selbst zur passenden Stelle im Programm und markiert den
// zugehörigen Reiter, die Seite dahinter bleibt bedienbar.
const TOUR = [
  { titel: 'Willkommen im Kleingarten-Manager',
    text: 'In 5 kurzen Schritten siehst du, was wohin gehört. Die Reiter oben führen dich der Reihe nach durch ein Abrechnungsjahr, und ich zeige dir jede Stelle direkt im Programm. Alles wird sofort gespeichert, du kannst nichts kaputt machen.',
    ziel: 'uebersicht', gehe: () => { S.tab = 'uebersicht'; } },
  { titel: '1. Einmalig einrichten',
    text: 'Unter »Admin« legst du zuerst die Pächter an (einzeln oder per Excel/CSV-Import) und trägst bei »Preise & Einstellungen« Vereinsname, Bankverbindung, Preise und Rechnungsdatum ein. Das bleibt in allen Folgejahren erhalten.',
    ziel: 'admin', gehe: () => { S.tab = 'admin'; S.adminTab = 'paechter'; } },
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
    text: 'Am Jahresende schließt »Admin → Jahreswechsel« das Jahr ab, mit Checkliste. Deine Daten werden automatisch gesichert; unter »Admin → Passwort« kannst du einen Zugang einrichten. Das Suchfeld oben findet jeden Pächter, und diese Einführung erreichst du jederzeit über den runden »?«-Knopf oben rechts.',
    ziel: 'hilfe', gehe: () => { S.tab = 'admin'; S.adminTab = 'jahreswechsel'; } },
];

async function tourGo(i) {
  S.tour = { i };
  const step = TOUR[i];
  if (step.gehe) step.gehe();
  if ((S.tab === 'admin' || S.tab === 'uebersicht') && S.year !== S.state.currentYear) S.year = S.state.currentYear;
  await reload();
}

function tourEnd() {
  S.tour = null;
  if (S.state && !S.state.tutorialGesehen) {
    S.state.tutorialGesehen = true;
    api('POST', '/api/tutorial').catch(() => { /* beim nächsten Start erneut anbieten */ });
  }
  render();
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
function globalSearchBox(st) {
  const list = st.paechter || [];
  const find = (q) => {
    q = q.trim().toLowerCase();
    if (!q) return [];
    return list.filter((p) => [p.mitgliedsnr, p.gartennr, p.name, p.strasse].some((x) => (x || '').toLowerCase().includes(q)))
      .sort(cmpNr).slice(0, 8);
  };
  const results = h('div', { class: 'globalsearch-results' });
  const goTo = (p) => {
    S.tab = 'eingabe';
    S.eingabeMode = 'schnell';
    S.quickId = p.id;
    S.globalSearchQ = '';
    render();
  };
  const draw = () => {
    results.replaceChildren();
    const m = find(S.globalSearchQ);
    if (!S.globalSearchQ.trim()) { results.classList.remove('open'); return; }
    results.classList.add('open');
    if (!m.length) { results.append(h('div', { class: 'gs-empty' }, 'Nichts gefunden.')); return; }
    for (const p of m)
      results.append(h('button', { class: 'gs-item', type: 'button', onclick: () => goTo(p) },
        h('b', null, p.gartennr || p.mitgliedsnr), ' ', p.name,
        h('span', { class: 'hint' }, p.mitgliedsnr)));
  };
  const input = h('input', {
    type: 'search', placeholder: '🔍 Garten, Name, Nummer …', class: 'gs-input', autocomplete: 'off',
    value: S.globalSearchQ,
    oninput: (e) => { S.globalSearchQ = e.target.value; draw(); },
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

// Erinnerung an offene Rechnungen des laufenden Jahres, direkt nach dem Öffnen sichtbar.
// Bleibt bis zum nächsten Programmstart ausgeblendet, sobald sie einmal weggeklickt wurde.
// Warnt, falls die jüngste Sicherung beim Programmstart nicht lesbar war
// (z. B. Festplattenfehler, abgebrochener Schreibvorgang). Sehr selten, aber
// sicherheitsrelevant genug, um auf jedem Tab zu erscheinen.
function sicherungBanner(st) {
  if (S.sicherungDismissed || !st.sicherungFehler) return null;
  return h('div', { class: 'banner err' },
    `⚠ ${st.sicherungFehler}.`, ' ',
    h('button', { class: 'btn small', onclick: () => { S.sicherungDismissed = true; render(); } }, 'Ausblenden'));
}

function openRechnungenBanner(st) {
  if (S.remindDismissed || S.tab === 'zahlungen' || S.tab === 'uebersicht' || st.year !== st.currentYear) return null;
  const all = (st.archive || []).filter((e) => e.status === 'gueltig' && e.gesamt >= 0);
  const open = all.filter(payIsOpen);
  if (!open.length) return null;
  const today = todayIso();
  const overdue = open.filter((e) => e.faellig && e.faellig < today);
  return h('div', { class: 'banner warn' },
    `${open.length} Rechnung${open.length === 1 ? '' : 'en'} ${st.year} noch offen${overdue.length ? `, davon ${overdue.length} überfällig` : ''}.`, ' ',
    h('button', { class: 'btn small', onclick: () => { S.tab = 'zahlungen'; render(); } }, 'Zu den Zahlungen'), ' ',
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
  if (S.jubilaeumDismissed || !istAdmin) return null;
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
    if (!seit) continue;
    const jahre = jahr - Number(seit.slice(0, 4));
    if (jahre > 0 && jahre % 5 === 0) treffer.push(`${p.name} (${jahre} Jahre)`);
  }
  if (!treffer.length) return null;
  return h('div', { class: 'banner info' },
    `🎉 ${treffer.length} Mitglied(er) haben ${jahr} ein rundes Jubiläum: ${treffer.join(', ')}.`, ' ',
    h('button', { class: 'btn small', onclick: () => { S.tab = 'admin'; S.adminTab = 'jubilaeen'; render(); } }, 'Zu den Jubiläen'), ' ',
    h('button', { class: 'btn small', onclick: () => { S.jubilaeumDismissed = true; render(); } }, 'Ausblenden'));
}

function viewUebersicht() {
  const st = S.state;
  const istAdmin = !st.hasPassword || st.loggedIn;
  const all = (st.archive || []).filter((e) => e.status === 'gueltig' && e.gesamt >= 0);
  const open = all.filter(payIsOpen);
  const today = todayIso();
  const overdue = open.filter((e) => e.faellig && e.faellig < today);
  const unvollstaendig = st.paechter.filter((p) => !(st.results[p.id] && st.results[p.id].vollstaendig));

  const card = (t, big, small, cls) => h('div', { class: 'card stat ' + (cls || '') }, h('div', { class: 'hint' }, t), h('div', { class: 'big' }, big), h('div', { class: 'hint' }, small));
  const cards = [
    card('Offene Rechnungen', String(open.length), open.length ? `${eur(open.reduce((a, e) => a + payOpen(e), 0))} · ${overdue.length} überfällig` : 'alles bezahlt', open.length ? 'warn' : 'ok'),
    card('Unvollständige Pächter', String(unvollstaendig.length), `von ${st.paechter.length} in ${st.currentYear}`, unvollstaendig.length ? 'warn' : 'ok'),
    card('Letzte Sicherung', st.letzteSicherung ? deDate(st.letzteSicherung) : 'noch keine',
      st.sicherungFehler ? 'beschädigt!' : 'automatisch bei jeder Änderung',
      st.sicherungFehler ? 'err' : st.letzteSicherung ? 'ok' : 'warn'),
  ];
  if (istAdmin) {
    if (S.dashKasse === null) { if (!S.dashKasseLoading) loadDashKasse(st.currentYear); }
    else if (S.dashKasse !== false) {
      cards.unshift(card('Kassenbestand', eur(S.dashKasse.kassenbestand), `Stand ${st.currentYear}`, S.dashKasse.kassenbestand >= 0 ? 'ok' : 'err'));
    }
  }

  return h('div', null,
    h('h2', null, `Willkommen beim Kleingarten-Manager ${st.currentYear}`),
    h('p', { class: 'hint' }, `${st.settings.vereinName}. Kurzer Überblick über den aktuellen Stand.`),
    jubilaeumBanner(st, istAdmin),
    h('div', { class: 'cards' }, cards),
    h('div', { class: 'actions', style: 'margin-top:6px' },
      h('button', { class: 'btn primary', onclick: () => { S.tab = 'eingabe'; render(); } }, 'Zu den Zählerständen'),
      h('button', { class: 'btn', onclick: () => { S.tab = 'zahlungen'; render(); } }, 'Zu den Zahlungen'),
      istAdmin ? h('button', { class: 'btn', onclick: () => { S.tab = 'admin'; S.adminTab = 'kassenbericht'; render(); } }, 'Zum Kassenbericht') : null));
}

// ------------------------------------------------------------------ Tab: Zählerstände

function statusBadge(res) {
  if (res.vollstaendig) return h('span', { class: 'badge ok' }, '✓ OK');
  return h('span', { class: 'badge ' + (res.status === 'Angaben fehlen' ? 'warn' : 'err'), title: res.status }, res.status === 'Angaben fehlen' ? 'unvollständig' : res.status);
}

function updateCalc(tr, res) {
  tr.querySelector('.c-wv').textContent = res.wasserVerbrauch == null ? '' : nfFlex.format(res.wasserVerbrauch);
  tr.querySelector('.c-sv').textContent = res.energieVerbrauch == null ? '' : nfFlex.format(res.energieVerbrauch);
  const g = tr.querySelector('.gesamt');
  g.className = 'gesamt' + (res.vollstaendig && res.gesamt < 0 ? ' guthaben' : '');
  g.title = res.vollstaendig && res.gesamt < 0 ? 'Guthaben zu Gunsten des Pächters' : '';
  g.textContent = res.vollstaendig ? eur(res.gesamt) : '–';
  const inf = ((S.state && S.state.issued) || {})[tr.dataset.id];
  const hints = res.hinweise || ((S.state && S.state.hinweise) || {})[tr.dataset.id];
  tr.querySelector('.status').replaceChildren(...[statusBadge(res),
    inf ? h('span', { class: 'badge ' + (inf.geaendert ? 'warn' : 'neu'), title: inf.geaendert ? 'Rechnung wurde ausgestellt, danach wurden Werte geändert' : 'Rechnung ist ausgestellt' }, inf.geaendert ? '⚠ nach Ausstellung geändert' : 'ausgestellt') : null,
    hints && hints.length ? h('span', { class: 'badge warn', title: hints.join('\n') }, '⚠ Verbrauch prüfen') : null].filter(Boolean));
}

// Nach einer Änderung neu vom Programm holen, ob eine ausgestellte Rechnung davon betroffen ist.
async function refreshIssued(tr) {
  try {
    const st = await api('GET', `/api/state?year=${S.year}`);
    S.state.issued = st.issued; S.state.archive = st.archive;
    updateCalc(tr, S.state.results[tr.dataset.id]);
  } catch (e) { /* nicht kritisch */ }
}

let savedTimer;
function showSaved() {
  const el = document.querySelector('.saveind');
  if (!el) return;
  el.classList.add('show');
  clearTimeout(savedTimer);
  savedTimer = setTimeout(() => el.classList.remove('show'), 1800);
}

function hasDetails(a) {
  return !!(a && (a.versicherung || a.grundsteuer || a.auslagen || (a.hinweis && a.hinweis.trim())));
}

async function saveRow(tr, p) {
  const f = (k) => tr.querySelector(`[data-f="${k}"]`);
  const keys = ['wasserVJ', 'wasserAkt', 'stromVJ', 'stromAkt', 'stunden', 'abschlag'];
  const vals = {};
  let bad = false;
  for (const k of keys) {
    const v = parseNum(f(k).value);
    const wrong = Number.isNaN(v) || (v !== null && v < 0);
    f(k).classList.toggle('err', wrong);
    bad = bad || wrong;
    vals[k] = v;
  }
  if (bad) { toast('Bitte nur Zahlen ab 0 eingeben (Komma ist erlaubt).', 'err'); return; }
  try {
    const wz = f('wz').value.trim(), sz = f('sz').value.trim();
    if (wz !== p.wasserzaehlerNr || sz !== p.stromzaehlerNr) {
      const np = await api('PUT', `/api/zaehler/${p.id}`, { wasserzaehlerNr: wz, stromzaehlerNr: sz });
      Object.assign(p, np);
    }
    const cur = S.state.ablesungen[p.id] || {};
    const body = { ...cur, wasserVJ: vals.wasserVJ, wasserAkt: vals.wasserAkt, stromVJ: vals.stromVJ, stromAkt: vals.stromAkt,
      stunden: vals.stunden, abschlag: vals.abschlag ?? 0 };
    const res = await api('PUT', `/api/ablesung/${p.id}?year=${S.year}`, body);
    S.state.ablesungen[p.id] = body;
    S.state.results[p.id] = res;
    S.state.hinweise = S.state.hinweise || {};
    if (res.hinweise && res.hinweise.length) S.state.hinweise[p.id] = res.hinweise; else delete S.state.hinweise[p.id];
    updateCalc(tr, res);
    showSaved();
    if ((S.state.issued || {})[p.id]) refreshIssued(tr);
  } catch (e) { handleErr(e); }
}

const WECHSEL_WASSER = { wKey: 'wasserWechsel', field: 'wz', label: 'Wasser', unit: 'm³' };
const WECHSEL_STROM = { wKey: 'stromWechsel', field: 'sz', label: 'Strom', unit: 'kWh' };

async function saveWechsel(tr, p, art, wechsel) {
  const cur = S.state.ablesungen[p.id] || {};
  const body = { ...cur, [art.wKey]: wechsel };
  const res = await api('PUT', `/api/ablesung/${p.id}?year=${S.year}`, body);
  S.state.ablesungen[p.id] = body;
  S.state.results[p.id] = res;
  S.state.hinweise = S.state.hinweise || {};
  if (res.hinweise && res.hinweise.length) S.state.hinweise[p.id] = res.hinweise; else delete S.state.hinweise[p.id];
  updateCalc(tr, res);
  if (wechsel && wechsel.neueNr) {
    p[art.field === 'wz' ? 'wasserzaehlerNr' : 'stromzaehlerNr'] = wechsel.neueNr;
    const el = tr.querySelector(`[data-f="${art.field}"]`);
    if (el) el.value = wechsel.neueNr;
  }
  const btn = tr.querySelector(`.wechselbtn-${art.field}`);
  if (btn) btn.classList.toggle('primary', !!wechsel);
  showSaved();
  if ((S.state.issued || {})[p.id]) refreshIssued(tr);
}

function wechselDialog(tr, p, art) {
  const cur = S.state.ablesungen[p.id] || {};
  const w = cur[art.wKey] || {};
  const altEnde = h('input', { type: 'text', inputmode: 'decimal', value: numIn(w.altEnde), autocomplete: 'off' });
  const neueNr = h('input', { type: 'text', maxlength: 40, value: w.neueNr || '', autocomplete: 'off' });
  const neuStart = h('input', { type: 'text', inputmode: 'decimal', value: numIn(w.neuStart != null ? w.neuStart : 0), autocomplete: 'off' });
  const fld = (label, input, unit) => h('div', { class: 'field' }, h('label', null, label),
    unit ? h('div', { class: 'inputunit' }, input, h('span', null, unit)) : input);
  const hatWechsel = w.altEnde != null || w.neuStart != null || (w.neueNr && w.neueNr.trim());
  const body = h('div', { class: 'form' },
    h('div', { class: 'field wide' }, h('p', { class: 'hint', style: 'margin:0' },
      `Wurde der ${art.label}zähler unterjährig getauscht? Endstand des alten und Anfangsstand des neuen Zählers eintragen – der Verbrauch wird dann aus beiden Zählern zusammengerechnet. Leer lassen, wenn es keinen Wechsel gab.`)),
    fld('Endstand alter Zähler', altEnde, art.unit), fld('Neue Zähler-Nr.', neueNr), fld('Anfangsstand neuer Zähler', neuStart, art.unit));
  return modal(`Zählerwechsel ${art.label} – ${p.name}`, body, [
    { label: 'Abbrechen', value: false },
    hatWechsel ? { label: 'Wechsel entfernen', cls: 'danger', value: 'del', action: async () => {
      try { await saveWechsel(tr, p, art, null); } catch (e) { handleErr(e); return false; }
    } } : null,
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const ae = parseNum(altEnde.value), ns = parseNum(neuStart.value);
      const leer = ae === null && ns === null && !neueNr.value.trim();
      if (!leer && (Number.isNaN(ae) || (ae !== null && ae < 0) || Number.isNaN(ns) || (ns !== null && ns < 0))) {
        toast('Bitte gültige Zahlen ab 0 eingeben.', 'err'); return false;
      }
      try { await saveWechsel(tr, p, art, leer ? null : { altEnde: ae, neueNr: neueNr.value.trim(), neuStart: ns }); } catch (e) { handleErr(e); return false; }
    } },
  ].filter(Boolean));
}

async function detailsDialog(tr, p) {
  const cur = S.state.ablesungen[p.id] || {};
  const num = (val, id) => h('input', { type: 'text', inputmode: 'decimal', id, value: numIn(val || null), autocomplete: 'off' });
  const vers = num(cur.versicherung, 'd-vers'), grund = num(cur.grundsteuer, 'd-grund'), ausl = num(cur.auslagen, 'd-ausl');
  const hint = h('textarea', { rows: 3, maxlength: 300, style: 'width:100%', id: 'd-hint' }, cur.hinweis || '');
  const field = (label, input, unit) => h('div', { class: 'field' }, h('label', null, label),
    unit ? h('div', { class: 'inputunit' }, input, h('span', null, unit)) : input);
  const body = h('div', { class: 'form' },
    field('Versicherung', vers, '€'), field('Grundsteuer', grund, '€'), field('Sonstige Auslagen', ausl, '€'),
    h('div', { class: 'field wide' }, h('label', null, 'Hinweis / Erläuterung (steht auf der Rechnung)'), hint));
  await modal(`Weitere Angaben – ${p.name}`, body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const vs = [vers, grund, ausl].map((i) => parseNum(i.value));
      if (vs.some((v) => Number.isNaN(v) || (v !== null && v < 0))) { toast('Bitte nur Beträge ab 0 eingeben.', 'err'); return false; }
      const body2 = { ...cur, versicherung: vs[0] ?? 0, grundsteuer: vs[1] ?? 0, auslagen: vs[2] ?? 0, hinweis: hint.value.trim() };
      try {
        const res = await api('PUT', `/api/ablesung/${p.id}?year=${S.year}`, body2);
        S.state.ablesungen[p.id] = body2;
        S.state.results[p.id] = res;
        updateCalc(tr, res);
        tr.querySelector('.detailbtn').classList.toggle('primary', hasDetails(body2));
        showSaved();
        if ((S.state.issued || {})[p.id]) refreshIssued(tr);
      } catch (e) { handleErr(e); return false; }
    } },
  ]);
}

function verlaufDialog(p) {
  const hist = ((S.state && S.state.history) || {})[p.id] || [];
  const row = (e) => h('tr', null,
    h('td', null, e.jahr),
    h('td', { class: 'r' }, e.wasser == null ? '–' : nfFlex.format(e.wasser) + ' m³'),
    h('td', { class: 'r' }, e.strom == null ? '–' : nfFlex.format(e.strom) + ' kWh'),
    h('td', { class: 'r' }, e.stunden == null ? '–' : nfFlex.format(e.stunden)),
    h('td', { class: 'r' }, e.gesamt == null ? '–' : eur(e.gesamt)),
    h('td', null, e.ausgestellt ? h('span', { class: 'badge ok' }, 'ausgestellt') : h('span', { class: 'badge neu' }, 'berechnet')));
  const chrono = [...hist].reverse();
  const body = hist.length
    ? h('div', null,
        h('p', { class: 'hint', style: 'margin:0 0 4px' }, 'Gesamtbetrag je Jahr:'),
        barChart(chrono.filter((e) => e.gesamt != null).map((e) => ({ label: String(e.jahr), value: e.gesamt })), { valueFmt: (v) => eur(v) }),
        h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
          h('thead', null, h('tr', null, ['Jahr', 'Wasser', 'Strom', 'Stunden', 'Gesamt', ''].map((t) => h('th', null, t)))),
          h('tbody', null, hist.map(row)))))
    : h('p', { class: 'hint' }, 'Für diesen Pächter liegen noch keine Werte aus Vorjahren vor.');
  return modal(`Jahresverlauf – ${p.name}`, body, [{ label: 'Schließen', value: true }]);
}

function eingabeRow(p, ro) {
  const a = S.state.ablesungen[p.id] || {};
  const res = S.state.results[p.id];
  const tr = h('tr', { 'data-id': p.id });
  const save = () => saveRow(tr, p);
  const inp = (key, val, cls) => h('input', { type: 'text', inputmode: 'decimal', class: cls || 'num', autocomplete: 'off',
    'data-f': key, value: numIn(val), disabled: ro, onchange: save });
  const txt = (key, val) => h('input', { type: 'text', class: 'nr', autocomplete: 'off', maxlength: 40, 'data-f': key,
    value: val || '', disabled: ro, onchange: save });
  tr.append(
    h('td', { class: 'sticky s1' }, p.mitgliedsnr),
    h('td', { class: 'name sticky s2', title: p.name }, p.name),
    h('td', null, txt('wz', p.wasserzaehlerNr), ro ? null : h('button', { class: 'btn small wechselbtn-wz' + (a.wasserWechsel ? ' primary' : ''),
      title: 'Unterjähriger Zählerwechsel', onclick: () => wechselDialog(tr, p, WECHSEL_WASSER) }, '⇄')),
    h('td', null, inp('wasserVJ', a.wasserVJ)),
    h('td', null, inp('wasserAkt', a.wasserAkt)),
    h('td', { class: 'calc c-wv' }),
    h('td', null, txt('sz', p.stromzaehlerNr), ro ? null : h('button', { class: 'btn small wechselbtn-sz' + (a.stromWechsel ? ' primary' : ''),
      title: 'Unterjähriger Zählerwechsel', onclick: () => wechselDialog(tr, p, WECHSEL_STROM) }, '⇄')),
    h('td', null, inp('stromVJ', a.stromVJ)),
    h('td', null, inp('stromAkt', a.stromAkt)),
    h('td', { class: 'calc c-sv' }),
    h('td', null, inp('stunden', a.stunden, 'small')),
    h('td', null, inp('abschlag', a.abschlag || null, 'small')),
    h('td', { class: 'gesamt' }),
    h('td', { class: 'status mid' }),
    h('td', null, h('button', { class: 'btn small detailbtn' + (hasDetails(a) ? ' primary' : ''), disabled: ro,
      title: 'Versicherung, Grundsteuer, Auslagen, Hinweis', onclick: () => detailsDialog(tr, p) }, 'Weitere'), ' ',
      h('button', { class: 'btn small', title: 'Verbrauch und Betrag der Vorjahre', onclick: () => verlaufDialog(p) }, 'Verlauf')),
  );
  updateCalc(tr, res);
  // Enter springt in die Zeile darunter (gleiche Spalte)
  tr.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter' || !e.target.dataset || !e.target.dataset.f) return;
    e.preventDefault();
    e.target.dispatchEvent(new Event('change'));
    let next = tr.nextElementSibling;
    while (next && next.hidden) next = next.nextElementSibling;
    const t = next && next.querySelector(`[data-f="${e.target.dataset.f}"]`);
    if (t) { t.focus(); t.select(); }
  });
  return tr;
}

function viewEingabe() {
  const st = S.state;
  return h('div', null,
    h('h2', null, `Zählerstände ${st.year}`),
    h('div', { class: 'subtabs' },
      h('button', { class: S.eingabeMode === 'schnell' ? 'active' : '', onclick: () => { S.eingabeMode = 'schnell'; render(); } }, 'Schnelleingabe (Nummer eintippen)'),
      h('button', { class: S.eingabeMode === 'tabelle' ? 'active' : '', onclick: () => { S.eingabeMode = 'tabelle'; render(); } }, 'Tabelle (alle Pächter)')),
    S.eingabeMode === 'schnell' ? viewSchnell() : viewTabelle());
}

// Schnelleingabe: Nummer eintippen, Name/Adresse/Größe/Zählernummern kommen aus den Stammdaten.
function viewSchnell() {
  const st = S.state;
  const ro = st.readOnly;
  const list = [...st.paechter].sort(cmpNr);
  const done = list.filter((p) => st.results[p.id] && st.results[p.id].vollstaendig).length;
  const box = h('div');
  const sugg = h('div', { class: 'sugg' });
  const find = (q) => {
    q = q.trim().toLowerCase();
    if (!q) return [];
    const exact = list.filter((p) => (p.mitgliedsnr || '').toLowerCase() === q);
    if (exact.length) return exact;
    const g = list.filter((p) => (p.gartennr || '').toLowerCase() === q);
    if (g.length) return g;
    return list.filter((p) => [p.mitgliedsnr, p.gartennr, p.name].some((x) => (x || '').toLowerCase().includes(q))).slice(0, 8);
  };
  const choose = (p) => { S.quickId = p.id; S.quickQ = ''; input.value = ''; sugg.replaceChildren(); drawCard(true); };
  const input = h('input', { type: 'search', class: 'bignr', placeholder: 'Nummer oder Name', autocomplete: 'off', style: 'width:280px', id: 'quicknr',
    oninput: () => {
      const m = find(input.value);
      sugg.replaceChildren();
      if (input.value.trim() && !m.length) sugg.append(h('span', { class: 'hint' }, 'Nichts gefunden.'));
      if (m.length > 1 || (m.length === 1 && input.value.trim().toLowerCase() !== (m[0].mitgliedsnr || '').toLowerCase()))
        for (const p of m) sugg.append(h('button', { class: 'btn small', onclick: () => choose(p) }, `${p.mitgliedsnr}  ${p.name}`));
    },
    onkeydown: (e) => {
      if (e.key !== 'Enter') return;
      e.preventDefault();
      const m = find(input.value);
      if (m.length === 1) choose(m[0]);
      else if (m.length > 1) toast('Mehrere Treffer – bitte einen anklicken oder die Nummer genauer eingeben.', 'err');
      else if (input.value.trim()) toast('Nummer nicht gefunden.', 'err');
    } });

  const drawCard = (focusFirst) => {
    box.replaceChildren();
    const p = list.find((x) => x.id === S.quickId);
    if (!p) {
      box.append(h('div', { class: 'card empty' }, list.length ? 'Tippe oben eine Nummer ein und drücke Enter. Alle Angaben des Pächters erscheinen dann automatisch.' : 'Noch keine Pächter angelegt. Das geht im Bereich »Admin« → Pächter.'));
      return;
    }
    const a = st.ablesungen[p.id] || {};
    const card = h('div', { class: 'card quick', 'data-id': p.id });
    const save = () => saveRow(card, p);
    const inp = (key, val, cls) => h('input', { type: 'text', inputmode: 'decimal', class: cls || 'num', autocomplete: 'off', 'data-f': key, value: numIn(val), disabled: ro, onchange: save });
    const txt = (key, val) => h('input', { type: 'text', class: 'nr', autocomplete: 'off', maxlength: 40, 'data-f': key, value: val || '', disabled: ro, onchange: save });
    const fld = (label, el, unit) => h('div', { class: 'field' }, h('label', null, label), unit ? h('div', { class: 'inputunit' }, el, h('span', null, unit)) : el);
    const info = (k, v) => v ? h('span', { class: 'chip' }, h('i', null, k + ' '), v) : null;
    card.append(
      h('div', { class: 'qhead' },
        h('div', null, h('h3', null, `${p.mitgliedsnr} – ${p.name}`), h('div', { class: 'hint' }, [p.strasse, p.plzOrt].filter(Boolean).join(', ')),
          p.notiz ? h('div', { class: 'hint', style: 'margin-top:2px' }, '📝 ', p.notiz) : null),
        h('div', { class: 'chips' }, info('Garten', p.gartennr), info('Größe', p.gartengroesse ? `${nfFlex.format(p.gartengroesse)} m²` : ''),
          info('Umlage', eur(p.umlageAbweichend != null ? p.umlageAbweichend : st.settings.umlageStandard)))),
      h('div', { class: 'qgrid' },
        h('fieldset', null, h('legend', null, 'Wasser'),
          fld('Zähler-Nr.', h('div', null, txt('wz', p.wasserzaehlerNr), ro ? null : h('button', { class: 'btn small wechselbtn-wz' + (a.wasserWechsel ? ' primary' : ''),
            title: 'Unterjähriger Zählerwechsel', onclick: () => wechselDialog(card, p, WECHSEL_WASSER) }, '⇄'))),
          fld('Stand Vorjahr', inp('wasserVJ', a.wasserVJ), 'm³'), fld('Stand aktuell', inp('wasserAkt', a.wasserAkt), 'm³'),
          h('div', { class: 'field' }, h('label', null, 'Verbrauch'), h('div', { class: 'calc c-wv qv' }))),
        h('fieldset', null, h('legend', null, 'Strom'),
          fld('Zähler-Nr.', h('div', null, txt('sz', p.stromzaehlerNr), ro ? null : h('button', { class: 'btn small wechselbtn-sz' + (a.stromWechsel ? ' primary' : ''),
            title: 'Unterjähriger Zählerwechsel', onclick: () => wechselDialog(card, p, WECHSEL_STROM) }, '⇄'))),
          fld('Stand Vorjahr', inp('stromVJ', a.stromVJ), 'kWh'), fld('Stand aktuell', inp('stromAkt', a.stromAkt), 'kWh'),
          h('div', { class: 'field' }, h('label', null, 'Verbrauch'), h('div', { class: 'calc c-sv qv' }))),
        h('fieldset', null, h('legend', null, 'Sonstiges'), fld('Arbeitsstunden', inp('stunden', a.stunden, 'small'), 'h'),
          fld('Abschlag', inp('abschlag', a.abschlag || null, 'small'), '€'),
          h('button', { class: 'btn small detailbtn' + (hasDetails(a) ? ' primary' : ''), disabled: ro, onclick: () => detailsDialog(card, p) }, 'Weitere Angaben'))),
      h('div', { class: 'qfoot' },
        h('div', null, h('span', { class: 'hint' }, 'Rechnungsbetrag: '), h('b', { class: 'gesamt' }), ' ', h('span', { class: 'status' })),
        h('div', { class: 'spacer' }),
        h('button', { class: 'btn', onclick: () => verlaufDialog(p) }, 'Verlauf'),
        h('button', { class: 'btn', onclick: () => { S.tab = 'rechnungen'; S.selInvoice = p.id; S.invTab = 'pruefen'; S.invView = 'archiv'; render(); } }, 'Rechnung ansehen'),
        h('button', { class: 'btn primary', onclick: () => { input.focus(); } }, 'Fertig – nächste Nummer')));
    updateCalc(card, st.results[p.id]);
    box.append(card);

    // Enter: nächstes Feld; Pflichtfelder immer, Zählernummer/Vorjahresstände nur wenn noch leer
    const seq = ['wz', 'wasserVJ', 'wasserAkt', 'sz', 'stromVJ', 'stromAkt', 'stunden', 'abschlag'];
    const must = new Set(['wasserAkt', 'stromAkt', 'stunden', 'abschlag']);
    const want = (k) => { const el = card.querySelector(`[data-f="${k}"]`); return el && !el.disabled && (must.has(k) || el.value.trim() === ''); };
    const goto = (from) => {
      for (let i = from; i < seq.length; i++) if (want(seq[i])) { const el = card.querySelector(`[data-f="${seq[i]}"]`); el.focus(); el.select(); return; }
      input.focus();
    };
    card.addEventListener('keydown', (e) => {
      if (e.key !== 'Enter' || !e.target.dataset || !e.target.dataset.f) return;
      e.preventDefault();
      e.target.dispatchEvent(new Event('change'));
      goto(seq.indexOf(e.target.dataset.f) + 1);
    });
    if (focusFirst && !ro) goto(0);
  };
  drawCard(false);

  return h('div', null,
    h('p', { class: 'hint' }, 'Nummer eintippen, Enter drücken: Name, Adresse, Gartengröße, Umlage und Zählernummern kommen automatisch aus den Stammdaten, der Vorjahresstand ist schon eingetragen. Du gibst nur noch die neuen Stände ein. Mit Enter springst du zum nächsten Feld, nach dem letzten wieder zur Nummer. Alles wird sofort gespeichert.'),
    h('div', { class: 'toolbar' }, input, h('span', { class: 'stat' }, `${done} von ${list.length} vollständig`), h('span', { class: 'saveind' }, '✓ gespeichert')),
    sugg, box);
}

function viewTabelle() {
  const st = S.state;
  const ro = st.readOnly;
  const list = [...st.paechter].sort(cmpNr);
  const done = list.filter((p) => st.results[p.id] && st.results[p.id].vollstaendig).length;

  const tbody = h('tbody');
  const fill = () => {
    tbody.replaceChildren();
    const q = S.filter.trim().toLowerCase();
    const shown = list.filter((p) => (!q || [p.mitgliedsnr, p.name, p.gartennr].some((x) => (x || '').toLowerCase().includes(q))) &&
      (!S.nurUnvollstaendig || !(st.results[p.id] && st.results[p.id].vollstaendig)));
    for (const p of shown) tbody.append(eingabeRow(p, ro));
    if (!shown.length) tbody.append(h('tr', null, h('td', { colspan: 15, class: 'empty' },
      list.length ? 'Keine Treffer.' : 'Noch keine Pächter angelegt. Das geht im Bereich »Admin« → Pächter.')));
  };
  fill();

  const search = h('input', { type: 'search', placeholder: 'Suchen (Nummer oder Name)', value: S.filter, style: 'width:240px',
    oninput: (e) => { S.filter = e.target.value; fill(); } });
  const nurOffen = h('label', { class: 'hint' },
    h('input', { type: 'checkbox', checked: S.nurUnvollstaendig, onchange: (e) => { S.nurUnvollstaendig = e.target.checked; fill(); } }),
    ` nur unvollständig${list.length - done ? ` (${list.length - done})` : ''}`);

  const th = (t, cls, extra) => h('th', { class: cls || '', ...(extra || {}) }, t);
  return h('div', null,
    h('p', { class: 'hint' }, 'Hier trägst du pro Pächter die Zählernummern, Zählerstände und Arbeitsstunden ein. Alles wird sofort gespeichert und die Beträge werden automatisch berechnet.'),
    h('div', { class: 'toolbar' }, search, nurOffen,
      h('span', { class: 'stat' }, `${list.length} Pächter · ${done} vollständig`),
      h('span', { class: 'saveind' }, '✓ gespeichert'),
      h('div', { class: 'spacer' }),
      h('a', { class: 'btn', href: `/api/export?year=${st.year}` }, 'Übersicht als Excel'),
    ),
    h('div', { class: 'tablewrap' }, h('table', null,
      h('thead', null,
        h('tr', { class: 'group' }, th('', 'sticky s1', { colspan: 2 }), th('Wasser', '', { colspan: 4 }), th('Strom / Energie', '', { colspan: 4 }),
          th('', '', { colspan: 5 })),
        h('tr', { class: 'sub' }, th('Nr.', 'sticky s1'), th('Name', 'sticky s2'),
          th('Zähler-Nr.'), th('Stand Vorjahr'), th('Stand aktuell'), th('Verbrauch m³'),
          th('Zähler-Nr.'), th('Stand Vorjahr'), th('Stand aktuell'), th('Verbrauch kWh'),
          th('Arbeits-stunden'), th('Abschlag €'), th('Gesamt-betrag'), th('Status'), th('')),
      ), tbody)),
  );
}

// ------------------------------------------------------------------ Tab: Rechnungen

const fmtWhen = (iso) => { const d = new Date(iso); return isNaN(d) ? '' : d.toLocaleString('de-DE', { dateStyle: 'medium', timeStyle: 'short' }); };

function issueSummary(r) {
  return h('div', null,
    h('p', null, `${r.created.length} Rechnung(en) ausgestellt und gespeichert. Die PDF-Dateien liegen hier:`), h('p', { class: 'mono' }, r.folder),
    (r.skipped || []).length ? h('div', { class: 'banner warn' }, h('b', null, `${r.skipped.length} nicht ausgestellt:`),
      h('ul', { class: 'plain' }, r.skipped.map((x) => h('li', null, `${x.name}: ${x.reason}`)))) : null);
}

async function issue(ids, replace) {
  const st = S.state;
  const r = await api('POST', `/api/invoices/issue?year=${st.year}`, { ids, replace: !!replace });
  const open = await modal('Fertig', issueSummary(r), [{ label: 'Schließen', value: false }, { label: 'Ordner öffnen', cls: 'primary', value: true }]);
  if (open) await api('POST', '/api/open-folder', { which: 'rechnungen', year: st.year });
  await reload();
}

// ------------------------------------------------------------------ Tab: Lageplan

// Kachelübersicht aller Gärten mit Status-Farbcode, sortiert nach Gartennummer
// (natürliche Sortierung, z. B. 2 vor 10). Ein echter geometrischer Lageplan
// bräuchte Koordinaten, die es nicht gibt – die Kacheln geben trotzdem auf
// einen Blick den Überblick, welcher Garten noch offen, unvollständig oder
// erledigt ist.
// Belegungshistorie eines Gartens: welcher Pächter hatte ihn wann. Bleibt
// auch nach endgültigem Löschen eines Pächters erhalten.
async function gartenHistorieDialog(gartennr) {
  let hist;
  try { hist = await api('GET', `/api/garten-historie?gartennr=${encodeURIComponent(gartennr)}`); } catch (e) { handleErr(e); return; }
  const chrono = [...hist].reverse();
  const row = (e) => h('tr', null, h('td', null, e.mitgliedsnr), h('td', null, e.name),
    h('td', null, deDate(e.seit)), h('td', null, e.bis ? deDate(e.bis) : h('span', { class: 'badge ok' }, 'aktuell')));
  const body = chrono.length
    ? h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
        h('thead', null, h('tr', null, ['Mitgl.-Nr.', 'Name', 'Seit', 'Bis'].map((t) => h('th', null, t)))),
        h('tbody', null, chrono.map(row))))
    : h('p', { class: 'hint' }, 'Für diesen Garten liegt noch keine Historie vor.');
  await modal(`Verlauf Garten ${gartennr}`, body, [{ label: 'Schließen', value: true }]);
}

function viewLageplan() {
  const st = S.state;
  const issuedOf = (p) => (st.issued || {})[p.id];
  const list = [...st.paechter].sort((a, b) =>
    (a.gartennr || a.mitgliedsnr).localeCompare(b.gartennr || b.mitgliedsnr, 'de', { numeric: true }));

  const statusOf = (p) => {
    const res = st.results[p.id];
    if (!res || !res.vollstaendig) return 'unvollstaendig';
    const inf = issuedOf(p);
    if (!inf) return 'berechnet';
    const e = (st.archive || []).find((x) => x.id === inf.id);
    if (e && e.gesamt >= 0 && payIsOpen(e)) return 'offen';
    return 'bezahlt';
  };
  const labels = { unvollstaendig: 'Angaben fehlen', berechnet: 'berechnet, noch nicht ausgestellt', offen: 'Rechnung offen', bezahlt: 'bezahlt / kein offener Betrag' };
  const counts = { unvollstaendig: 0, berechnet: 0, offen: 0, bezahlt: 0 };
  const tiles = list.map((p) => {
    const s = statusOf(p);
    counts[s]++;
    return h('div', { class: 'kachel kachel-' + s, title: `${p.name}${p.notiz ? ' · 📝 ' + p.notiz : ''} – ${labels[s]}`,
      onclick: () => { S.tab = 'eingabe'; S.eingabeMode = 'schnell'; S.quickId = p.id; render(); } },
      p.gartennr ? h('button', { class: 'kachel-hist', title: `Verlauf von Garten ${p.gartennr}`,
        onclick: (e) => { e.stopPropagation(); gartenHistorieDialog(p.gartennr); } }, '🕘') : null,
      h('div', { class: 'kachel-nr' }, p.gartennr || p.mitgliedsnr), h('div', { class: 'kachel-name' }, p.name));
  });
  const legend = (cls, label) => h('span', { class: 'hint', style: 'margin-right:14px' },
    h('span', { class: 'kachel-dot kachel-' + cls }), ` ${label} (${counts[cls]})`);

  return h('div', null,
    h('h2', null, `Lageplan ${st.year}`),
    h('p', { class: 'hint' }, 'Kachel pro Garten, sortiert nach Gartennummer. Farbe zeigt den Abrechnungsstatus, Klick auf eine Kachel springt zur Schnelleingabe dieses Pächters.'),
    h('div', { style: 'margin-bottom:12px' }, legend('unvollstaendig', 'Angaben fehlen'), legend('berechnet', 'berechnet'), legend('offen', 'Rechnung offen'), legend('bezahlt', 'bezahlt')),
    list.length ? h('div', { class: 'kachelgrid' }, tiles) : h('div', { class: 'card empty' }, 'Noch keine Pächter angelegt.'));
}

function viewRechnungen() {
  const st = S.state;
  const sub = h('div', { class: 'subtabs' },
    h('button', { class: S.invTab !== 'archiv' ? 'active' : '', onclick: () => { S.invTab = 'pruefen'; render(); } }, 'Rechnungen prüfen & ausstellen'),
    h('button', { class: S.invTab === 'archiv' ? 'active' : '', onclick: () => { S.invTab = 'archiv'; render(); } }, `Archiv (${(st.archive || []).length})`));
  return h('div', null, h('h2', null, `Rechnungen ${st.year}`), sub, S.invTab === 'archiv' ? viewArchiv() : viewAusstellen());
}

async function loadArchivAlle() {
  if (S.archivAlleLoading) return;
  S.archivAlleLoading = true;
  try { S.archivAlle = await api('GET', '/api/archiv-alle'); } catch (e) { handleErr(e); }
  S.archivAlleLoading = false;
  render();
}

// Öffnet die gespeicherte Rechnungs-PDF im PDF-Programm des Rechners (dort gibt es den vollen Druckdialog).
async function imPdfProgrammOeffnen(id) {
  try { await api('POST', `/api/open-invoice/${id}`); toast('Die Rechnung wurde im PDF-Programm geöffnet – dort drucken.', 'ok'); }
  catch (e) { handleErr(e); }
}

// Alle Rechnungen mit Postversand als eine PDF-Datei zum Ausdrucken (nach Mitgliedsnummer sortiert).
async function postRechnungenDrucken(year) {
  try {
    if (inNativeWindow()) {
      const r = await api('POST', `/api/admin/rechnungen-druck/oeffnen?year=${year}`);
      toast(`${r.anzahl} Rechnung(en) im PDF-Programm geöffnet – dort drucken.`, 'ok');
      return;
    }
    const r = await fetch(`/api/admin/rechnungen-druck?year=${year}`, { headers: { 'X-GA-Request': '1' } });
    if (!r.ok) {
      const d = await r.json().catch(() => null);
      const err = new Error((d && d.error) || `Fehler ${r.status}`);
      err.status = r.status;
      throw err;
    }
    const url = URL.createObjectURL(await r.blob());
    openDocument(url, 'Rechnungen Postversand');
    setTimeout(() => URL.revokeObjectURL(url), 60000);
  } catch (e) { handleErr(e); }
}

function archivRow(a, mitJahr) {
  const url = `/api/archive/${a.id}.pdf`;
  return h('tr', { class: a.status === 'ersetzt' ? 'ersetzt' : '' },
    mitJahr ? h('td', { class: 'mid' }, a.jahr) : null,
    h('td', null, a.mitgliedsnr), h('td', null, a.name), h('td', { class: 'mono' }, a.nummer), h('td', { class: 'mid' }, a.version),
    h('td', { class: 'r' }, eur(a.gesamt)), h('td', null, fmtWhen(a.ausgestellt)),
    h('td', null, h('span', { class: 'badge ' + (a.status === 'gueltig' ? 'ok' : 'warn') }, a.status === 'gueltig' ? 'gültig' : 'ersetzt')),
    h('td', null, h('a', { class: 'btn small', href: url, target: '_blank' }, 'Ansehen'), ' ',
      h('a', { class: 'btn small', href: url, download: a.datei ? a.datei.split('/').pop() : 'Rechnung.pdf' }, 'Speichern'),
      inNativeWindow() && a.datei ? [' ', h('button', { class: 'btn small', title: 'Im PDF-Programm öffnen und dort drucken', onclick: () => imPdfProgrammOeffnen(a.id) }, 'Drucken')] : null));
}

function viewArchiv() {
  const st = S.state;
  const q = S.archivSuche.trim().toLowerCase();
  const suchModus = q.length > 0;
  if (suchModus && !S.archivAlle && !S.archivAlleLoading) loadArchivAlle();

  const suchfeld = h('input', { type: 'search', placeholder: 'Suche über alle Jahre (Nummer oder Name)', value: S.archivSuche, style: 'width:300px',
    oninput: (e) => { S.archivSuche = e.target.value; render(); } });

  let body;
  if (suchModus) {
    if (!S.archivAlle) {
      body = h('div', { class: 'card empty' }, 'Wird geladen …');
    } else {
      const treffer = S.archivAlle.filter((a) => [a.mitgliedsnr, a.name, a.nummer].some((x) => (x || '').toLowerCase().includes(q)));
      body = treffer.length
        ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
            h('thead', null, h('tr', null, ['Jahr', 'Nr.', 'Name', 'Rechnungs-Nr.', 'Version', 'Betrag', 'Ausgestellt am', 'Status', ''].map((t) => h('th', null, t)))),
            h('tbody', null, treffer.map((a) => archivRow(a, true)))))
        : h('div', { class: 'card empty' }, 'Keine Treffer.');
    }
  } else {
    const rows = st.archive || [];
    body = rows.length
      ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
          h('thead', null, h('tr', null, ['Nr.', 'Name', 'Rechnungs-Nr.', 'Version', 'Betrag', 'Ausgestellt am', 'Status', ''].map((t) => h('th', null, t)))),
          h('tbody', null, rows.map((a) => archivRow(a, false)))))
      : h('div', { class: 'card empty' }, 'In diesem Jahr wurde noch keine Rechnung ausgestellt.');
  }

  return h('div', null,
    h('p', { class: 'hint' }, `Hier liegen alle ausgestellten Rechnungen des Jahres ${st.year}. Sie sind mit den Preisen und Angaben von damals gespeichert und ändern sich nie, auch wenn du später Preise, Namen oder Zählerstände änderst. Wurde eine Rechnung neu ausgestellt, bleibt die alte als »ersetzt« erhalten.`),
    h('div', { class: 'toolbar' }, suchfeld, h('div', { class: 'spacer' }),
      h('button', { class: 'btn', onclick: () => api('POST', '/api/open-folder', { which: 'rechnungen', year: st.year }).catch(handleErr) }, 'Rechnungsordner öffnen')),
    body);
}

function viewAusstellen() {
  const st = S.state;
  const list = [...st.paechter].sort(cmpNr);
  if (!list.some((p) => p.id === S.selInvoice)) S.selInvoice = list.length ? list[0].id : null;
  const right = h('div');
  const issuedOf = (p) => (st.issued || {})[p.id];

  const drawPreview = () => {
    right.replaceChildren();
    const p = list.find((x) => x.id === S.selInvoice);
    if (!p) { right.append(h('div', { class: 'card empty' }, 'Noch keine Pächter vorhanden.')); return; }
    const res = st.results[p.id];
    const inf = issuedOf(p);
    const showLive = !inf || S.invView === 'live';
    const url = showLive ? `/api/invoice/${p.id}.pdf?year=${st.year}` : `/api/archive/${inf.id}.pdf`;
    const canShow = showLive ? res.vollstaendig : true;

    const bar = h('div', { class: 'toolbar' }, h('b', null, `${p.mitgliedsnr} – ${p.name}`), h('div', { class: 'spacer' }));
    if (!inf && res.vollstaendig) {
      bar.append(h('button', { class: 'btn primary', onclick: async () => { try { await issue([p.id], false); } catch (e) { handleErr(e); } } }, 'Rechnung ausstellen & speichern'));
    }
    if (canShow) {
      bar.append(h('a', { class: 'btn', href: url, target: '_blank' }, 'PDF öffnen / drucken'),
        h('a', { class: 'btn', href: url, download: `Rechnung_${p.mitgliedsnr}.pdf` }, 'PDF speichern'));
      if (inNativeWindow() && inf && !showLive) {
        bar.append(h('button', { class: 'btn', title: 'Im PDF-Programm des Rechners öffnen und dort drucken', onclick: () => imPdfProgrammOeffnen(inf.id) }, 'Im PDF-Programm öffnen'));
      }
    }
    right.append(bar);

    if (inf) {
      right.append(h('div', { class: 'banner info' },
        `Ausgestellt am ${fmtWhen(inf.ausgestellt)} · Rechnungs-Nr. ${inf.nummer}${inf.version > 1 ? ` (Version ${inf.version})` : ''} · ${eur(inf.gesamt)}. Diese Rechnung ist gespeichert und ändert sich nicht mehr.`));
      if (inf.geaendert) {
        right.append(h('div', { class: 'banner warn' },
          'Seit der Ausstellung wurden Werte geändert (Zählerstände, Preise oder Pächterdaten). Die ausgestellte Rechnung bleibt unverändert. ',
          h('button', { class: 'btn small', onclick: () => { S.invView = showLive ? 'archiv' : 'live'; drawPreview(); } }, showLive ? 'Ausgestellte Rechnung ansehen' : 'Mit aktuellen Werten ansehen'), ' ',
          res.vollstaendig ? h('button', { class: 'btn small', onclick: async () => {
            const ok = await confirmBox(`Für ${p.name} wird eine neue Rechnung mit den aktuellen Werten ausgestellt. Die bisherige bleibt als »ersetzt« im Archiv. Fortfahren?`, 'Neu ausstellen');
            if (ok) { try { S.invView = 'archiv'; await issue([p.id], true); } catch (e) { handleErr(e); } }
          } }, 'Neu ausstellen (ersetzt die alte)') : null));
      }
    }
    if (!canShow) {
      right.append(h('div', { class: 'banner warn' }, `Für diese Rechnung fehlt noch etwas: ${res.status}. Bitte im Tab »Zählerstände« ergänzen.`,
        ' ', h('button', { class: 'btn small', onclick: () => { S.tab = 'eingabe'; S.filter = p.mitgliedsnr; render(); } }, 'Zu den Zählerständen')));
      return;
    }
    right.append(h('div', { class: 'preview' }, h('iframe', { src: `${url}${showLive ? '&t=' + Date.now() : ''}#view=FitH`, title: 'Rechnungsvorschau' })));
  };

  const items = h('div', { class: 'card list' });
  const drawList = () => {
    items.replaceChildren();
    for (const p of list) {
      const ok = st.results[p.id] && st.results[p.id].vollstaendig;
      const inf = issuedOf(p);
      const cls = inf ? (inf.geaendert ? 'chg' : 'iss') : ok ? 'ok' : '';
      const tip = inf ? (inf.geaendert ? 'ausgestellt, danach geändert' : 'ausgestellt') : ok ? 'bereit zum Ausstellen' : 'unvollständig';
      items.append(h('button', { class: 'item' + (p.id === S.selInvoice ? ' active' : ''), onclick: () => { S.selInvoice = p.id; S.invView = 'archiv'; drawList(); drawPreview(); } },
        h('span', { class: 'dot ' + cls, title: tip }),
        h('span', { class: 'nr' }, p.mitgliedsnr), h('span', null, p.name)));
    }
    if (!list.length) items.append(h('div', { class: 'empty' }, 'Keine Pächter.'));
  };
  drawList();
  drawPreview();

  const open = list.filter((p) => st.results[p.id] && st.results[p.id].vollstaendig && !issuedOf(p));
  const nIss = list.filter((p) => issuedOf(p)).length;
  const allBtn = h('button', { class: 'btn primary', disabled: !open.length, onclick: async () => {
    const ok = await confirmBox(`${open.length} Rechnung(en) werden jetzt ausgestellt und als PDF gespeichert. Unvollständige Rechnungen werden übersprungen. Fortfahren?`, 'Ausstellen');
    if (!ok) return;
    allBtn.disabled = true;
    try { await issue([], false); } catch (e) { handleErr(e); }
    allBtn.disabled = false;
  } }, `Alle offenen ausstellen (${open.length})`);

  const nPost = list.filter((p) => issuedOf(p) && String(p.versand || '').toLowerCase() === 'postversand').length;
  const postBtn = h('button', { class: 'btn', disabled: !nPost,
    title: 'Alle ausgestellten Rechnungen mit Versandart Postversand als eine PDF-Datei, nach Mitgliedsnummer sortiert',
    onclick: () => postRechnungenDrucken(st.year) }, `Postversand drucken (${nPost})`);

  return h('div', null,
    h('p', { class: 'hint' }, 'Wähle links einen Pächter und prüfe die Rechnung. Mit »Ausstellen« wird sie festgeschrieben, als PDF gespeichert und ins Archiv gelegt. Punkte: grau = unvollständig, grün = bereit, blau = ausgestellt, orange = ausgestellt, danach geändert.'),
    h('div', { class: 'toolbar' }, allBtn, h('span', { class: 'hint' }, `${nIss} von ${list.length} ausgestellt`), h('div', { class: 'spacer' }), postBtn,
      h('button', { class: 'btn', onclick: () => api('POST', '/api/open-folder', { which: 'rechnungen', year: st.year }).catch(handleErr) }, 'Rechnungsordner öffnen')),
    h('div', { class: 'split' }, items, right));
}

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
const todayIso = () => { const d = new Date(); const p = (n) => String(n).padStart(2, '0'); return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`; };
const payTotal = (e) => Math.abs(e.gesamt);
const payPaid = (e) => (e.bezahltAm ? (e.bezahltBetrag != null ? e.bezahltBetrag : payTotal(e)) : 0);
const payOpen = (e) => Math.round((payTotal(e) - payPaid(e)) * 100) / 100;
const payIsOpen = (e) => payOpen(e) > 0.004;

async function savePayment(e, patch) {
  const body = { bezahltAm: e.bezahltAm || '', bezahltBetrag: e.bezahltBetrag ?? null, notiz: e.notiz || '', ...patch };
  await api('PUT', `/api/payment/${e.id}`, body);
  Object.assign(e, { bezahltAm: body.bezahltAm, bezahltBetrag: body.bezahltAm ? body.bezahltBetrag : null, notiz: body.notiz });
  // volle Zahlung: Betrag nicht separat führen
  if (e.bezahltBetrag != null && Math.round(e.bezahltBetrag * 100) === Math.round(payTotal(e) * 100)) e.bezahltBetrag = null;
}

async function paymentDialog(e) {
  const date = h('input', { type: 'date', value: e.bezahltAm || todayIso() });
  const amt = h('input', { type: 'text', inputmode: 'decimal', value: numIn(e.bezahltAm ? payPaid(e) : payTotal(e)), style: 'width:120px' });
  const note = h('input', { type: 'text', maxlength: 200, value: e.notiz || '', style: 'width:100%' });
  const body = h('div', { class: 'form' },
    h('p', null, `${e.mitgliedsnr} – ${e.name}: Rechnungsbetrag ${eur(payTotal(e))}${e.gesamt < 0 ? ' (Guthaben, wird an den Pächter ausgezahlt)' : ''}`),
    h('div', { class: 'field' }, h('label', null, 'Bezahlt am'), date),
    h('div', { class: 'field' }, h('label', null, 'Bezahlter Betrag (bei Teilzahlung anpassen)'), h('div', { class: 'inputunit' }, amt, h('span', null, '€'))),
    h('div', { class: 'field wide' }, h('label', null, 'Notiz (z. B. bar, Überweisung, Ratenzahlung)'), note));
  return modal('Zahlung vermerken', body, [
    { label: 'Abbrechen', value: false },
    e.bezahltAm ? { label: 'Zahlung zurücknehmen', cls: 'danger', value: 'undo', action: async () => { try { await savePayment(e, { bezahltAm: '', bezahltBetrag: null }); } catch (er) { handleErr(er); return false; } } } : null,
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const v = parseNum(amt.value);
      if (!date.value) { toast('Bitte ein Datum angeben.', 'err'); return false; }
      if (v === null || Number.isNaN(v) || v < 0) { toast('Bitte einen gültigen Betrag angeben.', 'err'); return false; }
      try { await savePayment(e, { bezahltAm: date.value, bezahltBetrag: v, notiz: note.value.trim() }); } catch (er) { handleErr(er); return false; }
    } },
  ].filter(Boolean));
}

async function mahnungErzeugen(ids) {
  if (!ids.length) { toast('Bitte mindestens eine offene Rechnung auswählen.', 'err'); return; }
  try {
    const r = await fetch('/api/admin/mahnung', { method: 'POST', headers: { 'X-GA-Request': '1', 'Content-Type': 'application/json' }, body: JSON.stringify({ ids }) });
    if (!r.ok) { const d = await r.json().catch(() => null); throw new Error((d && d.error) || `Fehler ${r.status}`); }
    const blob = await r.blob();
    const url = URL.createObjectURL(blob);
    if ((r.headers.get('content-type') || '').includes('pdf')) openDocument(url, 'Mahnung');
    else { const a = h('a', { href: url, download: 'Mahnungen.zip' }); document.body.append(a); a.click(); a.remove(); }
    setTimeout(() => URL.revokeObjectURL(url), 30000);
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
    const card = (t, big, small, cls) => h('div', { class: 'card stat ' + (cls || '') }, h('div', { class: 'hint' }, t), h('div', { class: 'big' }, big), h('div', { class: 'hint' }, small));
    summary.replaceChildren(...[
      card('Forderungen gesamt', eur(sum(claims, payTotal)), `${claims.length} Rechnungen`),
      card('Bereits bezahlt', eur(sum(claims, payPaid)), `${claims.filter((e) => !payIsOpen(e)).length} vollständig bezahlt`, 'ok'),
      card('Noch offen', eur(sum(openClaims, payOpen)), `${openClaims.length} Rechnungen`, openClaims.length ? 'warn' : 'ok'),
      card('Überfällig', String(overdue.length), overdue.length ? eur(sum(overdue, payOpen)) : 'nichts überfällig', overdue.length ? 'err' : 'ok'),
      credits.length ? card('Guthaben auszuzahlen', eur(sum(credits.filter(payIsOpen), payOpen)), `${credits.filter(payIsOpen).length} von ${credits.length} noch offen`) : null].filter(Boolean));

    mahnBar.replaceChildren(
      h('button', { class: 'btn', disabled: !S.mahnSel.size, onclick: () => mahnungErzeugen([...S.mahnSel]) },
        `Mahnung erzeugen${S.mahnSel.size ? ` (${S.mahnSel.size})` : ''}`),
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
      const mahnbar = open && e.gesamt >= 0;
      const mchk = h('input', { type: 'checkbox', checked: S.mahnSel.has(e.id), disabled: !mahnbar, title: mahnbar ? 'für Mahnung auswählen' : 'nicht offen oder ein Guthaben',
        onchange: (ev) => { if (ev.target.checked) S.mahnSel.add(e.id); else S.mahnSel.delete(e.id); draw(); } });
      tbody.append(h('tr', { class: late ? 'late' : '' },
        h('td', null, mchk),
        h('td', null, e.mitgliedsnr), h('td', null, e.name), h('td', { class: 'mono' }, e.nummer),
        h('td', { class: 'r' }, (e.gesamt < 0 ? '−' : '') + eur(payTotal(e))),
        h('td', null, deDate(e.faellig)),
        h('td', null, h('span', { class: 'badge ' + (partial ? 'warn' : !open ? 'ok' : late ? 'err' : 'neu') },
          partial ? `Teilzahlung, offen ${eur(payOpen(e))}` : !open ? (e.gesamt < 0 ? 'ausgezahlt' : 'bezahlt') : late ? 'überfällig' : e.gesamt < 0 ? 'Guthaben offen' : 'offen')),
        h('td', { class: 'paycell' }, chk, date),
        h('td', { class: 'note', title: e.notiz || '' }, e.notiz || ''),
        h('td', null, h('button', { class: 'btn small', onclick: async () => { const r = await paymentDialog(e); if (r !== false && r !== undefined) draw(); } }, 'Details'))));
    }
    if (!shown.length) tbody.append(h('tr', null, h('td', { colspan: 10, class: 'empty' },
      !all.length ? `Für ${st.year} wurde noch keine Rechnung ausgestellt. Das machst du im Tab »Rechnungen«.` : S.payFilter === 'offen' && !q ? 'Alles bezahlt – nichts mehr offen.' : 'Keine Treffer.')));
  };
  draw();

  const notIssued = st.paechter.length - Object.keys(st.issued || {}).length;
  return h('div', null,
    h('h2', null, `Zahlungen ${st.year}`),
    h('p', { class: 'hint' }, 'Hier siehst du, welche Rechnungen bezahlt sind und welche noch offen. Haken setzen = bezahlt heute (das Datum kannst du ändern). Über »Details« trägst du Teilzahlungen oder eine Notiz ein. Rechnungen mit Guthaben erscheinen hier auch, damit du siehst, was noch an Pächter auszuzahlen ist.'),
    notIssued > 0 ? h('div', { class: 'banner info' }, `${notIssued} Pächter haben in ${st.year} noch keine ausgestellte Rechnung und tauchen deshalb hier noch nicht auf.`) : null,
    summary,
    h('div', { class: 'toolbar' },
      h('input', { type: 'search', placeholder: 'Suchen (Nummer oder Name)', value: S.payQ, style: 'width:240px', oninput: (e) => { S.payQ = e.target.value; draw(); } }),
      h('div', { class: 'spacer' }),
      h('a', { class: 'btn', href: `/api/export-payments?year=${st.year}&nur=offen` }, 'Offene Posten als Excel'),
      h('a', { class: 'btn', href: `/api/export-payments?year=${st.year}` }, 'Alle Zahlungen als Excel'),
      h('a', { class: 'btn', href: `/api/export-ics?year=${st.year}`, title: 'Fälligkeitstermine der offenen Rechnungen zum Import in den eigenen Kalender' }, 'Fälligkeiten als Kalender (.ics)')),
    filters,
    mahnBar,
    h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['', 'Nr.', 'Name', 'Rechnungs-Nr.', 'Betrag', 'Fällig am', 'Status', 'Bezahlt / am', 'Notiz', ''].map((t) => h('th', null, t)))),
      tbody)));
}

// ------------------------------------------------------------------ Tab: Admin

function viewAdmin() {
  const st = S.state;
  if (st.hasPassword && !st.loggedIn) return viewLogin();
  const subs = [['paechter', 'Pächter'], ['einstellungen', 'Preise & Einstellungen'], ['jahreswechsel', 'Jahreswechsel'],
    ['kassenbericht', 'Kassenbericht'], ['jubilaeen', 'Jubiläen'], ['daten', 'Import / Export / Sicherung'], ['protokoll', 'Änderungsprotokoll'], ['datenschutz', 'Datenschutz'], ['sicherheit', 'Passwort']];
  const body = S.adminTab === 'paechter' ? adminPaechter() : S.adminTab === 'einstellungen' ? adminSettings()
    : S.adminTab === 'jahreswechsel' ? adminYear() : S.adminTab === 'kassenbericht' ? adminKassenbericht(false)
      : S.adminTab === 'jubilaeen' ? adminJubilaeen() : S.adminTab === 'daten' ? adminData() : S.adminTab === 'protokoll' ? adminProtokoll() : S.adminTab === 'datenschutz' ? adminDatenschutz() : adminSecurity();
  return h('div', null,
    h('h2', null, 'Admin-Bereich'),
    !st.hasPassword ? h('div', { class: 'banner warn' }, 'Für den Admin-Bereich ist noch kein Passwort gesetzt – jeder kann hier Pächter und Preise ändern. ',
      h('button', { class: 'btn small', onclick: () => { S.adminTab = 'sicherheit'; render(); } }, 'Passwort festlegen')) : null,
    h('div', { class: 'subtabs' }, subs.map(([id, label]) => h('button', { class: S.adminTab === id ? 'active' : '',
      onclick: () => { S.adminTab = id; S.importPreview = null; if (id === 'protokoll') S.protokoll = null; if (id === 'jubilaeen') S.jubilaeen = null; if (id === 'datenschutz') S.datenschutz = null; render(); } }, label))),
    body);
}

function viewLogin() {
  const st = S.state;
  const pw = h('input', { type: 'password', autocomplete: 'current-password', style: 'width:100%', autofocus: true });
  const go = async () => {
    try { await api('POST', '/api/admin/login', { password: pw.value }); await reload(); } catch (e) { handleErr(e); pw.select(); }
  };
  pw.addEventListener('keydown', (e) => { if (e.key === 'Enter') go(); });
  setTimeout(() => pw.focus(), 50);
  return h('div', { class: 'center card' }, h('h2', null, 'Admin-Anmeldung'),
    h('p', { class: 'hint' }, 'Der Admin-Bereich ist durch ein Passwort geschützt.'), pw,
    h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: go }, 'Anmelden')),
    st.hasPruefPassword ? h('p', { style: 'margin-top:18px' },
      h('button', { class: 'btn', onclick: () => { S.pruefLoginMode = true; render(); } }, 'Als Kassenprüfer anmelden')) : null);
}

function viewPruefLogin() {
  const pw = h('input', { type: 'password', autocomplete: 'current-password', style: 'width:100%', autofocus: true });
  const go = async () => {
    try { await api('POST', '/api/pruef/login', { password: pw.value }); S.pruefLoginMode = false; await reload(); } catch (e) { handleErr(e); pw.select(); }
  };
  pw.addEventListener('keydown', (e) => { if (e.key === 'Enter') go(); });
  setTimeout(() => pw.focus(), 50);
  return h('div', { class: 'center card' }, h('h2', null, 'Kassenprüfer-Anmeldung'),
    h('p', { class: 'hint' }, 'Lesezugriff auf den Kassenbericht, ohne die übrigen Admin-Rechte.'), pw,
    h('div', { class: 'actions' },
      h('button', { class: 'btn primary', onclick: go }, 'Anmelden'),
      h('button', { class: 'btn', onclick: () => { S.pruefLoginMode = false; render(); } }, 'Zurück')));
}

// Eigenständige, schmale Oberfläche für den Kassenprüfer-Zugang: nur Lesezugriff
// auf den Kassenbericht (inkl. Geprüft-Haken), kein Zugriff auf die übrigen Tabs.
function viewPruefShell() {
  const st = S.state;
  if (S.kasseYear == null) S.kasseYear = st.year;
  const app = h('div');
  const yearSel = h('select', { onchange: async (e) => { S.year = Number(e.target.value); S.kasse = null; S.kasseYear = S.year; await reload(); } },
    st.years.map((y) => h('option', { value: y, selected: y === st.year }, y === st.currentYear ? `${y} (aktuell)` : `${y} (abgeschlossen)`)));
  app.append(
    h('header', { class: 'top' },
      h('div', { class: 'brand' }, h('b', null, 'Kleingarten-Manager'), h('span', null, st.settings.vereinName + ' · Kassenprüfer-Zugang')),
      h('div', { class: 'spacer' }),
      h('div', null, h('label', null, 'Jahr'), yearSel),
      h('button', { class: 'btn', onclick: async () => { try { await api('POST', '/api/pruef/logout'); } catch (e) { /* egal */ } await reload(); } }, 'Abmelden')),
    h('main', null, h('h2', null, 'Kassenbericht'), adminKassenbericht(true)));
  return app;
}

// ---- Pächter verwalten

function paechterDialog(p) {
  const isNew = !p;
  const cur = p || { mitgliedsnr: '', gartennr: '', anrede: 'Herr', name: '', strasse: '', plzOrt: S.state.settings.ort, versand: 'Emailsendung',
    gartengroesse: 0, umlageAbweichend: null, wasserzaehlerNr: '', stromzaehlerNr: '', notiz: '' };
  const t = (val, ph) => h('input', { type: 'text', value: val || '', placeholder: ph || '', maxlength: 100, autocomplete: 'off' });
  const f = {
    mitgliedsnr: t(cur.mitgliedsnr, 'z. B. 35-95'), gartennr: t(cur.gartennr, 'z. B. 35'),
    anrede: h('select', { value: cur.anrede }, ['Herr', 'Frau', 'Familie', 'Herr und Frau', ''].map((a) => h('option', { value: a }, a || '(keine)'))),
    name: t(cur.name), strasse: t(cur.strasse), plzOrt: t(cur.plzOrt),
    versand: h('select', { value: cur.versand }, ['Emailsendung', 'Postversand'].map((a) => h('option', { value: a }, a))),
    gartengroesse: t(numIn(cur.gartengroesse || null)),
    umlage: t(numIn(cur.umlageAbweichend), `leer = Standard (${eur(S.state.settings.umlageStandard)})`),
    wz: t(cur.wasserzaehlerNr), sz: t(cur.stromzaehlerNr),
    notiz: h('textarea', { rows: 2, maxlength: 300, style: 'width:100%' }, cur.notiz || ''),
  };
  f.gartengroesse.setAttribute('inputmode', 'decimal');
  f.umlage.setAttribute('inputmode', 'decimal');
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const body = h('div', { class: 'form' },
    fld('Mitgliedsnummer *', f.mitgliedsnr), fld('Gartennummer', f.gartennr), fld('Anrede', f.anrede),
    fld('Name *', f.name), fld('Straße und Hausnummer', f.strasse), fld('PLZ und Ort', f.plzOrt),
    fld('Versandart', f.versand, 'Steht oben rechts auf der Rechnung'),
    fld('Gartengröße in m² *', f.gartengroesse), fld('Umlage abweichend in €', f.umlage, 'Nur ausfüllen, wenn dieser Pächter nicht die Standard-Umlage zahlt'),
    fld('Wasserzähler-Nr.', f.wz), fld('Stromzähler-Nr.', f.sz),
    fld('Notiz (nur intern, steht nicht auf der Rechnung – bitte keine Gesundheitsdaten oder sonstige sensible Angaben)', f.notiz, null, true));
  return modal(isNew ? 'Pächter anlegen' : `Pächter bearbeiten – ${cur.name}`, body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const gg = parseNum(f.gartengroesse.value), um = parseNum(f.umlage.value);
      if (Number.isNaN(gg) || Number.isNaN(um) || (gg !== null && gg < 0) || (um !== null && um < 0)) { toast('Gartengröße und Umlage müssen Zahlen ab 0 sein.', 'err'); return false; }
      const data = { mitgliedsnr: f.mitgliedsnr.value, gartennr: f.gartennr.value, anrede: f.anrede.value, name: f.name.value,
        strasse: f.strasse.value, plzOrt: f.plzOrt.value, versand: f.versand.value, gartengroesse: gg ?? 0, umlageAbweichend: um,
        wasserzaehlerNr: f.wz.value, stromzaehlerNr: f.sz.value, notiz: f.notiz.value.trim() };
      try {
        if (isNew) await api('POST', '/api/admin/paechter', data);
        else await api('PUT', `/api/admin/paechter/${cur.id}`, data);
        toast('Gespeichert', 'ok');
      } catch (e) { handleErr(e); return false; }
    } },
  ]);
}

function adminPaechter() {
  const st = S.state;
  const list = [...st.paechter].sort(cmpNr);
  const tbody = h('tbody');
  const fill = () => {
    tbody.replaceChildren();
    const q = S.filter.trim().toLowerCase();
    const shown = list.filter((p) => !q || [p.mitgliedsnr, p.name, p.gartennr, p.strasse].some((x) => (x || '').toLowerCase().includes(q)));
    for (const p of shown) {
      tbody.append(h('tr', null,
        h('td', null, p.mitgliedsnr), h('td', null, p.gartennr),
        h('td', { class: 'name', title: p.name }, p.name, p.notiz ? h('span', { title: p.notiz, style: 'margin-left:4px;cursor:help' }, '📝') : null),
        h('td', null, [p.strasse, p.plzOrt].filter(Boolean).join(', ')),
        h('td', { class: 'num' }, nfFlex.format(p.gartengroesse) + ' m²'),
        h('td', { class: 'num' }, p.umlageAbweichend == null ? 'Standard' : eur(p.umlageAbweichend)),
        h('td', null, p.wasserzaehlerNr), h('td', null, p.stromzaehlerNr),
        h('td', null,
          h('button', { class: 'btn small', onclick: async () => { if (await paechterDialog(p)) await reload(); } }, 'Bearbeiten'), ' ',
          auskunftLink(p), ' ',
          h('button', { class: 'btn small danger', onclick: async () => {
            const ok = await confirmBox(`Pächter „${p.name}“ (${p.mitgliedsnr}) in den Papierkorb legen? Er verschwindet aus allen Ansichten, Zählerstände und Rechnungen bleiben aber erhalten und lassen sich im Papierkorb wiederherstellen.`, 'In den Papierkorb', true);
            if (!ok) return;
            try { await api('DELETE', `/api/admin/paechter/${p.id}`); toast('In den Papierkorb gelegt', 'ok'); S.papierkorb = null; await reload(); } catch (e) { handleErr(e); }
          } }, 'Löschen'))));
    }
    if (!shown.length) tbody.append(h('tr', null, h('td', { colspan: 9, class: 'empty' }, list.length ? 'Keine Treffer.' : 'Noch keine Pächter. Lege den ersten mit »Pächter anlegen« an oder importiere eine Liste.')));
  };
  fill();
  return h('div', null,
    h('p', { class: 'hint' }, 'Hier pflegst du die Stammdaten der Pächter. Sie ändern sich selten und müssen nur einmal angelegt werden.'),
    h('div', { class: 'toolbar' },
      h('button', { class: 'btn primary', onclick: async () => { if (await paechterDialog(null)) await reload(); } }, '+ Pächter anlegen'),
      h('input', { type: 'search', placeholder: 'Suchen', value: S.filter, style: 'width:220px', oninput: (e) => { S.filter = e.target.value; fill(); } }),
      h('span', { class: 'stat' }, `${list.length} Pächter`)),
    h('div', { class: 'tablewrap' }, h('table', null,
      h('thead', null, h('tr', null, ['Mitgl.-Nr.', 'Garten', 'Name', 'Anschrift', 'Größe', 'Umlage', 'Wasserzähler', 'Stromzähler', ''].map((x) => h('th', null, x)))), tbody)),
    papierkorbCard());
}

function auskunftLink(p) {
  return h('a', { class: 'btn small', href: `/api/admin/paechter/${p.id}/auskunft`,
    title: 'Alle gespeicherten Daten dieser Person als Datei herunterladen (Auskunft nach Art. 15 DSGVO)' }, 'Auskunft');
}

// ---- Datenschutz

async function loadDatenschutz() {
  if (S.datenschutzLoading) return;
  S.datenschutzLoading = true;
  try { S.datenschutz = await api('GET', '/api/admin/datenschutz'); } catch (e) { handleErr(e); S.datenschutz = false; }
  S.datenschutzLoading = false;
  render();
}

function adminDatenschutz() {
  const st = S.state;
  if (S.datenschutz == null) { if (!S.datenschutzLoading) loadDatenschutz(); }
  const d = S.datenschutz || null;
  const list = [...st.paechter].sort(cmpNr);
  const sel = h('select', { style: 'min-width:260px' }, list.map((p) => h('option', { value: p.id }, `${p.mitgliedsnr}  ${p.name}`)));
  const bereinigen = async () => {
    const ok = await confirmBox(`Bei ${d.faelligRechnungen} Rechnung(en) und ${d.faelligJahre} Jahresunterlage(n) ist die Aufbewahrungsfrist (${d.aufbewahrungJahre} Jahre) abgelaufen. Name und Anschrift werden dort entfernt, die PDF-Dateien gelöscht, auch in den Sicherungen. Die Beträge bleiben für die Statistik. Das kann nicht rückgängig gemacht werden.`, 'Jetzt bereinigen', true);
    if (!ok) return;
    try {
      const r = await api('POST', '/api/admin/datenschutz/bereinigen');
      toast(`Bereinigt: ${r.rechnungen} Rechnung(en), ${r.jahre} Jahresunterlage(n), ${r.dateien} PDF-Datei(en), ${r.sicherungen} Sicherung(en)`, 'ok');
      S.datenschutz = null; S.archivAlle = null; await reload();
    } catch (e) { handleErr(e); }
  };
  const li = (...kids) => h('li', null, ...kids);
  return h('div', null,
    h('p', { class: 'hint' }, 'Hilfen für den Umgang mit personenbezogenen Daten nach der DSGVO. Für die Einhaltung ist der Verein verantwortlich; das Programm unterstützt dabei, ersetzt aber keine Rechtsberatung.'),

    h('div', { class: 'card', style: 'margin-bottom:14px' },
      h('h3', { style: 'margin-top:0' }, 'Was wird wo gespeichert?'),
      h('ul', { style: 'margin:0;padding-left:20px;line-height:1.6' },
        li(h('b', null, 'Alles liegt nur auf diesem Rechner'), ' (Datenordner: ', h('span', { class: 'mono' }, st.dataDir), '). Es gibt keine Cloud und keine Übertragung ins Internet.'),
        li('Pächter: Name, Anschrift, Mitgliedsnummer, Garten, Zählernummern, interne Notiz; dazu Zählerstände, Arbeitsstunden, Rechnungen, Zahlungen.'),
        li('Kopien davon stecken in ausgestellten Rechnungen (Archiv und PDF-Dateien), abgeschlossenen Jahren, der Garten-Historie, dem Änderungsprotokoll und den automatischen Sicherungen.'),
        li('Einzige Verbindung nach außen: der Knopf »Nach Updates suchen« (nur auf Klick, überträgt dabei die IP-Adresse an GitHub).'))),

    h('div', { class: 'card', style: 'margin-bottom:14px' },
      h('h3', { style: 'margin-top:0' }, 'Auskunft und Datenkopie (Art. 15 und 20 DSGVO)'),
      h('p', { class: 'hint' }, 'Wer wissen möchte, was über sie oder ihn gespeichert ist, bekommt hier alle Daten als Datei (maschinenlesbar, JSON). Dieselbe Datei gibt es auch je Person in der Pächter-Liste (»Auskunft«) und im Papierkorb – am besten erstellst du sie, bevor du jemanden löschst.'),
      list.length
        ? h('div', { class: 'toolbar' }, sel, h('a', { class: 'btn primary', href: '#', onclick: (e) => { e.preventDefault(); location.href = `/api/admin/paechter/${sel.value}/auskunft`; } }, 'Auskunft herunterladen'))
        : h('p', { class: 'hint' }, 'Noch keine Pächter angelegt.')),

    h('div', { class: 'card', style: 'margin-bottom:14px' },
      h('h3', { style: 'margin-top:0' }, 'Löschen und Aufbewahrungsfristen (Art. 17 DSGVO)'),
      h('p', { class: 'hint' }, 'Ein Pächter, der nicht mehr dabei ist, kommt zuerst in den Papierkorb. Dort kannst du ihn »endgültig löschen«: Dabei werden Stammdaten, Notizen, offene Zählerstände und sein Name im Änderungsprotokoll, in der Garten-Historie und in allen Sicherungen entfernt. Rechnungen und Jahresunterlagen müssen aus steuerlichen Gründen aber noch eine Zeit lang bleiben. Diese Frist beträgt hier ' + (d ? d.aufbewahrungJahre : 10) + ' Jahre ab Ende des Ausstellungsjahres (die längere der üblichen Fristen; ob bei euch 8 oder 10 Jahre genügen, klärt ihr mit Steuerberatung oder Kassenprüfern). Danach entfernst du hier den Personenbezug.'),
      !d ? h('p', { class: 'hint' }, d === false ? 'Konnte nicht geladen werden.' : 'Wird geladen …') : h('div', null,
        h('ul', { style: 'margin:0 0 10px;padding-left:20px;line-height:1.6' },
          li(`Im Papierkorb wartend: ${d.papierkorb} Pächter`),
          li(d.faelligRechnungen + d.faelligJahre > 0
            ? h('b', null, `Aufbewahrungsfrist abgelaufen: ${d.faelligRechnungen} Rechnung(en), ${d.faelligJahre} Jahresunterlage(n) (ausgestellt bis ${d.bereinigtBisJahr})`)
            : 'Keine Unterlagen mit abgelaufener Aufbewahrungsfrist.'),
          d.naechsteFrist ? li(`Die nächsten Rechnungen werden ${d.naechsteFrist} freigegeben.`) : null),
        h('button', { class: 'btn danger', disabled: d.faelligRechnungen + d.faelligJahre === 0, onclick: bereinigen }, 'Abgelaufene Unterlagen bereinigen'))),

    h('div', { class: 'card' },
      h('h3', { style: 'margin-top:0' }, 'Was der Verein selbst erledigen muss'),
      h('ul', { style: 'margin:0;padding-left:20px;line-height:1.6' },
        li('Verzeichnis der Verarbeitungstätigkeiten führen und die Mitglieder über die Verarbeitung informieren (Vorlagen: Datei ', h('span', { class: 'mono' }, 'DATENSCHUTZ.md'), ' im Projekt).'),
        li('Admin-Passwort setzen (Reiter »Passwort«) und die Festplatte verschlüsseln (z. B. BitLocker, FileVault); die Datendatei selbst ist nicht verschlüsselt.'),
        li('Sicherungen (auch den zweiten Sicherungsordner, USB-Stick) sicher aufbewahren; sie enthalten dieselben Daten.'),
        li('Nur nötige Angaben erfassen, keine Gesundheitsdaten oder sonstigen sensiblen Angaben in Notizfeldern.'))));
}

async function loadPapierkorb() {
  if (S.papierkorbLoading) return;
  S.papierkorbLoading = true;
  try { S.papierkorb = await api('GET', '/api/admin/paechter-papierkorb'); } catch (e) { handleErr(e); }
  S.papierkorbLoading = false;
  render();
}

function papierkorbCard() {
  if (!S.papierkorbOffen) {
    return h('div', { style: 'margin-top:14px' },
      h('button', { class: 'btn', onclick: () => { S.papierkorbOffen = true; S.papierkorb = null; render(); } }, '🗑 Papierkorb'));
  }
  if (!S.papierkorb) {
    if (!S.papierkorbLoading) loadPapierkorb();
    return h('div', { class: 'card', style: 'margin-top:14px' }, h('p', { class: 'hint', style: 'margin:0' }, 'Wird geladen …'));
  }
  const rows = S.papierkorb.map((p) => h('tr', null,
    h('td', null, p.mitgliedsnr), h('td', null, p.name), h('td', null, deDate(p.geloeschtAm)),
    h('td', null,
      h('button', { class: 'btn small primary', onclick: async () => {
        try { await api('POST', `/api/admin/paechter/${p.id}/wiederherstellen`); toast('Wiederhergestellt', 'ok'); S.papierkorb = null; await reload(); }
        catch (e) { handleErr(e); }
      } }, 'Wiederherstellen'), ' ',
      auskunftLink(p), ' ',
      h('button', { class: 'btn small danger', onclick: async () => {
        const ok = await confirmBox(`Pächter „${p.name}“ (${p.mitgliedsnr}) endgültig löschen? Das kann nicht rückgängig gemacht werden. Stammdaten, Notizen, offene Zählerstände sowie der Name im Änderungsprotokoll, in der Garten-Historie und in allen Sicherungen werden entfernt. Bereits ausgestellte Rechnungen müssen wegen der gesetzlichen Aufbewahrungspflicht noch 10 Jahre (ab Ende des Ausstellungsjahres) bleiben und verlieren ihren Namen erst danach unter Admin → Datenschutz.`, 'Endgültig löschen', true);
        if (!ok) return;
        try { await api('DELETE', `/api/admin/paechter/${p.id}/endgueltig`); toast('Endgültig gelöscht', 'ok'); S.papierkorb = null; render(); await loadPapierkorb(); }
        catch (e) { handleErr(e); }
      } }, 'Endgültig löschen'))));
  return h('div', { class: 'card', style: 'margin-top:14px' },
    h('div', { class: 'toolbar' }, h('b', null, 'Papierkorb'), h('div', { class: 'spacer' }),
      h('button', { class: 'btn small', onclick: () => { S.papierkorbOffen = false; render(); } }, 'Schließen')),
    S.papierkorb.length
      ? h('div', { class: 'tablewrap', style: 'margin-top:10px' }, h('table', { class: 'data' },
          h('thead', null, h('tr', null, ['Mitgl.-Nr.', 'Name', 'Gelöscht am', ''].map((t) => h('th', null, t)))),
          h('tbody', null, rows)))
      : h('p', { class: 'hint', style: 'margin:10px 0 0' }, 'Papierkorb ist leer.'));
}

// ---- Einstellungen

const SETTINGS_FORM = [
  ['Verein und Rechnung', [
    ['vereinName', 'Vereinsname', 'text', '', true], ['ort', 'PLZ und Ort', 'text'], ['absenderzeile', 'Absenderzeile (kleine Zeile über der Anschrift)', 'text', '', true],
    ['rechnungsdatum', 'Rechnungsdatum', 'date'], ['zahlungsziel', 'Zahlbar bis', 'date'],
    ['rechnungsnrPraefix', 'Rechnungsnummer-Vorsatz', 'text', 'Rechnungsnr. = Vorsatz + Mitgliedsnr., z. B. 100-35-95'],
    ['einspruchTage', 'Einspruchsfrist', 'num', 'Tage']]],
  ['Bankverbindung', [['bankName', 'Bank', 'text'], ['iban', 'IBAN', 'text'], ['bic', 'BIC', 'text']]],
  ['Wasser', [['wasserGrundpreis', 'Grundpreis pro Jahr', 'num', '€'], ['wasserPreis', 'Preis je m³', 'num', '€']]],
  ['Strom / Energie', [['energieGrundpreis', 'Grundpreis pro Jahr', 'num', '€'], ['energiePreis', 'Preis je kWh', 'num', '€']]],
  ['Arbeitsstunden', [
    ['pflichtstunden', 'Pflichtstunden', 'num', 'Stunden'], ['stundenObergrenze', 'Vergütung bis zu dieser Stundenzahl', 'num', 'Stunden'],
    ['verguetungJeStd', 'Vergütung je Stunde über der Pflicht', 'num', '€'], ['nachzahlungJeStd', 'Nachzahlung je fehlende Stunde', 'num', '€']]],
  ['Pacht und Beiträge (für das Folgejahr)', [
    ['pachtJeQm', 'Pacht je m²', 'num', '€'], ['vereinsflaecheQm', 'Anteil Vereinsfläche', 'num', 'm²'], ['freieGaertenQm', 'Pachtanteil freie Gärten', 'num', 'm²'],
    ['vereinsbeitrag', 'Vereinsbeitrag', 'num', '€'], ['territorialverband', 'Territorialverband', 'num', '€'], ['umlageStandard', 'Umlage (Standard für alle)', 'num', '€']]],
];

function adminSettings() {
  const st = S.state;
  const inputs = {};
  const form = h('div', { class: 'form' });
  for (const [title, fields] of SETTINGS_FORM) {
    form.append(h('div', { class: 'section-title' }, title));
    for (const [key, label, type, hint, wide] of fields) {
      const val = st.settings[key];
      const input = type === 'date' ? h('input', { type: 'date', value: val })
        : type === 'num' ? h('input', { type: 'text', inputmode: 'decimal', value: numIn(val), autocomplete: 'off' })
          : h('input', { type: 'text', value: val, maxlength: 250, autocomplete: 'off' });
      inputs[key] = { input, type };
      const unit = type === 'num' ? hint : null;
      form.append(h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label),
        unit ? h('div', { class: 'inputunit' }, input, h('span', null, unit)) : input,
        type !== 'num' && hint ? h('span', { class: 'fhint' }, hint) : null));
    }
  }
  const save = async () => {
    const data = { jahr: st.settings.jahr };
    for (const [key, { input, type }] of Object.entries(inputs)) {
      if (type === 'num') {
        const v = parseNum(input.value);
        if (v === null || Number.isNaN(v) || v < 0) { input.classList.add('err'); toast('Bitte in allen Zahlenfeldern eine Zahl ab 0 eintragen.', 'err'); return; }
        input.classList.remove('err');
        data[key] = v;
      } else data[key] = input.value;
    }
    data.einspruchTage = Math.round(data.einspruchTage);
    try { await api('PUT', '/api/admin/settings', data); toast('Einstellungen gespeichert', 'ok'); await reload(); } catch (e) { handleErr(e); }
  };
  return h('div', null,
    h('p', { class: 'hint' }, `Diese Werte gelten für das laufende Abrechnungsjahr ${st.settings.jahr} und für alle Rechnungen. Nach einer Änderung rechnen alle Beträge automatisch neu.`),
    h('div', { class: 'card' }, form, h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: save }, 'Speichern'))));
}

// ---- Jahreswechsel

async function loadAbschlussCheck(year) {
  if (S.abschlussLoading) return;
  S.abschlussLoading = true;
  try { S.abschlussCheck = await api('GET', `/api/admin/kassenbericht?year=${year}`); } catch (e) { S.abschlussCheck = false; }
  S.abschlussLoading = false;
  render();
}

// Geführter Überblick vor dem Jahreswechsel: zeigt, was noch offen ist. Ist
// nur ein Hinweis, blockiert den Jahreswechsel nicht (wie die übrigen
// Plausibilitätshinweise im Programm auch).
function abschlussChecklist(st) {
  if (S.abschlussCheck == null || (S.abschlussCheck && S.abschlussCheck.jahr !== st.currentYear)) {
    if (!S.abschlussLoading) loadAbschlussCheck(st.currentYear);
  }
  const total = st.paechter.length;
  const done = st.paechter.filter((p) => st.results[p.id] && st.results[p.id].vollstaendig).length;
  const issued = Object.keys(st.issued || {}).length;
  const offenePosten = (st.archive || []).filter((e) => e.status === 'gueltig' && e.gesamt >= 0 && payIsOpen(e)).length;
  const k = S.abschlussCheck || null;
  const ungeprueft = k ? k.ausgaben.filter((x) => !x.geprueft).length : null;

  const item = (ok, label) => h('li', { class: ok ? 'ok' : 'warn' }, ok ? '✓ ' : '⚠ ', label);
  const items = [
    item(done === total, `Zählerstände vollständig: ${done} von ${total} Pächtern`),
    item(issued >= done, `Rechnungen ausgestellt: ${issued} von ${done} vollständigen Pächtern`),
    item(offenePosten === 0, offenePosten === 0 ? 'Keine offenen Zahlungen mehr' : `${offenePosten} Rechnung(en) noch offen`),
  ];
  if (k) {
    items.push(item(ungeprueft === 0, ungeprueft === 0 ? 'Alle sonstigen Ausgaben sind geprüft' : `${ungeprueft} sonstige Ausgabe(n) noch ohne Geprüft-Haken`));
    items.push(item(k.kassenbestand >= 0, k.kassenbestand >= 0 ? `Kassenbestand positiv (${eur(k.kassenbestand)})` : `Kassenbestand negativ (${eur(k.kassenbestand)})`));
    const wk = k.ausgaben.filter((x) => x.wiederkehrend).length;
    if (wk > 0) items.push(h('li', null, '🔁 ', `${wk} wiederkehrende Ausgabe(n) werden automatisch als Vorschlag fürs neue Jahr übernommen`));
  }
  return h('div', { class: 'card narrow', style: 'max-width:760px;margin-bottom:16px' },
    h('h3', { style: 'margin-top:0' }, 'Vor dem Abschluss prüfen'),
    h('ul', { class: 'checklist' }, items),
    h('p', { class: 'hint', style: 'margin-bottom:0' }, 'Nur ein Hinweis – der Jahreswechsel lässt sich trotzdem durchführen, offene Punkte bleiben danach weiter bearbeitbar.'));
}

function adminYear() {
  const st = S.state;
  const total = st.paechter.length;
  const done = st.paechter.filter((p) => st.results[p.id] && st.results[p.id].vollstaendig).length;
  return h('div', null,
    abschlussChecklist(st),
    h('div', { class: 'card narrow', style: 'max-width:760px' },
      h('h3', { style: 'margin-top:0' }, `Abrechnungsjahr ${st.currentYear} abschließen`),
      h('p', null, 'Am Ende der Abrechnung schließt du das Jahr ab. Dabei passiert Folgendes:'),
      h('ul', { class: 'plain' },
        h('li', null, `Das Jahr ${st.currentYear} wird eingefroren. Preise und Pächterdaten bleiben so gespeichert, dass du alte Rechnungen jederzeit unverändert neu drucken kannst.`),
        h('li', null, `Es startet das Jahr ${st.currentYear + 1}: Die aktuellen Zählerstände werden zu den Vorjahresständen. Stunden, Abschlagszahlungen und weitere Angaben sind wieder leer.`),
        h('li', null, 'Rechnungsdatum und Zahlungsziel rücken um ein Jahr weiter. Prüfe sie danach unter »Preise & Einstellungen«.'),
        h('li', null, 'Vorher wird automatisch eine Sicherung angelegt.')),
      done < total ? h('div', { class: 'banner warn', style: 'margin-top:12px' }, `Achtung: Bei ${total - done} von ${total} Pächtern fehlen noch Angaben.`) : null,
      h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: async () => {
        if (!(await confirmBox(`Jahr ${st.currentYear} jetzt abschließen und ${st.currentYear + 1} beginnen? Das kann nicht rückgängig gemacht werden (die Sicherung vorher bleibt aber erhalten).`, 'Jahr abschließen', true))) return;
        try { await api('POST', '/api/admin/jahreswechsel'); toast('Neues Jahr gestartet', 'ok'); S.year = null; S.dashKasse = null; S.kasse = null; S.kasseVerlauf = null; S.abschlussCheck = null; await reload(); } catch (e) { handleErr(e); }
      } }, `Jahr ${st.currentYear} abschließen`))));
}

// ---- Kassenbericht

async function loadKasse(year) {
  if (S.kasseLoading) return;
  S.kasseLoading = true;
  try { S.kasse = await api('GET', `/api/admin/kassenbericht?year=${year}`); } catch (e) { handleErr(e); }
  S.kasseLoading = false;
  render();
}

async function refreshKasse() { S.kasse = null; S.kasseVerlauf = null; await loadKasse(S.kasseYear); }

async function loadKasseVerlauf() {
  if (S.kasseVerlaufLoading) return;
  S.kasseVerlaufLoading = true;
  try { S.kasseVerlauf = await api('GET', '/api/admin/kassenbericht-verlauf'); } catch (e) { handleErr(e); }
  S.kasseVerlaufLoading = false;
  render();
}

const AUSGABEN_KATEGORIEN = ['Instandhaltung', 'Anschaffung', 'Verwaltung', 'Versicherung & Gebühren', 'Sonstiges'];

function ausgabeDialog(a) {
  const isNew = !a;
  const datum = h('input', { type: 'date', value: (a && a.datum) || todayIso() });
  const besch = h('input', { type: 'text', maxlength: 120, value: (a && a.beschreibung) || '', autocomplete: 'off' });
  const kategorie = h('input', { type: 'text', list: 'kategorien-liste', maxlength: 40, value: (a && a.kategorie) || '', placeholder: AUSGABEN_KATEGORIEN[AUSGABEN_KATEGORIEN.length - 1], autocomplete: 'off' });
  const katList = h('datalist', { id: 'kategorien-liste' }, AUSGABEN_KATEGORIEN.map((x) => h('option', { value: x })));
  const betrag = h('input', { type: 'text', inputmode: 'decimal', value: a ? numIn(a.betrag) : '', autocomplete: 'off' });
  const wiederkehrend = h('input', { type: 'checkbox', checked: !!(a && a.wiederkehrend) });
  const fileInput = h('input', { type: 'file', accept: '.jpg,.jpeg,.png,.webp,.pdf', multiple: true });
  const fld = (label, input) => h('div', { class: 'field' }, h('label', null, label), input);

  // Vorhandene Belege: Liste mit eigenem Entfernen-Knopf je Beleg, aktualisiert
  // sich selbst ohne den Dialog zu schließen.
  const belegeBox = h('div', { style: 'display:flex;flex-wrap:wrap;gap:8px;margin:4px 0' });
  let belegeAktuell = (a && a.belege) || [];
  const zeichneBelege = () => {
    belegeBox.replaceChildren();
    if (!belegeAktuell.length) { belegeBox.append(h('span', { class: 'hint' }, 'Noch keine Belege hinterlegt.')); return; }
    belegeAktuell.forEach((_, i) => {
      belegeBox.append(h('span', { class: 'chip' },
        h('a', { href: `/api/admin/beleg/${a.id}?year=${S.kasseYear}&idx=${i}`, target: '_blank' }, `Beleg ${i + 1}`), ' ',
        h('button', { class: 'btn small danger', type: 'button', title: 'Beleg entfernen', onclick: async () => {
          const ok = await confirmBox(`Beleg ${i + 1} wirklich entfernen?`, 'Entfernen', true);
          if (!ok) return;
          try {
            await api('DELETE', `/api/admin/ausgaben/${a.id}/beleg?year=${S.kasseYear}&idx=${i}`);
            belegeAktuell = belegeAktuell.filter((_, j) => j !== i);
            zeichneBelege();
            // Tabelle hinter dem Dialog sofort aktualisieren, damit sie nicht
            // veraltet bleibt, falls danach "Abbrechen" statt "Speichern" kommt
            // (das Entfernen ist bereits endgültig passiert, nicht Teil des
            // Speichern-Schritts).
            await refreshKasse();
          } catch (e) { handleErr(e); }
        } }, '×')));
    });
  };
  if (!isNew) zeichneBelege();

  const body = h('div', { class: 'form' },
    fld('Datum', datum), fld('Beschreibung (z. B. Rasenmäher, Kontoführungsgebühren)', besch),
    fld('Kategorie', h('div', null, kategorie, katList)),
    h('div', { class: 'field' }, h('label', null, 'Betrag'), h('div', { class: 'inputunit' }, betrag, h('span', null, '€'))),
    h('div', { class: 'field wide', style: 'border:none' },
      h('label', { style: 'display:flex;align-items:center;gap:6px;font-weight:normal' }, wiederkehrend, 'Jährlich wiederkehrend'),
      h('span', { class: 'fhint' }, 'Wird beim Jahreswechsel automatisch als Vorschlag fürs neue Jahr angelegt (gleiche Beschreibung, Kategorie und Betrag, ungeprüft, ohne Beleg) – z. B. für Kontoführungsgebühren. Lässt sich dort weiterhin anpassen oder löschen.')),
    h('div', { class: 'field wide' }, h('label', null, 'Belege (Fotos oder PDF, z. B. Vorder- und Rückseite, optional)'),
      isNew ? null : belegeBox,
      fileInput,
      isNew ? h('span', { class: 'fhint' }, 'Werden nach dem Anlegen hochgeladen.') : null));
  return modal(isNew ? 'Ausgabe erfassen' : `Ausgabe bearbeiten – ${a.beschreibung}`, body, [
    { label: 'Abbrechen', value: false },
    isNew ? null : { label: 'Löschen', cls: 'danger', value: 'del', action: async () => {
      const ok = await confirmBox(`Ausgabe „${a.beschreibung}“ (${eur(a.betrag)}) wirklich löschen?`, 'Löschen', true);
      if (!ok) return false;
      try { await api('DELETE', `/api/admin/ausgaben/${a.id}?year=${S.kasseYear}`); } catch (e) { handleErr(e); return false; }
    } },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const b = parseNum(betrag.value);
      if (!datum.value) { toast('Bitte ein Datum angeben.', 'err'); return false; }
      if (!besch.value.trim()) { toast('Bitte eine Beschreibung eintragen.', 'err'); return false; }
      if (b === null || Number.isNaN(b) || b <= 0) { toast('Bitte einen Betrag größer 0 eintragen.', 'err'); return false; }
      const data = { datum: datum.value, beschreibung: besch.value.trim(), kategorie: kategorie.value.trim(), betrag: b, wiederkehrend: wiederkehrend.checked };
      try {
        const saved = isNew ? await api('POST', `/api/admin/ausgaben?year=${S.kasseYear}`, data) : await api('PUT', `/api/admin/ausgaben/${a.id}?year=${S.kasseYear}`, data);
        for (const file of fileInput.files) {
          const fd = new FormData();
          fd.append('file', file);
          await api('POST', `/api/admin/ausgaben/${saved.id}/beleg?year=${S.kasseYear}`, fd, true);
        }
      } catch (e) { handleErr(e); return false; }
    } },
  ].filter(Boolean));
}

function adminKassenbericht(pruefMode) {
  const st = S.state;
  if (S.kasseYear == null) S.kasseYear = st.currentYear;
  const yearSel = h('select', { onchange: (e) => { S.kasseYear = Number(e.target.value); S.kasse = null; render(); } },
    st.years.map((y) => h('option', { value: y, selected: y === S.kasseYear }, y === st.currentYear ? `${y} (aktuell)` : `${y} (abgeschlossen)`)));
  if (!S.kasse || S.kasse.jahr !== S.kasseYear) {
    if (!S.kasseLoading) loadKasse(S.kasseYear);
    return h('div', null, h('div', { class: 'toolbar' }, h('label', null, 'Jahr'), yearSel), h('p', { class: 'hint' }, 'Wird geladen …'));
  }
  const k = S.kasse;
  if (!S.kasseVerlauf && !S.kasseVerlaufLoading) loadKasseVerlauf();
  const vorjahr = (S.kasseVerlauf || []).find((j) => j.jahr === S.kasseYear - 1);
  const vjDelta = (cur, prev) => {
    if (prev == null) return '';
    const diff = cur - prev;
    if (Math.abs(diff) < 0.005) return ` · = Vorjahr (${S.kasseYear - 1})`;
    const pct = prev !== 0 ? Math.round((diff / Math.abs(prev)) * 1000) / 10 : null;
    const pctTxt = pct == null ? '' : ` (${pct > 0 ? '+' : ''}${nfFlex.format(pct)} %)`;
    return ` · ${diff > 0 ? '▲' : '▼'} ${eur(Math.abs(diff))}${pctTxt} ggü. ${S.kasseYear - 1}`;
  };
  const card = (t, big, small, cls) => h('div', { class: 'card stat ' + (cls || '') }, h('div', { class: 'hint' }, t), h('div', { class: 'big' }, big), h('div', { class: 'hint' }, small));
  const cards = h('div', { class: 'cards' },
    card('Kassenbestand', eur(k.kassenbestand), `Anfangsbestand ${eur(k.anfangsbestand)} + Zahlungen − Ausgaben${vorjahr ? vjDelta(k.kassenbestand, vorjahr.kassenbestand) : ''}`, k.kassenbestand >= 0 ? 'ok' : 'err'),
    card('Von Pächtern eingegangen', `${eur(k.einnahmenBezahlt)}`, `tatsächlich gezahlt, nach heutigem Stand${vorjahr ? vjDelta(k.einnahmenBezahlt, vorjahr.einnahmenBezahlt) : ''}`, 'ok'),
    k.guthabenAusgezahlt ? card('An Pächter ausgezahlt', eur(k.guthabenAusgezahlt), 'Guthaben') : null,
    card('Sonstige Ausgaben', eur(k.ausgabenSumme), `${k.ausgaben.length} Buchung(en)${vorjahr ? vjDelta(k.ausgabenSumme, vorjahr.ausgabenSumme) : ''}`, k.ausgabenSumme ? 'warn' : 'ok'));

  const anfInput = h('input', { type: 'text', inputmode: 'decimal', value: numIn(k.anfangsbestand), style: 'width:140px', autocomplete: 'off' });
  const saveAnfang = async () => {
    const v = parseNum(anfInput.value);
    if (v === null || Number.isNaN(v) || v < 0) { toast('Bitte eine Zahl ab 0 eingeben.', 'err'); return; }
    try { S.kasse = await api('PUT', `/api/admin/anfangsbestand?year=${S.kasseYear}`, { betrag: v }); S.kasseVerlauf = null; toast('Gespeichert', 'ok'); render(); } catch (e) { handleErr(e); }
  };

  const ohneBeleg = k.ausgaben.filter((x) => !(x.belege && x.belege.length)).length;
  const belegFilter = h('input', { type: 'checkbox', checked: S.kasseNurOhneBeleg,
    onchange: (e) => { S.kasseNurOhneBeleg = e.target.checked; render(); } });
  const gezeigt = S.kasseNurOhneBeleg ? k.ausgaben.filter((x) => !(x.belege && x.belege.length)) : k.ausgaben;
  const toggleGeprueft = async (x, checked) => {
    try { await api('PUT', `/api/admin/ausgaben/${x.id}/geprueft?year=${S.kasseYear}`, { geprueft: checked }); await refreshKasse(); }
    catch (e) { handleErr(e); }
  };
  const tbody = h('tbody');
  for (const x of gezeigt) {
    tbody.append(h('tr', null, h('td', null, deDate(x.datum)),
      h('td', null, x.beschreibung, x.wiederkehrend ? h('span', { title: 'Jährlich wiederkehrend', style: 'margin-left:4px;cursor:help' }, '🔁') : null),
      h('td', null, x.kategorie),
      h('td', { class: 'r' }, eur(x.betrag)),
      h('td', null, x.belege && x.belege.length ? h('a', { href: `/api/admin/beleg/${x.id}?year=${S.kasseYear}&idx=0`, target: '_blank', title: `${x.belege.length} Beleg(e) ansehen` }, `📎${x.belege.length > 1 ? ' ' + x.belege.length : ''}`) : null),
      h('td', { title: x.geprueft ? `Geprüft am ${deDate(x.geprueftAm)}` : 'Von der Kassenprüfung abhaken' },
        h('input', { type: 'checkbox', checked: x.geprueft, onchange: (e) => toggleGeprueft(x, e.target.checked) })),
      h('td', null, pruefMode ? null : h('button', { class: 'btn small', onclick: async () => { const r = await ausgabeDialog(x); if (r !== false && r !== undefined) await refreshKasse(); } }, 'Bearbeiten'))));
  }
  if (!gezeigt.length) tbody.append(h('tr', null, h('td', { colspan: 7, class: 'empty' },
    k.ausgaben.length ? 'Alle Ausgaben haben einen Beleg.' : 'Noch keine sonstigen Ausgaben erfasst.')));

  const katTable = (k.ausgabenKategorie || []).length > 1
    ? h('div', { class: 'tablewrap', style: 'max-width:360px;margin-top:10px' }, h('table', { class: 'data' },
        h('thead', null, h('tr', null, ['Kategorie', 'Summe'].map((t) => h('th', null, t)))),
        h('tbody', null, k.ausgabenKategorie.map((kat) => h('tr', null, h('td', null, kat.kategorie), h('td', { class: 'r' }, eur(kat.summe)))))))
    : null;

  const vW3 = h('input', { type: 'text', inputmode: 'decimal', value: numIn(k.versorger.wasserM3), autocomplete: 'off', disabled: !!pruefMode });
  const vWE = h('input', { type: 'text', inputmode: 'decimal', value: numIn(k.versorger.wasserEUR), autocomplete: 'off', disabled: !!pruefMode });
  const vSK = h('input', { type: 'text', inputmode: 'decimal', value: numIn(k.versorger.stromKWh), autocomplete: 'off', disabled: !!pruefMode });
  const vSE = h('input', { type: 'text', inputmode: 'decimal', value: numIn(k.versorger.stromEUR), autocomplete: 'off', disabled: !!pruefMode });
  const diffRow = (label, total, vInput, unit) => {
    const v = parseNum(vInput.value);
    const diff = v == null || Number.isNaN(v) ? null : Math.round((v - total) * 100) / 100;
    return h('tr', null, h('td', null, label), h('td', { class: 'r' }, unit === '€' ? eur(total) : nfFlex.format(total) + ' ' + unit),
      h('td', null, vInput), h('td', { class: 'r' + (diff != null && Math.abs(diff) > 0.004 ? ' diffwarn' : '') },
        diff == null ? '–' : (unit === '€' ? eur(diff) : nfFlex.format(diff) + ' ' + unit)));
  };
  const versorgerTable = h('table', { class: 'data' },
    h('thead', null, h('tr', null, ['', 'Pächter gesamt', 'Versorger / Hauptzähler', 'Differenz'].map((t) => h('th', null, t)))),
    h('tbody', null,
      diffRow('Wasser', k.wasserVerbrauch, vW3, 'm³'), diffRow('Wasser (€)', k.summen.kostenWasser, vWE, '€'),
      diffRow('Strom', k.stromVerbrauch, vSK, 'kWh'), diffRow('Strom (€)', k.summen.kostenEnergie, vSE, '€')));
  const saveVersorger = async () => {
    const vals = [vW3, vWE, vSK, vSE].map((i) => parseNum(i.value));
    if (vals.some((v) => Number.isNaN(v) || (v !== null && v < 0))) { toast('Bitte nur Zahlen ab 0 eingeben.', 'err'); return; }
    try {
      S.kasse = await api('PUT', `/api/admin/versorger?year=${S.kasseYear}`, { wasserM3: vals[0], wasserEUR: vals[1], stromKWh: vals[2], stromEUR: vals[3] });
      toast('Gespeichert', 'ok');
      render();
    } catch (e) { handleErr(e); }
  };

  return h('div', null,
    h('div', { class: 'toolbar' }, h('label', null, 'Jahr'), yearSel),
    h('p', { class: 'hint' }, `Übersicht für ${S.kasseYear}: ${k.paechter} Pächter (${k.ausgestellt} aus ausgestellten Rechnungen, ${k.berechnet} berechnet, aber noch nicht ausgestellt${k.unvollstaendig.length ? `, ${k.unvollstaendig.length} unvollständig` : ''}). Die Beträge »sonstige Ausgaben« sind Vereinskosten neben der Pächterabrechnung, z. B. Kontoführungsgebühren, Anwaltskosten oder Anschaffungen.`),
    k.unvollstaendig.length ? h('div', { class: 'banner warn' }, `Nicht enthalten (Angaben fehlen): ${k.unvollstaendig.join(', ')}`) : null,
    h('div', { class: 'card', style: 'margin-bottom:14px' },
      h('div', { class: 'toolbar' },
        h('label', null, 'Anfangsbestand der Kasse'),
        pruefMode ? h('b', null, eur(k.anfangsbestand)) : h('div', { class: 'inputunit' }, anfInput, h('span', null, '€')),
        pruefMode ? null : h('button', { class: 'btn small primary', onclick: saveAnfang }, 'Speichern'),
        pruefMode ? null : h('span', { class: 'hint' }, 'wird beim Jahreswechsel automatisch aus dem Kassenbestand des Vorjahres übernommen, ist hier aber jederzeit änderbar'))),
    cards,
    h('h3', null, 'Sonstige Ausgaben der Vereinskasse'),
    h('div', { class: 'card' },
      h('div', { class: 'toolbar' },
        pruefMode ? null : h('button', { class: 'btn primary', onclick: async () => { const r = await ausgabeDialog(null); if (r !== false && r !== undefined) await refreshKasse(); } }, '+ Ausgabe erfassen'),
        h('div', { class: 'spacer' }),
        h('label', { class: 'hint' }, belegFilter, ` nur ohne Beleg${ohneBeleg ? ` (${ohneBeleg})` : ''}`)),
      h('div', { class: 'tablewrap', style: 'max-height:360px' }, h('table', { class: 'data' },
        h('thead', null, h('tr', null, ['Datum', 'Beschreibung', 'Kategorie', 'Betrag', 'Beleg', 'Geprüft', ''].map((t) => h('th', null, t)))), tbody)),
      katTable),
    h('h3', null, 'Vergleich mit dem Versorger'),
    h('div', { class: 'card' },
      h('p', { class: 'hint' }, 'Trage hier die Werte der Hauptzähler bzw. der Versorgerrechnung ein, um sie mit der Summe der Pächterabrechnung zu vergleichen. Eine größere Abweichung kann auf einen Zählerfehler, Schwund oder eine falsche Ablesung hindeuten.'),
      h('div', { class: 'tablewrap' }, versorgerTable),
      pruefMode ? null : h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: saveVersorger }, 'Werte speichern'))),
    h('div', { class: 'actions', style: 'margin-top:16px;margin-bottom:22px' },
      h('a', { class: 'btn', href: `/api/admin/export-kassenbericht?year=${S.kasseYear}` }, `Kassenbericht ${S.kasseYear} als Excel`)),
    h('h3', null, 'Verlauf über die Jahre'),
    kassenberichtVerlaufCard());
}

function kassenberichtVerlaufCard() {
  if (!S.kasseVerlauf) {
    if (!S.kasseVerlaufLoading) loadKasseVerlauf();
    return h('div', { class: 'card' }, h('p', { class: 'hint', style: 'margin:0' }, 'Wird geladen …'));
  }
  const v = S.kasseVerlauf;
  if (!v.length) return h('div', { class: 'card empty' }, 'Noch keine Jahre vorhanden.');
  const rows = v.map((j) => h('tr', null,
    h('td', null, j.jahr),
    h('td', { class: 'r' }, eur(j.rechnungssumme)),
    h('td', { class: 'r' }, eur(j.einnahmenBezahlt)),
    h('td', { class: 'r' }, eur(j.ausgabenSumme)),
    h('td', { class: 'r' }, eur(j.kassenbestand))));
  const chartData = [...v].reverse().map((j) => ({ label: String(j.jahr), value: j.kassenbestand }));
  return h('div', { class: 'card' },
    h('p', { class: 'hint' }, 'Summen des Vereins insgesamt (nicht je Pächter), zum Vergleich über die Jahre – z. B. für die Mitgliederversammlung.'),
    h('p', { class: 'hint', style: 'margin:0 0 4px' }, 'Kassenbestand am Jahresende:'),
    barChart(chartData, { valueFmt: (v) => eur(v) }),
    h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Jahr', 'Rechnungen gestellt', 'Zahlungen eingegangen', 'Sonstige Ausgaben', 'Kassenbestand'].map((t) => h('th', null, t)))),
      h('tbody', null, rows))));
}

// ---- Import / Export / Sicherung

function adminData() {
  const st = S.state;
  const wrap = h('div');
  const file = h('input', { type: 'file', accept: '.xlsx,.csv,.txt' });
  const previewBox = h('div');

  const drawPreview = () => {
    previewBox.replaceChildren();
    const pv = S.importPreview;
    if (!pv) return;
    const checks = [];
    const tb = h('tbody');
    for (const r of pv.rows) {
      const c = h('input', { type: 'checkbox', checked: true });
      checks.push([c, r]);
      tb.append(h('tr', null, h('td', null, c), h('td', null, h('span', { class: 'badge ' + (r.aktion === 'neu' ? 'neu' : 'warn') }, r.aktion === 'neu' ? 'neu' : 'aktualisieren')),
        h('td', null, r.paechter.mitgliedsnr), h('td', null, r.paechter.name), h('td', null, [r.paechter.strasse, r.paechter.plzOrt].filter(Boolean).join(', ')),
        h('td', { class: 'num' }, r.paechter.gartengroesse ? nfFlex.format(r.paechter.gartengroesse) + ' m²' : ''),
        h('td', null, r.ablesung ? '✓ Zählerstände' : ''), h('td', null, (r.warnungen || []).join('; '))));
    }
    previewBox.append(h('div', null,
      h('h3', null, `Vorschau: ${pv.rows.length} Zeilen gefunden`),
      (pv.warnings || []).length ? h('div', { class: 'banner warn' }, h('ul', { class: 'plain', style: 'margin:0' }, pv.warnings.map((w) => h('li', null, w)))) : null,
      h('p', { class: 'hint' }, 'Erkannte Spalten: ', (pv.rows[0].felder || []).join(', ') || 'nur Mitgliedsnr. und Name',
        '. Bereits vorhandene Pächter (gleiche Mitgliedsnummer) werden aktualisiert, alle anderen neu angelegt. Angaben, die in der Datei fehlen, bleiben unverändert.'),
      h('div', { class: 'tablewrap', style: 'max-height:360px' }, h('table', null,
        h('thead', null, h('tr', null, ['', 'Aktion', 'Mitgl.-Nr.', 'Name', 'Anschrift', 'Größe', 'Zählerstände', 'Hinweis'].map((x) => h('th', null, x)))), tb)),
      h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: async () => {
        const rows = checks.filter(([c]) => c.checked).map(([, r]) => r);
        if (!rows.length) { toast('Keine Zeile ausgewählt.', 'err'); return; }
        try {
          const r = await api('POST', '/api/admin/import/apply', { rows });
          toast(`Import fertig: ${r.neu} neu, ${r.aktualisiert} aktualisiert`, 'ok');
          S.importPreview = null;
          await reload();
        } catch (e) { handleErr(e); }
      } }, 'Ausgewählte Zeilen importieren'), h('button', { class: 'btn', onclick: () => { S.importPreview = null; drawPreview(); file.value = ''; } }, 'Abbrechen'))));
  };
  file.addEventListener('change', async () => {
    if (!file.files.length) return;
    const fd = new FormData();
    fd.append('file', file.files[0]);
    try { S.importPreview = await api('POST', '/api/admin/import/preview', fd, true); drawPreview(); } catch (e) { S.importPreview = null; drawPreview(); handleErr(e); }
  });
  drawPreview();

  wrap.append(
    h('div', { class: 'card', style: 'margin-bottom:16px' },
      h('h3', { style: 'margin-top:0' }, 'Pächter aus Excel oder CSV importieren'),
      h('p', { class: 'hint' }, 'Die Datei braucht eine Kopfzeile mit mindestens »Mitgliedsnr.« und »Name«. Weitere Spalten werden automatisch erkannt: Gartennr., Anrede, Straße, PLZ Ort, Versandart, Gartengröße, Umlage abweichend, Wasserzähler-Nr., Stromzähler-Nr., Wasser/Energie Stand Vorjahr und aktuell, Arbeitsstunden, Versicherung, Grundsteuer, Auslagen, Hinweis, Abschlag. Eine passend aufgebaute Excel-Vorlage (Blatt »Mitglieder«) funktioniert direkt.'),
      file, previewBox),
    h('div', { class: 'card', style: 'margin-bottom:16px' },
      h('h3', { style: 'margin-top:0' }, 'Export'),
      h('p', { class: 'hint' }, `Alle Pächter des Jahres ${st.year} mit Zählerständen und Beträgen als Excel-Datei, zum Beispiel für die Kassenprüfung.`),
      h('a', { class: 'btn', href: `/api/export?year=${st.year}` }, `Jahresübersicht ${st.year} als Excel`)),
    h('div', { class: 'card' },
      h('h3', { style: 'margin-top:0' }, 'Datensicherung'),
      h('p', { class: 'hint' }, 'Das Programm legt bei Änderungen automatisch eine Tagessicherung an (die letzten 60 Tage) und vor dem Jahreswechsel, Löschen und Import zusätzlich eine eigene. Du kannst außerdem den gesamten Datenbestand herunterladen. Zum Wiederherstellen die gewünschte Sicherungsdatei in »kleingarten-manager-daten.json« umbenennen und im Datenordner ersetzen (Programm vorher beenden).'),
      h('div', { class: 'actions', style: 'margin-top:8px' },
        h('a', { class: 'btn', href: '/api/admin/backup' }, 'Gesamten Datenbestand herunterladen'),
        h('button', { class: 'btn', onclick: () => api('POST', '/api/open-folder', { which: 'sicherungen' }).catch(handleErr) }, 'Sicherungsordner öffnen'),
        h('button', { class: 'btn', onclick: () => api('POST', '/api/open-folder', { which: 'daten' }).catch(handleErr) }, 'Datenordner öffnen')),
      zweiteSicherungField()));
  return wrap;
}

function zweiteSicherungField() {
  const st = S.state;
  const inp = h('input', { type: 'text', value: st.settings.zweiteSicherung || '', style: 'width:100%', autocomplete: 'off',
    placeholder: 'z. B. D:\\Sicherung-Kleingarten-Manager oder ein Netzlaufwerk-Pfad' });
  const save = async () => {
    try {
      await api('PUT', '/api/admin/settings', { ...st.settings, zweiteSicherung: inp.value.trim() });
      toast('Gespeichert', 'ok');
      await reload();
    } catch (e) { handleErr(e); }
  };
  return h('div', { style: 'margin-top:16px;border-top:1px solid var(--line,#ddd);padding-top:12px' },
    h('div', { class: 'field wide' }, h('label', null, 'Zweiter Sicherungsordner (optional, z. B. USB-Stick oder Netzlaufwerk)'), inp,
      h('span', { class: 'fhint' }, 'Leer lassen = deaktiviert. Ist der Pfad gerade nicht erreichbar (USB-Stick nicht eingesteckt), läuft das Programm trotzdem normal weiter, es wird nur dort nicht zusätzlich gesichert.')),
    h('div', { class: 'actions' }, h('button', { class: 'btn', onclick: save }, 'Speichern')));
}

// ---- Jubiläen

async function loadJubilaeen() {
  if (S.jubilaeenLoading) return;
  S.jubilaeenLoading = true;
  try { S.jubilaeen = await api('GET', '/api/garten-historie'); } catch (e) { handleErr(e); S.jubilaeen = []; }
  S.jubilaeenLoading = false;
  render();
}

// Zeigt, wie lange ein Pächter schon dabei ist – aus dem frühesten Eintrag der
// Gartenhistorie je Pächter, nützlich z. B. für die Ehrung langjähriger
// Mitglieder bei der Mitgliederversammlung. Steht erst ab Einführung der
// Gartenhistorie zur Verfügung; ältere Pächter ohne Eintrag landen gesondert.
function adminJubilaeen() {
  const st = S.state;
  if (!S.jubilaeen) {
    if (!S.jubilaeenLoading) loadJubilaeen();
    return h('p', { class: 'hint' }, 'Wird geladen …');
  }
  const seitByPaechter = {};
  for (const e of S.jubilaeen) {
    if (!seitByPaechter[e.paechterId] || e.seit < seitByPaechter[e.paechterId]) seitByPaechter[e.paechterId] = e.seit;
  }
  const list = [...st.paechter].sort(cmpNr);
  const rows = [];
  const ohneDatum = [];
  for (const p of list) {
    const seit = seitByPaechter[p.id];
    if (!seit) { ohneDatum.push(p); continue; }
    rows.push({ p, seit, jahre: jahreSeit(seit) });
  }
  rows.sort((a, b) => a.seit.localeCompare(b.seit));
  const tbody = h('tbody', null, rows.map((r) => h('tr', null,
    h('td', null, r.p.mitgliedsnr), h('td', null, r.p.name), h('td', null, r.p.gartennr),
    h('td', null, deDate(r.seit)),
    h('td', { class: 'r' }, r.jahre > 0 && r.jahre % 5 === 0
      ? h('b', { title: 'Jubiläum', style: 'color:var(--ok)' }, `🎉 ${r.jahre} Jahre`)
      : `${r.jahre} Jahre`))));
  return h('div', null,
    h('p', { class: 'hint' }, 'Mitglied-seit-Datum aus der Gartenhistorie (frühester Eintrag je Pächter). Nützlich für die Mitgliederversammlung, z. B. um langjährige Mitglieder zu ehren. Steht erst ab Einführung der Gartenhistorie-Funktion zur Verfügung – bei Pächtern von davor fehlt das Datum, sie stehen unten in einer eigenen Liste.'),
    h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Mitgl.-Nr.', 'Name', 'Garten', 'Mitglied seit', 'Dabei seit'].map((t) => h('th', null, t)))),
      rows.length ? tbody : h('tbody', null, h('tr', null, h('td', { colspan: 5, class: 'empty' }, 'Noch keine Daten.'))))),
    ohneDatum.length ? h('div', { class: 'card', style: 'margin-top:12px' },
      h('h3', { style: 'margin-top:0' }, 'Ohne bekanntes Eintrittsdatum'),
      h('p', { class: 'hint' }, ohneDatum.map((p) => `${p.mitgliedsnr} ${p.name}`).join(', '))) : null);
}

// ---- Änderungsprotokoll

async function loadProtokoll() {
  if (S.protokollLoading) return;
  S.protokollLoading = true;
  try { S.protokoll = await api('GET', '/api/admin/audit-log'); } catch (e) { handleErr(e); S.protokoll = []; }
  S.protokollLoading = false;
  render();
}

function adminProtokoll() {
  if (!S.protokoll) {
    if (!S.protokollLoading) loadProtokoll();
    return h('p', { class: 'hint' }, 'Wird geladen …');
  }
  const fmtZeit = (iso) => { const d = new Date(iso); return isNaN(d) ? iso : d.toLocaleString('de-DE'); };
  const rows = S.protokoll.map((e) => h('tr', null, h('td', { class: 'mono', style: 'white-space:nowrap' }, fmtZeit(e.zeit)), h('td', null, e.aktion)));
  return h('div', null,
    h('p', { class: 'hint' }, 'Wer wann welchen Pächter oder welche Preise geändert hat. Da es nur ein gemeinsames Admin-Passwort gibt, wird nicht festgehalten, welche Person es war – nur Zeitpunkt und Art der Änderung. Die letzten 1000 Einträge bleiben erhalten.'),
    h('div', { class: 'card tablewrap', style: 'max-height:520px' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, h('th', null, 'Zeitpunkt'), h('th', null, 'Änderung'))),
      h('tbody', null, rows.length ? rows : h('tr', null, h('td', { colspan: 2, class: 'empty' }, 'Noch keine Einträge.'))))));
}

// ---- Passwort

function adminSecurity() {
  const st = S.state;
  const old = h('input', { type: 'password', autocomplete: 'current-password' });
  const n1 = h('input', { type: 'password', autocomplete: 'new-password' });
  const n2 = h('input', { type: 'password', autocomplete: 'new-password' });
  const fld = (label, input, hint) => h('div', { class: 'field' }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const save = async () => {
    if (n1.value !== n2.value) { toast('Die beiden neuen Passwörter sind nicht gleich.', 'err'); return; }
    if (!st.hasPassword && !n1.value) { toast('Bitte ein Passwort eingeben.', 'err'); return; }
    try {
      await api('POST', '/api/admin/password', { old: old.value, new: n1.value });
      toast(n1.value ? 'Passwort gespeichert' : 'Passwortschutz entfernt', 'ok');
      await reload();
    } catch (e) { handleErr(e); }
  };
  return h('div', null,
    h('div', { class: 'card narrow' },
      h('h3', { style: 'margin-top:0' }, st.hasPassword ? 'Admin-Passwort ändern oder entfernen' : 'Admin-Passwort festlegen'),
      h('p', { class: 'hint' }, 'Das Passwort schützt Pächter, Preise, Jahreswechsel und Import. Die Zählerstände kann weiterhin jeder eintragen, der das Programm öffnet.'),
      h('div', { class: 'form', style: 'grid-template-columns:1fr' },
        st.hasPassword ? fld('Bisheriges Passwort', old) : null,
        fld('Neues Passwort', n1, st.hasPassword ? 'Leer lassen, um den Passwortschutz zu entfernen' : 'Mindestens 8 Zeichen'),
        fld('Neues Passwort wiederholen', n2)),
      h('div', { class: 'actions' },
        h('button', { class: 'btn primary', onclick: save }, 'Speichern'),
        st.hasPassword ? h('button', { class: 'btn', onclick: async () => { try { await api('POST', '/api/admin/logout'); await reload(); } catch (e) { handleErr(e); } } }, 'Abmelden') : null),
      h('p', { class: 'hint', style: 'margin-top:16px' }, 'Passwort vergessen? Das Programm mit dem Zusatz "--reset-admin" starten (in der Eingabeaufforderung: Kleingarten-Manager.exe --reset-admin). Dann ist der Passwortschutz entfernt und du kannst ein neues Passwort festlegen.')),
    pruefPasswortCard(),
    updateCheckCard());
}

// ---- Kassenprüfer-Zugang verwalten (nur der Admin kann ihn einrichten)

function pruefPasswortCard() {
  const st = S.state;
  const n1 = h('input', { type: 'password', autocomplete: 'new-password' });
  const n2 = h('input', { type: 'password', autocomplete: 'new-password' });
  const fld = (label, input, hint) => h('div', { class: 'field' }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const save = async () => {
    if (n1.value !== n2.value) { toast('Die beiden Passwörter sind nicht gleich.', 'err'); return; }
    try {
      await api('POST', '/api/admin/pruef-password', { new: n1.value });
      toast(n1.value ? 'Kassenprüfer-Zugang gespeichert' : 'Kassenprüfer-Zugang entfernt', 'ok');
      await reload();
    } catch (e) { handleErr(e); }
  };
  return h('div', { class: 'card narrow', style: 'margin-top:16px' },
    h('h3', { style: 'margin-top:0' }, 'Kassenprüfer-Zugang'),
    h('p', { class: 'hint' }, `Eigenes, optionales Passwort für den Kassenprüfer: Lesezugriff auf den Kassenbericht (inkl. Geprüft-Haken für die sonstigen Ausgaben), aber ohne die übrigen Admin-Rechte. ${st.hasPruefPassword ? 'Zugang ist eingerichtet.' : 'Zugang ist nicht eingerichtet.'}`),
    h('div', { class: 'form', style: 'grid-template-columns:1fr' },
      fld('Passwort', n1, st.hasPruefPassword ? 'Leer lassen, um den Zugang zu entfernen' : 'Mindestens 8 Zeichen'),
      fld('Passwort wiederholen', n2)),
    h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: save }, 'Speichern')));
}

// ---- Update-Prüfung (nur auf Knopfdruck, nie automatisch)

async function checkUpdate() {
  if (S.updateChecking) return;
  S.updateChecking = true;
  try { S.updateCheck = await api('GET', '/api/admin/check-update'); } catch (e) { S.updateCheck = { hinweis: e.message }; }
  S.updateChecking = false;
  render();
}

function updateCheckCard() {
  const st = S.state;
  const r = S.updateCheck;
  return h('div', { class: 'card narrow', style: 'margin-top:16px' },
    h('h3', { style: 'margin-top:0' }, 'Nach Updates suchen'),
    h('p', { class: 'hint' }, `Installierte Version: ${st.version}. Die Prüfung fragt nur auf Knopfdruck einmalig die öffentliche GitHub-Release-Seite dieses Projekts ab, es läuft sonst keine Internetverbindung im Hintergrund.`),
    h('div', { class: 'actions' },
      h('button', { class: 'btn', disabled: S.updateChecking, onclick: checkUpdate }, S.updateChecking ? 'Prüfe …' : 'Nach Updates suchen')),
    r ? (r.available
      ? h('div', { class: 'banner info', style: 'margin-top:10px' }, `Version ${r.version} ist verfügbar. `,
          h('a', { href: r.url, target: '_blank', rel: 'noopener noreferrer', onclick: (e) => {
            // Im eigenen Programmfenster über den Systembrowser öffnen statt im Fenster selbst.
            if (typeof window.kgmOpenExternal !== 'function') return;
            e.preventDefault();
            window.kgmOpenExternal(r.url).catch(() => toast('Die Release-Seite konnte nicht geöffnet werden.', 'err'));
          } }, 'Release-Seite öffnen'))
      : r.hinweis
        ? h('p', { class: 'hint', style: 'margin-top:10px' }, r.hinweis)
        : h('p', { class: 'hint', style: 'margin-top:10px' }, 'Du hast die aktuelle Version.')) : null);
}

// ------------------------------------------------------------------ Start

(async function start() {
  document.body.append(h('div', { id: 'toasts' }));
  try { await load(); render(); } catch (e) {
    document.getElementById('app').replaceChildren(h('div', { class: 'center card' }, h('h2', null, 'Verbindung fehlgeschlagen'), h('p', null, e.message)));
  }
})();
