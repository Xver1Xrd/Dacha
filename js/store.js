/* ============================================================
   ХРАНИЛИЩЕ ДАННЫХ САЙТА (SNTStore)
   ------------------------------------------------------------
   Данные хранятся в localStorage браузера. Значения по умолчанию
   («сид») берутся из map-data.js — поэтому map-data.js нужно
   подключать ДО store.js.

   ВАЖНО: localStorage привязан к конкретному браузеру/устройству.
   Правки администратора видны там, где он их сделал. Чтобы данные
   были общими для всех посетителей с любых устройств, нужен бэкенд
   (например, Firebase / Supabase) — это отдельная доработка.
   ============================================================ */
(function () {
  'use strict';

  var KEYS = {
    alleys: 'snt_alleys',
    works: 'snt_works',
    workers: 'snt_workers',
    admins: 'snt_admins',
    session: 'snt_session'
  };
  var WEEK = 7 * 24 * 60 * 60 * 1000;

  function load(k, def) {
    try { var v = localStorage.getItem(k); return v ? JSON.parse(v) : def; }
    catch (e) { return def; }
  }
  function save(k, v) {
    try { localStorage.setItem(k, JSON.stringify(v)); return true; }
    catch (e) { return false; }
  }

  var DEFAULT_WORKERS = [
    { name: 'Ярослав', phone: '+7 967 592 58 71', addedAt: 0 },
    { name: 'Роман',   phone: '+7 981 204 11 78', addedAt: 0 },
    { name: 'Денис',   phone: '+7 950 029 03 98', addedAt: 0 }
  ];
  // Первый (главный) администратор. Логин/пароль смените после первого входа.
  var DEFAULT_ADMINS = [{ login: 'admin', pass: 'admin', primary: true }];

  var Store = {
    KEYS: KEYS,
    WEEK: WEEK,

    getAlleys: function () { return load(KEYS.alleys, window.SNT_ALLEYS || []); },
    setAlleys: function (v) { return save(KEYS.alleys, v); },

    getWorks: function () { return load(KEYS.works, window.SNT_WORKS || {}); },
    setWorks: function (v) { return save(KEYS.works, v); },
    setWork: function (key, rec) {
      var w = this.getWorks();
      if (rec === null) { delete w[key]; } else { w[key] = rec; }
      return this.setWorks(w);
    },

    getWorkers: function () { return load(KEYS.workers, DEFAULT_WORKERS); },
    setWorkers: function (v) { return save(KEYS.workers, v); },
    addWorker: function (name, phone) {
      var list = this.getWorkers();
      list.push({ name: name, phone: phone, addedAt: Date.now() });
      return this.setWorkers(list);
    },
    removeWorker: function (idx) {
      var list = this.getWorkers();
      list.splice(idx, 1);
      return this.setWorkers(list);
    },
    isNewbie: function (w) { return !!w.addedAt && (Date.now() - w.addedAt) < WEEK; },

    getAdmins: function () { return load(KEYS.admins, DEFAULT_ADMINS); },
    setAdmins: function (v) { return save(KEYS.admins, v); },
    addAdmin: function (login, pass) {
      var list = this.getAdmins();
      if (list.some(function (a) { return a.login === login; })) return false;
      list.push({ login: login, pass: pass, primary: false });
      return this.setAdmins(list);
    },
    removeAdmin: function (login) {
      var list = this.getAdmins().filter(function (a) { return !(a.login === login && !a.primary); });
      return this.setAdmins(list);
    },

    login: function (login, pass) {
      var a = this.getAdmins().filter(function (x) { return x.login === login && x.pass === pass; })[0];
      if (a) { save(KEYS.session, { login: login, at: Date.now() }); return true; }
      return false;
    },
    session: function () { return load(KEYS.session, null); },
    logout: function () { try { localStorage.removeItem(KEYS.session); } catch (e) {} }
  };

  window.SNTStore = Store;
})();
