/* ============================================================
   КЛИЕНТ БЭКЕНДА (SNTStore)
   ------------------------------------------------------------
   Данные приходят с сервера (Go, REST API). Публичные данные
   (карта, работники) кэшируются в памяти после загрузки —
   геттеры синхронные, мутации асинхронные (возвращают Promise).
   Используйте SNTStore.ready(cb), чтобы дождаться первой загрузки.
   ============================================================ */
(function () {
  'use strict';

  var cache = { alleys: [], works: {}, workers: [], reviews: [], session: null };
  var readyCbs = [];
  var isReady = false;

  function readCookie(name) {
    var m = document.cookie.match('(?:^|; )' + name + '=([^;]*)');
    return m ? decodeURIComponent(m[1]) : '';
  }

  function api(path, opts) {
    opts = opts || {};
    opts.credentials = 'same-origin';
    opts.headers = opts.headers || {};
    if (opts.body !== undefined) {
      opts.headers['Content-Type'] = 'application/json';
      opts.body = JSON.stringify(opts.body);
    }
    var method = (opts.method || 'GET').toUpperCase();
    if (method !== 'GET' && method !== 'HEAD') {
      var csrf = readCookie('snt_csrf');
      if (csrf) opts.headers['X-CSRF-Token'] = csrf;
    }
    return fetch('/api' + path, opts);
  }

  function loadData() {
    return api('/data').then(function (r) { return r.json(); }).then(function (d) {
      cache.alleys = d.alleys || [];
      cache.works = d.works || {};
      cache.workers = d.workers || [];
      cache.reviews = d.reviews || [];
    });
  }
  function loadSession() {
    return api('/session').then(function (r) { return r.ok ? r.json() : null; })
      .then(function (s) { cache.session = s; })
      .catch(function () { cache.session = null; });
  }

  var Store = {
    WEEK: 7 * 24 * 60 * 60 * 1000,

    ready: function (cb) { if (isReady) cb(); else readyCbs.push(cb); },
    refresh: function () { return loadData(); },

    getAlleys: function () { return cache.alleys; },
    getWorks: function () { return cache.works; },
    getWorkers: function () { return cache.workers; },
    getReviews: function () { return cache.reviews; },
    isNewbie: function (w) { return !!w.addedAt && (Date.now() - w.addedAt) < this.WEEK; },
    session: function () { return cache.session; },

    login: function (login, pass) {
      return api('/login', { method: 'POST', body: { login: login, pass: pass } })
        .then(function (r) {
          if (!r.ok) return false;
          return r.json().then(function (s) { cache.session = s; return true; });
        });
    },
    logout: function () {
      return api('/logout', { method: 'POST' }).then(function () { cache.session = null; });
    },

    getAdmins: function () {
      return api('/admins').then(function (r) { return r.ok ? r.json() : []; });
    },
    addAdmin: function (login, pass, perms) {
      return api('/admins', { method: 'POST', body: { login: login, pass: pass, perms: perms || {} } })
        .then(function (r) { return r.ok; });
    },
    removeAdmin: function (login) {
      return api('/admins/' + encodeURIComponent(login), { method: 'DELETE' })
        .then(function (r) { return r.ok; });
    },

    addWorker: function (name, phone, telegram, clients) {
      return api('/workers', { method: 'POST', body: { name: name, phone: phone, telegram: telegram, clients: clients || 0 } })
        .then(function (r) { return r.ok; }).then(function (ok) { return loadData().then(function () { return ok; }); });
    },
    removeWorker: function (id) {
      return api('/workers/' + encodeURIComponent(id), { method: 'DELETE' })
        .then(function (r) { return r.ok; }).then(function (ok) { return loadData().then(function () { return ok; }); });
    },
    editWorker: function (id, name, phone, telegram, clients) {
      return api('/workers/' + encodeURIComponent(id), { method: 'PUT', body: { name: name, phone: phone, telegram: telegram, clients: clients || 0 } })
        .then(function (r) { return r.ok; }).then(function (ok) { return loadData().then(function () { return ok; }); });
    },

    setWork: function (key, rec) {
      return api('/works', { method: 'POST', body: { key: key, rec: rec } })
        .then(function (r) { return r.ok; }).then(function (ok) { return loadData().then(function () { return ok; }); });
    },

    addReview: function (name, text, stars, color) {
      return api('/reviews', { method: 'POST', body: { name: name, text: text, stars: stars || 5, color: color || '' } })
        .then(function (r) { return r.ok; }).then(function (ok) { return loadData().then(function () { return ok; }); });
    },
    removeReview: function (id) {
      return api('/reviews/' + encodeURIComponent(id), { method: 'DELETE' })
        .then(function (r) { return r.ok; }).then(function (ok) { return loadData().then(function () { return ok; }); });
    },
    editReview: function (id, name, text, stars, color) {
      return api('/reviews/' + encodeURIComponent(id), { method: 'PUT', body: { name: name, text: text, stars: stars, color: color || '' } })
        .then(function (r) { return r.ok; })
        .then(function (ok) { return loadData().then(function () { return ok; }); });
    }
  };

  Promise.all([loadData(), loadSession()]).then(finish, finish);
  function finish() { isReady = true; readyCbs.forEach(function (cb) { cb(); }); readyCbs = []; }

  window.SNTStore = Store;
})();
