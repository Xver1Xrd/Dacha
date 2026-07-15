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
  function pluralClients(n) {
    var n100 = Math.abs(n) % 100, n10 = n100 % 10;
    if (n100 > 10 && n100 < 20) return 'клиентов';
    if (n10 > 1 && n10 < 5) return 'клиента';
    if (n10 === 1) return 'клиент';
    return 'клиентов';
  }

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

  var editingWorkerId = null;

  /* ---------------- вход ---------------- */
  var loginView = $('loginView'), adminView = $('adminView');

  function showGate() {
    var sess = S.session();
    if (sess) {
      loginView.style.display = 'none';
      adminView.style.display = '';
      $('whoami').textContent = sess.login;
      renderWorkers(); renderAdmins(); renderPlan(); renderReviews();
    } else {
      loginView.style.display = '';
      adminView.style.display = 'none';
    }
  }

  $('loginForm').addEventListener('submit', function (e) {
    e.preventDefault();
    S.login($('li-login').value.trim(), $('li-pass').value).then(function (ok) {
      if (ok) { $('loginErr').textContent = ''; $('loginForm').reset(); showGate(); }
      else { $('loginErr').textContent = 'Неверный логин или пароль.'; }
    });
  });
  $('logoutBtn').addEventListener('click', function () { S.logout().then(showGate); });

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
    var html = list.map(function (w) {
      var nb = S.isNewbie(w) ? '<span class="tag-new">Новичок</span>' : '';
      var tg = w.telegram ? (' · <a href="' + esc(w.telegram) + '" target="_blank" rel="noopener" style="color:#229ed9">Telegram</a>') : '';
      var clients = ' · ' + (w.clients || 0) + ' ' + pluralClients(w.clients || 0);
      return '<div class="list-item">' +
        '<span class="li-ava">' + esc(initials(w.name)) + '</span>' +
        '<span class="li-body"><span class="li-name">' + esc(w.name) + nb + '</span>' +
        '<span class="li-meta">' + esc(w.phone || '') + tg + clients + '</span></span>' +
        '<div class="list-actions">' +
        '<button class="btn-edit" data-we="' + w.id + '" type="button">✎</button>' +
        '<button class="btn-del" data-wi="' + w.id + '" type="button">Удалить</button></div>' +
        '</div>';
    }).join('');
    $('workersList').innerHTML = html || '<p class="muted">Пока нет работников.</p>';
  }

  $('workerForm').addEventListener('submit', function (e) {
    e.preventDefault();
    var clients = parseInt($('w-clients').value, 10) || 0;
    if (editingWorkerId) {
      S.editWorker(editingWorkerId, $('w-name').value.trim(), $('w-phone').value.trim(), $('w-tg').value.trim(), clients).then(function () {
        $('workerForm').reset(); editingWorkerId = null;
        $('workerForm').querySelector('button[type="submit"]').textContent = 'Добавить';
        renderWorkers(); toast('Сохранено');
      });
    } else {
      S.addWorker($('w-name').value.trim(), $('w-phone').value.trim(), $('w-tg').value.trim(), clients).then(function () {
        $('workerForm').reset(); renderWorkers(); toast('Работник добавлен');
      });
    }
  });

  $('workersList').addEventListener('click', function (e) {
    var del = e.target.closest('[data-wi]');
    if (del) {
      if (confirm('Удалить работника?')) {
        S.removeWorker(del.getAttribute('data-wi')).then(function () { renderWorkers(); toast('Удалено'); });
      }
      return;
    }
    var edit = e.target.closest('[data-we]');
    if (edit) {
      var id = edit.getAttribute('data-we');
      var workers = S.getWorkers();
      var w = workers.filter(function (x) { return x.id == id; })[0];
      if (!w) return;
      $('w-name').value = w.name;
      $('w-phone').value = w.phone || '';
      $('w-tg').value = w.telegram || '';
      $('w-clients').value = w.clients || 0;
      editingWorkerId = id;
      $('workerForm').querySelector('button[type="submit"]').textContent = 'Сохранить';
      $('w-name').focus();
    }
  });

  /* ---------------- администраторы ---------------- */
  function renderPermBadge(p) {
    var m = []; if (p.map) m.push('К'); if (p.workers) m.push('Р'); if (p.admins) m.push('А'); if (p.reviews) m.push('О');
    return m.length ? '<span class="tag-perms">' + m.join('') + '</span>' : '';
  }

  function renderAdmins() {
    S.getAdmins().then(function (list) {
      var html = list.map(function (a) {
        var tag = a.primary ? '<span class="tag-primary">Главный</span>' : '';
        var perms = a.primary ? '' : renderPermBadge(a.perms || {});
        var del = a.primary ? '' : '<button class="btn-del" data-al="' + esc(a.login) + '" type="button">Удалить</button>';
        var pass = '<button class="btn-edit" data-pw="' + esc(a.login) + '" type="button" title="Сменить пароль">🔑</button>';
        return '<div class="list-item">' +
          '<span class="li-ava">' + esc(initials(a.login)) + '</span>' +
          '<span class="li-body"><span class="li-name">' + esc(a.login) + tag + perms + '</span>' +
          '<span class="li-meta">пароль скрыт</span></span>' +
          '<div class="list-actions">' + pass + del + '</div></div>';
      }).join('');
      $('adminsList').innerHTML = html;
    });
  }

  $('adminForm').addEventListener('submit', function (e) {
    e.preventDefault();
    var login = $('a-login').value.trim(), pass = $('a-pass').value;
    if (!login || !pass) return;
    var perms = {};
    document.querySelectorAll('#adminForm [data-perm]').forEach(function (cb) {
      perms[cb.getAttribute('data-perm')] = cb.checked;
    });
    S.addAdmin(login, pass, perms).then(function (ok) {
      if (ok) { $('adminForm').reset(); $('adminErr').textContent = ''; renderAdmins(); toast('Администратор добавлен'); }
      else { $('adminErr').textContent = 'Такой логин уже есть.'; }
    });
  });
  $('adminsList').addEventListener('click', function (e) {
    var pw = e.target.closest('[data-pw]');
    if (pw) {
      var login = pw.getAttribute('data-pw');
      var newPass = prompt('Новый пароль для «' + login + '» (минимум 6 символов):');
      if (newPass === null) return;
      if (newPass.length < 6) { toast('Пароль должен быть не короче 6 символов'); return; }
      S.setAdminPassword(login, newPass).then(function (ok) {
        toast(ok ? 'Пароль изменён' : 'Не удалось сменить пароль');
      });
      return;
    }
    var b = e.target.closest('[data-al]');
    if (!b) return;
    if (confirm('Удалить администратора?')) {
      S.removeAdmin(b.getAttribute('data-al')).then(function () { renderAdmins(); toast('Удалено'); });
    }
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
    var rec = (st === 'none' && !work && !workers.length) ? null : { status: st, workers: workers, work: work, date: date };
    S.setWork(editingKey, rec).then(function () { renderPlan(); closePanel(); toast('Сохранено'); });
  }

  function resetEditor() {
    S.setWork(editingKey, null).then(function () { renderPlan(); closePanel(); toast('Сброшено'); });
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

  /* ---------------- отзывы ---------------- */
  var editingReviewId = null;

  function renderReviews() {
    var list = S.getReviews();
    var html = list.map(function (r, i) {
      var stars = '';
      for (var j = 0; j < 5; j++) {
        stars += '<svg viewBox="0 0 24 24" fill="' + (j < r.stars ? 'currentColor' : 'none') + '" stroke="currentColor" stroke-width="2" style="width:16px;height:16px;color:var(--sun)"><path d="M12 2l2.4 7.4H22l-6 4.6 2.3 7.4L12 16.9 5.7 21.4 8 14 2 9.4h7.6z"/></svg>';
      }
      var colorStyle = r.color ? 'style="background:' + r.color + '"' : '';
      var upBtn = i > 0 ? '<button class="btn-edit" data-move-up="' + r.id + '" type="button" title="Выше" aria-label="Переместить выше">&uarr;</button>' : '';
      var downBtn = i < list.length - 1 ? '<button class="btn-edit" data-move-down="' + r.id + '" type="button" title="Ниже" aria-label="Переместить ниже">&darr;</button>' : '';
      return '<div class="list-item">' +
        '<span class="li-ava" ' + colorStyle + '>' + esc(initials(r.name)) + '</span>' +
        '<span class="li-body"><span class="li-name">' + esc(r.name) + '</span>' +
        '<span class="li-meta">' + stars + '</span>' +
        '<span class="li-text" style="font-size:.85rem;color:var(--soil-soft);margin-top:4px;display:block;line-height:1.3">' + esc(r.text) + '</span></span>' +
        '<div class="list-actions">' +
        upBtn + downBtn +
        '<button class="btn-edit" data-re="' + r.id + '" type="button">✎</button>' +
        '<button class="btn-del" data-ri="' + r.id + '" type="button">Удалить</button></div>' +
        '</div>';
    }).join('');
    $('reviewsList').innerHTML = html || '<p class="muted">Пока нет отзывов.</p>';
  }

  $('reviewForm').addEventListener('submit', function (e) {
    e.preventDefault();
    var name = $('r-name').value.trim();
    var text = $('r-text').value.trim();
    var stars = parseInt($('r-stars').value) || 5;
    if (editingReviewId) {
      S.editReview(editingReviewId, name, text, stars, '').then(function () {
        $('reviewForm').reset(); editingReviewId = null;
        $('reviewForm').querySelector('button[type="submit"]').textContent = 'Добавить';
        renderReviews(); toast('Сохранено');
      });
    } else {
      S.addReview(name, text, stars).then(function () {
        $('reviewForm').reset(); renderReviews(); toast('Отзыв добавлен');
      });
    }
  });
  $('reviewsList').addEventListener('click', function (e) {
    var up = e.target.closest('[data-move-up]');
    if (up) {
      S.moveReview(up.getAttribute('data-move-up'), 'up').then(function () { renderReviews(); });
      return;
    }
    var down = e.target.closest('[data-move-down]');
    if (down) {
      S.moveReview(down.getAttribute('data-move-down'), 'down').then(function () { renderReviews(); });
      return;
    }
    var b = e.target.closest('[data-ri]');
    if (b) {
      if (confirm('Удалить отзыв?')) {
        S.removeReview(b.getAttribute('data-ri')).then(function () { renderReviews(); toast('Удалено'); });
      }
      return;
    }
    var edit = e.target.closest('[data-re]');
    if (edit) {
      var id = edit.getAttribute('data-re');
      var reviews = S.getReviews();
      var r = reviews.filter(function (x) { return x.id == id; })[0];
      if (!r) return;
      $('r-name').value = r.name;
      $('r-text').value = r.text;
      $('r-stars').value = r.stars;
      editingReviewId = id;
      $('reviewForm').querySelector('button[type="submit"]').textContent = 'Сохранить';
      $('r-name').focus();
    }
  });

  /* ---------------- старт ---------------- */
  S.ready(showGate);
})();