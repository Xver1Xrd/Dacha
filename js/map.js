/* Публичная карта (только просмотр). Данные — из SNTStore. */
(function () {
  'use strict';

  var wrap = document.getElementById('plan-wrap');
  if (!wrap || !window.SNTStore) return;

  var STATUS = {
    none:     { label: 'Работы не проводились' },
    done:     { label: 'Завершено' },
    progress: { label: 'Сейчас в работе' },
    planned:  { label: 'Запланировано' }
  };

  var NS = 'http://www.w3.org/2000/svg';
  var CELL = 34, GAP = 5, ROWS = 2;
  var STEP = CELL + GAP, SIDE = 24, TOP = 54, BLOCK_GAP = 22, LABEL_H = 20;

  function esc(s) {
    return String(s).replace(/[&<>"]/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c];
    });
  }
  function cssEsc(s) { return String(s).replace(/(["\\])/g, '\\$1'); }

  function build() {
    var alleys = window.SNTStore.getAlleys();
    var works = window.SNTStore.getWorks();

    var blocks = [], y = TOP, maxW = 0;
    alleys.forEach(function (a) {
      var cols = Math.ceil(a.count / ROWS);
      var w = cols * STEP - GAP;
      blocks.push({ a: a, cols: cols, w: w, y: y });
      if (w > maxW) maxW = w;
      y += LABEL_H + ROWS * STEP - GAP + BLOCK_GAP;
    });
    var W = maxW + SIDE * 2;
    var H = y + 70;

    var svg = '<svg id="plan" xmlns="' + NS + '" viewBox="0 0 ' + W + ' ' + H +
      '" width="' + W + '" height="' + H + '" role="img" aria-label="План СНТ Михайловское">';
    svg += '<rect x="4" y="4" width="' + (W - 8) + '" height="' + (H - 8) + '" rx="16" fill="none" stroke="var(--line)" stroke-width="1.5"/>';
    svg += '<text class="plan-title" x="' + (W / 2) + '" y="30">План СНТ «Михайловское»</text>';
    svg += '<text class="plan-note" x="' + (W / 2) + '" y="46">нажмите на участок, чтобы увидеть работы</text>';

    blocks.forEach(function (b) {
      var startX = (W - b.w) / 2;
      svg += '<text class="alley-label" x="' + (W / 2) + '" y="' + (b.y + 13) + '">' + esc(b.a.label) + '</text>';
      var top = b.y + LABEL_H;
      for (var i = 0; i < b.a.count; i++) {
        var row = i < b.cols ? 0 : 1;
        var col = i < b.cols ? i : i - b.cols;
        var n = i + 1;
        var x = startX + col * STEP, py = top + row * STEP;
        var key = b.a.label + ' ' + n;
        var st = works[key] ? works[key].status : 'none';
        svg += '<g class="plot" data-status="' + st + '" data-key="' + esc(key) + '" tabindex="0" role="button" aria-label="' + esc(key) + '">' +
          '<rect class="cell" x="' + x + '" y="' + py + '" width="' + CELL + '" height="' + CELL + '" rx="7"/>' +
          '<circle class="dot" cx="' + (x + CELL - 6) + '" cy="' + (py + 6) + '" r="3.6"/>' +
          '<text class="num" x="' + (x + CELL / 2) + '" y="' + (py + CELL / 2 + 1) + '">' + n + '</text>' +
          '</g>';
      }
    });

    var zx = W / 2 - 70, zy = H - 52;
    svg += '<rect class="zona" x="' + zx + '" y="' + zy + '" width="140" height="34" rx="9"/>';
    svg += '<text class="plan-note" x="' + (W / 2) + '" y="' + (zy + 21) + '">Зона отдыха</text>';
    svg += '</svg>';
    wrap.innerHTML = svg;
  }

  build();

  // ---- панель информации (просмотр) ----
  var back = document.getElementById('panelBack');
  var panel = document.getElementById('panel');
  var pTitle = document.getElementById('pTitle');
  var pSub = document.getElementById('pSub');
  var pBody = document.getElementById('pBody');
  var selected = null;

  function initials(name) { return name.trim().charAt(0).toUpperCase(); }

  function openPanel(key) {
    var rec = window.SNTStore.getWorks()[key];
    var st = rec ? rec.status : 'none';
    var parts = key.split(' '), num = parts.pop(), alley = parts.join(' ');

    pTitle.textContent = 'Участок №' + num;
    pSub.textContent = alley;

    var html = '<span class="badge ' + st + '"><span class="d"></span>' + STATUS[st].label + '</span>';
    if (rec) {
      var who = (rec.workers && rec.workers.length)
        ? rec.workers.map(function (w) { return '<span class="worker"><span class="ava">' + esc(initials(w)) + '</span>' + esc(w) + '</span>'; }).join('')
        : '<span class="panel-empty">—</span>';
      var together = (rec.workers && rec.workers.length > 1)
        ? '<div class="v" style="margin-top:6px;color:var(--soil-soft);font-size:.88rem">Работали вместе</div>' : '';
      html += '<div class="panel-row"><div class="k">Кто работал</div><div class="workers">' + who + '</div>' + together + '</div>';
      html += '<div class="panel-row"><div class="k">Что делали</div><div class="v">' + esc(rec.work || '—') + '</div></div>';
      if (rec.date) html += '<div class="panel-row"><div class="k">Когда</div><div class="v">' + esc(rec.date) + '</div></div>';
    } else {
      html += '<div class="panel-row"><div class="v panel-empty">На этом участке работы не проводились. Нужна помощь — звоните, договоримся.</div></div>';
      html += '<div class="panel-cta"><a href="#contacts" class="btn btn-sun">Связаться</a></div>';
    }
    pBody.innerHTML = html;
    back.classList.add('open');
    panel.classList.add('open');

    if (selected) selected.classList.remove('sel');
    selected = wrap.querySelector('.plot[data-key="' + cssEsc(key) + '"]');
    if (selected) selected.classList.add('sel');
  }

  function closePanel() {
    back.classList.remove('open');
    panel.classList.remove('open');
    if (selected) { selected.classList.remove('sel'); selected = null; }
  }

  wrap.addEventListener('click', function (e) {
    var g = e.target.closest ? e.target.closest('.plot') : null;
    if (g) openPanel(g.getAttribute('data-key'));
  });
  wrap.addEventListener('keydown', function (e) {
    if ((e.key === 'Enter' || e.key === ' ') && e.target.classList && e.target.classList.contains('plot')) {
      e.preventDefault(); openPanel(e.target.getAttribute('data-key'));
    }
  });
  back.addEventListener('click', closePanel);
  var closeBtn = document.getElementById('panelClose');
  if (closeBtn) closeBtn.addEventListener('click', closePanel);
  document.addEventListener('keydown', function (e) { if (e.key === 'Escape') closePanel(); });

  // ---- поиск по номеру ----
  var search = document.getElementById('plotSearch');
  if (search) {
    search.addEventListener('input', function () {
      var q = search.value.trim();
      var plots = wrap.querySelectorAll('.plot');
      if (!q) { plots.forEach(function (p) { p.classList.remove('hit', 'dim'); }); return; }
      var first = null;
      plots.forEach(function (p) {
        var n = p.getAttribute('data-key').split(' ').pop();
        if (n === q) { p.classList.add('hit'); p.classList.remove('dim'); if (!first) first = p; }
        else { p.classList.remove('hit'); p.classList.add('dim'); }
      });
      if (first) first.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'center' });
    });
  }
})();
