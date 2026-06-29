/* Админпанель: вход, работники, администраторы, редактор карты. */
(function () {
  'use strict';
  var S = window.SNTStore;
  if (!S) return;

  var $ = function (id) { return document.getElementById(id); };
  function esc(s) {
    return String(s).replace(/[&<>"]/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c];
    });
  }
  function cssEsc(s) { return String(s).replace(/(["\\])/g, '\\$1'); }
  function initials(n) { return n.trim().charAt(0).toUpperCase(); }

  var toastT;
  function toast(msg) {
    var t = $('toast'); t.textContent = msg; t.classList.add('show');
    clearTimeout(toastT); toastT = setTimeout(function () { t.classList.remove('show'); }, 1800);
  }

  var STATUSES = [
    { id: 'none', label: 'Не работали' },
    { id: 'done', label: 'Завершено' },
    { id: 'progress', label: 'В работе' },
    { id: 'planned', label: 'Запланировано' }
  ];

  /* ---------------- вход ---------------- */
  var loginView = $('loginView'), adminView = $('adminView');

  function showGate() {
    var sess = S.session();
    if (sess) {
      loginView.style.display = 'none';
      adminView.style.display = '';
      $('whoami').textContent = sess.login;
      renderWorkers(); renderAdmins(); renderPlan();
    } else {
      loginView.style.display = '';
      adminView.style.display = 'none';
    }
  }

  $('loginForm').addEventListener('submit', function (e) {
    e.preventDefault();
    var ok = S.login($('li-login').value.trim(), $('li-pass').value);
    if (ok) { $('loginErr').textContent = ''; $('loginForm').reset(); showGate(); }
    else { $('loginErr').textContent = 'Неверный логин или пароль.'; }
  });
  $('logoutBtn').addEventListener('click', function () { S.logout(); showGate(); });

  /* ---------------- вкладки ---------------- */
  var tabs = document.querySelectorAll('.tab');
  tabs.forEach(function (t) {
    t.addEventListener('click', function () {
      tabs.forEach(function (x) { x.classList.remove('on'); });
      t.classList.add('on');
      document.querySelectorAll('.tabpane').forEach(function (p) { p.classList.remove('on'); });
      $('pane-' + t.getAttribute('data-tab')).classList.add('on');
    });
  });

  /* ---------------- работники ---------------- */
  function renderWorkers() {
    var list = S.getWorkers();
    var html = list.map(function (w, i) {
      var nb = S.isNewbie(w) ? '<span class="tag-new">Новичок</span>' : '';
      return '<div class="list-item">' +
        '<span class="li-ava">' + esc(initials(w.name)) + '</span>' +
        '<span class="li-body"><span class="li-name">' + esc(w.name) + nb + '</span>' +
        '<span class="li-meta">' + esc(w.phone || '') + '</span></span>' +
        '<button class="btn-del" data-wi="' + i + '" type="button">Удалить</button>' +
        '</div>';
    }).join('');
    $('workersList').innerHTML = html || '<p class="muted">Пока нет работников.</p>';
  }

  $('workerForm').addEventListener('submit', function (e) {
    e.preventDefault();
    S.addWorker($('w-name').value.trim(), $('w-phone').value.trim());
    $('workerForm').reset();
    renderWorkers();
    toast('Работник добавлен');
  });
  $('workersList').addEventListener('click', function (e) {
    var b = e.target.closest('[data-wi]');
    if (!b) return;
    if (confirm('Удалить работника?')) { S.removeWorker(+b.getAttribute('data-wi')); renderWorkers(); toast('Удалено'); }
  });

  /* ---------------- администраторы ---------------- */
  function renderAdmins() {
    var list = S.getAdmins();
    var html = list.map(function (a) {
      var tag = a.primary ? '<span class="tag-primary">Главный</span>' : '';
      var del = a.primary ? '' : '<button class="btn-del" data-al="' + esc(a.login) + '" type="button">Удалить</button>';
      return '<div class="list-item">' +
        '<span class="li-ava">' + esc(initials(a.login)) + '</span>' +
        '<span class="li-body"><span class="li-name">' + esc(a.login) + tag + '</span>' +
        '<span class="li-meta">пароль: ' + esc('•'.repeat(Math.max(4, (a.pass || '').length))) + '</span></span>' +
        del + '</div>';
    }).join('');
    $('adminsList').innerHTML = html;
  }

  $('adminForm').addEventListener('submit', function (e) {
    e.preventDefault();
    var login = $('a-login').value.trim(), pass = $('a-pass').value;
    if (!login || !pass) return;
    if (S.addAdmin(login, pass)) { $('adminForm').reset(); $('adminErr').textContent = ''; renderAdmins(); toast('Администратор добавлен'); }
    else { $('adminErr').textContent = 'Такой логин уже есть.'; }
  });
  $('adminsList').addEventListener('click', function (e) {
    var b = e.target.closest('[data-al]');
    if (!b) return;
    if (confirm('Удалить администратора?')) { S.removeAdmin(b.getAttribute('data-al')); renderAdmins(); toast('Удалено'); }
  });

  /* ---------------- карта (редактор) ---------------- */
  var NS = 'http://www.w3.org/2000/svg';
  var CELL = 34, GAP = 5, ROWS = 2, STEP = CELL + GAP, SIDE = 24, TOP = 54, BLOCK_GAP = 22, LABEL_H = 20;
  var wrap = $('plan-wrap');

  function renderPlan() {
    var alleys = S.getAlleys(), works = S.getWorks();
    var blocks = [], y = TOP, maxW = 0;
    alleys.forEach(function (a) {
      var cols = Math.ceil(a.count / ROWS), w = cols * STEP - GAP;
      blocks.push({ a: a, cols: cols, w: w, y: y });
      if (w > maxW) maxW = w;
      y += LABEL_H + ROWS * STEP - GAP + BLOCK_GAP;
    });
    var W = maxW + SIDE * 2, H = y + 30;

    var svg = '<svg id="plan" xmlns="' + NS + '" viewBox="0 0 ' + W + ' ' + H + '" width="' + W + '" height="' + H + '">';
    svg += '<rect x="4" y="4" width="' + (W - 8) + '" height="' + (H - 8) + '" rx="16" fill="none" stroke="var(--line)" stroke-width="1.5"/>';
    svg += '<text class="plan-title" x="' + (W / 2) + '" y="34">План СНТ «Михайловское»</text>';
    blocks.forEach(function (b) {
      var startX = (W - b.w) / 2;
      svg += '<text class="alley-label" x="' + (W / 2) + '" y="' + (b.y + 13) + '">' + esc(b.a.label) + '</text>';
      var top = b.y + LABEL_H;
      for (var i = 0; i < b.a.count; i++) {
        var row = i < b.cols ? 0 : 1, col = i < b.cols ? i : i - b.cols, n = i + 1;
        var x = startX + col * STEP, py = top + row * STEP, key = b.a.label + ' ' + n;
        var st = works[key] ? works[key].status : 'none';
        svg += '<g class="plot" data-status="' + st + '" data-key="' + esc(key) + '" tabindex="0" role="button" aria-label="' + esc(key) + '">' +
          '<rect class="cell" x="' + x + '" y="' + py + '" width="' + CELL + '" height="' + CELL + '" rx="7"/>' +
          '<circle class="dot" cx="' + (x + CELL - 6) + '" cy="' + (py + 6) + '" r="3.6"/>' +
          '<text class="num" x="' + (x + CELL / 2) + '" y="' + (py + CELL / 2 + 1) + '">' + n + '</text></g>';
      }
    });
    svg += '</svg>';
    wrap.innerHTML = svg;
  }

  var back = $('panelBack'), panel = $('panel'), pTitle = $('pTitle'), pSub = $('pSub'), pBody = $('pBody');
  var selected = null, editingKey = null;

  function openEditor(key) {
    editingKey = key;
    var rec = S.getWorks()[key] || { status: 'none', workers: [], work: '', date: '' };
    var parts = key.split(' '), num = parts.pop(), alley = parts.join(' ');
    pTitle.textContent = 'Участок №' + num;
    pSub.textContent = alley;

    var statusHtml = STATUSES.map(function (s) {
      return '<label><input type="radio" name="st" value="' + s.id + '"' + (rec.status === s.id ? ' checked' : '') +
        '><span class="d ' + s.id + '" style="background:var(--st-' + s.id + ')"></span>' + s.label + '</label>';
    }).join('');

    var workers = S.getWorkers();
    var chosen = rec.workers || [];
    var workersHtml = workers.length
      ? workers.map(function (w) {
          var on = chosen.indexOf(w.name) !== -1;
          return '<label><input type="checkbox" value="' + esc(w.name) + '"' + (on ? ' checked' : '') + '> ' + esc(w.name) + '</label>';
        }).join('')
      : '<p class="muted">Нет работников. Добавьте их во вкладке «Работники».</p>';

    pBody.innerHTML =
      '<div class="panel-row"><div class="k">Статус (цвет точки)</div><div class="status-pick" id="ed-status">' + statusHtml + '</div></div>' +
      '<div class="panel-row"><div class="k">Кто работал</div><div class="workers-pick" id="ed-workers">' + workersHtml + '</div></div>' +
      '<div class="field" style="margin-top:18px"><label for="ed-work">Что делали</label><textarea id="ed-work" placeholder="Например: покос травы, уборка мусора">' + esc(rec.work || '') + '</textarea></div>' +
      '<div class="field"><label for="ed-date">Когда (необязательно)</label><input id="ed-date" placeholder="например, июнь 2026" value="' + esc(rec.date || '') + '"></div>' +
      '<div class="panel-cta"><button class="btn btn-sun" id="ed-save" type="button">Сохранить</button></div>' +
      '<div style="margin-top:10px"><button class="btn btn-ghost" id="ed-reset" type="button" style="width:100%">Сбросить (серый, без работ)</button></div>';

    $('ed-save').addEventListener('click', saveEditor);
    $('ed-reset').addEventListener('click', resetEditor);

    back.classList.add('open'); panel.classList.add('open');
    if (selected) selected.classList.remove('sel');
    selected = wrap.querySelector('.plot[data-key="' + cssEsc(key) + '"]');
    if (selected) selected.classList.add('sel');
  }

  function saveEditor() {
    var st = (document.querySelector('#ed-status input:checked') || {}).value || 'none';
    var workers = Array.prototype.map.call(document.querySelectorAll('#ed-workers input:checked'), function (i) { return i.value; });
    var work = $('ed-work').value.trim();
    var date = $('ed-date').value.trim();
    if (st === 'none' && !work && !workers.length) {
      S.setWork(editingKey, null);
    } else {
      S.setWork(editingKey, { status: st, workers: workers, work: work, date: date });
    }
    renderPlan(); closePanel(); toast('Сохранено');
  }

  function resetEditor() {
    S.setWork(editingKey, null);
    renderPlan(); closePanel(); toast('Сброшено');
  }

  function closePanel() {
    back.classList.remove('open'); panel.classList.remove('open');
    if (selected) { selected.classList.remove('sel'); selected = null; }
    editingKey = null;
  }

  wrap.addEventListener('click', function (e) {
    var g = e.target.closest ? e.target.closest('.plot') : null;
    if (g) openEditor(g.getAttribute('data-key'));
  });
  wrap.addEventListener('keydown', function (e) {
    if ((e.key === 'Enter' || e.key === ' ') && e.target.classList && e.target.classList.contains('plot')) {
      e.preventDefault(); openEditor(e.target.getAttribute('data-key'));
    }
  });
  back.addEventListener('click', closePanel);
  $('panelClose').addEventListener('click', closePanel);
  document.addEventListener('keydown', function (e) { if (e.key === 'Escape') closePanel(); });

  /* ---------------- старт ---------------- */
  showGate();
})();
