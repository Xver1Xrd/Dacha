document.documentElement.classList.add('js');

var ACCENTS = ['var(--green)', 'var(--sun)', '#3f7d2e', 'var(--soil)'];

(function () {
  var track = document.getElementById('carouselTrack');
  var dots = document.getElementById('carouselDots');
  var prev = document.querySelector('.carousel-prev');
  var next = document.querySelector('.carousel-next');
  if (!track || !window.SNTStore) return;
  function initName(n) { return n.trim().charAt(0).toUpperCase(); }
  function esc(s) {
    return String(s).replace(/[&<>"]/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c];
    });
  }
  window.SNTStore.ready(function () {
    var reviews = window.SNTStore.getReviews();
    if (!reviews.length) { track.innerHTML = '<p class="muted" style="text-align:center;padding:2rem 0">Пока нет отзывов.</p>'; return; }

    function slideHtml(r, isClone) {
      var stars = '';
      for (var j = 0; j < 5; j++) {
        stars += '<svg viewBox="0 0 24 24" fill="' + (j < r.stars ? 'currentColor' : 'none') + '" stroke="currentColor" stroke-width="2" style="width:18px;height:18px;color:var(--sun)"><path d="M12 2l2.4 7.4H22l-6 4.6 2.3 7.4L12 16.9 5.7 21.4 8 14 2 9.4h7.6z"/></svg>';
      }
      var colorStyle = r.color ? 'style="background:' + esc(r.color) + '"' : '';
      var hidden = isClone ? ' aria-hidden="true"' : '';
      return '<div class="carousel-slide card-rev"' + hidden + '>' +
        '<span class="quote-mark">\u00AB</span>' +
        '<div class="stars" aria-label="' + r.stars + ' \u0438\u0437 5">' + stars + '</div>' +
        '<p class="txt">' + esc(r.text) + '</p>' +
        '<div class="rev-author"><span class="rev-ava" ' + colorStyle + '>' + esc(initName(r.name)) + '</span>' +
        '<span class="rev-meta"><b>' + esc(r.name) + '</b><span>\u0421\u041D\u0422 \u00AB\u041C\u0438\u0445\u0430\u0439\u043B\u043E\u0432\u0441\u043A\u043E\u0435\u00BB</span></span></div></div>';
    }

    var numReal = reviews.length;
    var realHtml = reviews.map(function (r) { return slideHtml(r, false); }).join('');
    track.innerHTML = realHtml;

    var dotHtml = reviews.map(function (_, i) {
      return '<button class="carousel-dot' + (i === 0 ? ' active' : '') + '" data-index="' + i + '" type="button" aria-label="\u041E\u0442\u0437\u044B\u0432 ' + (i + 1) + '"></button>';
    }).join('');
    dots.innerHTML = dotHtml;

    // \u0437\u0430\u0446\u0438\u043A\u043B\u0438\u0432\u0430\u0435\u043c \u0442\u043e\u043b\u044c\u043a\u043e \u0435\u0441\u043b\u0438 \u0440\u0435\u0430\u043b\u044c\u043d\u043e \u0435\u0441\u0442\u044c \u043f\u0440\u043e\u043a\u0440\u0443\u0442\u043a\u0430 \u0438 \u0431\u043e\u043b\u044c\u0448\u0435 \u043e\u0434\u043d\u043e\u0433\u043e \u043e\u0442\u0437\u044b\u0432\u0430 -
    // \u043a\u043b\u043e\u043d\u0438\u0440\u0443\u0435\u043c \u043d\u0435\u0441\u043a\u043e\u043b\u044c\u043a\u043e \u043a\u0430\u0440\u0442\u043e\u0447\u0435\u043a \u0441 \u043a\u0430\u0436\u0434\u043e\u0439 \u0441\u0442\u043e\u0440\u043e\u043d\u044b, \u0447\u0442\u043e\u0431\u044b \u0441\u0432\u0430\u0439\u043f \u201c\u0443\u0442\u044b\u043a\u0430\u043b\u0441\u044f\u201d
    // \u043d\u0435 \u0432 \u043a\u0440\u0430\u0439, \u0430 \u0431\u0435\u0441\u0448\u043e\u0432\u043d\u043e \u043f\u0435\u0440\u0435\u0442\u0435\u043a\u0430\u043b \u043e\u0431\u0440\u0430\u0442\u043d\u043e \u043a \u043d\u0430\u0447\u0430\u043b\u0443
    var scrollable = track.scrollWidth > track.clientWidth + 4;
    // клонов должно хватать, чтобы прокрутка физически могла дотянуться до
    // клон-зоны - на широких экранах в кадре сразу несколько карточек, и
    // фиксированных 1-2 клонов недостаточно (браузер просто не даёт
    // проскроллить дальше, если после последней карточки не хватает "хвоста")
    var realSlides = [].slice.call(track.children);
    var cardSpan = realSlides.length > 1 ? realSlides[1].offsetLeft - realSlides[0].offsetLeft : track.clientWidth;
    var perScreen = Math.max(1, Math.ceil(track.clientWidth / cardSpan));
    var CLONES = Math.min(numReal - 1, perScreen);
    var loopable = scrollable && numReal > 1 && CLONES > 0;

    if (loopable) {
      var headClones = reviews.slice(-CLONES).map(function (r) { return slideHtml(r, true); }).join('');
      var tailClones = reviews.slice(0, CLONES).map(function (r) { return slideHtml(r, true); }).join('');
      track.innerHTML = headClones + realHtml + tailClones;
    }

    var slides = [].slice.call(track.children);
    var realStart = loopable ? CLONES : 0;
    var current = realStart, timer = null;
    var reduceMotion = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;

    function slideStep() {
      return slides.length > 1 ? slides[1].offsetLeft - slides[0].offsetLeft : track.clientWidth;
    }

    function realIndexOf(visualIndex) {
      return ((visualIndex - realStart) % numReal + numReal) % numReal;
    }

    function setActiveDot(realIndex) {
      [].forEach.call(dots.children, function (d, k) {
        d.classList.toggle('active', k === realIndex);
      });
    }

    function maxScroll() { return track.scrollWidth - track.clientWidth; }

    function scrollToIndex(index, smooth) {
      var left = slides[index].offsetLeft - slides[0].offsetLeft;
      if (left < 0) left = 0;
      if (left > maxScroll()) left = maxScroll();
      track.scrollTo({ left: left, behavior: smooth && !reduceMotion ? 'smooth' : 'auto' });
    }

    function goTo(index) {
      if (loopable) {
        if (index < 0) index = 0;
        if (index > slides.length - 1) index = slides.length - 1;
      } else {
        index = ((index % numReal) + numReal) % numReal;
      }
      current = index;
      scrollToIndex(index, true);
      setActiveDot(realIndexOf(index));
    }

    function nextSlide() { goTo(current + 1); }
    function prevSlide() { goTo(current - 1); }

    // \u043f\u043e\u0441\u043b\u0435 \u0442\u043e\u0433\u043e \u043a\u0430\u043a \u0441\u043a\u0440\u043e\u043b\u043b "\u0443\u0441\u0442\u0430\u043a\u0430\u043d\u0438\u043b\u0441\u044f" - \u0435\u0441\u043b\u0438 \u0443\u0435\u0445\u0430\u043b\u0438 \u0432 \u043a\u043b\u043e\u043d-\u0437\u043e\u043d\u0443,
    // \u0431\u0435\u0441\u0448\u043e\u0432\u043d\u043e \u0442\u0435\u043b\u0435\u043f\u043e\u0440\u0442\u0438\u0440\u0443\u0435\u043c\u0441\u044f \u043d\u0430 \u0442\u0430\u043a\u043e\u0439 \u0436\u0435 \u0440\u0435\u0430\u043b\u044c\u043d\u044b\u0439 \u0441\u043b\u0430\u0439\u0434
    function settle() {
      if (!loopable) return;
      var i = Math.max(0, Math.min(slides.length - 1, Math.round(track.scrollLeft / slideStep())));
      if (i >= realStart + numReal) {
        current = i - numReal;
        scrollToIndex(current, false);
      } else if (i < realStart) {
        current = i + numReal;
        scrollToIndex(current, false);
      } else {
        current = i;
      }
      setActiveDot(realIndexOf(current));
    }

    var settleTimer = null;
    track.addEventListener('scroll', function () {
      var i = Math.max(0, Math.min(slides.length - 1, Math.round(track.scrollLeft / slideStep())));
      setActiveDot(realIndexOf(i));
      clearTimeout(settleTimer);
      settleTimer = setTimeout(settle, 120);
    }, { passive: true });

    function startTimer() {
      stopTimer();
      if (reduceMotion || !scrollable) return;
      timer = setInterval(function () {
        if (!document.hidden) nextSlide();
      }, 5000);
    }
    function stopTimer() { if (timer) { clearInterval(timer); timer = null; } }

    if (prev) prev.addEventListener('click', function () { prevSlide(); startTimer(); });
    if (next) next.addEventListener('click', function () { nextSlide(); startTimer(); });
    dots.addEventListener('click', function (e) {
      var dot = e.target.closest('.carousel-dot');
      if (dot) { goTo(realStart + parseInt(dot.getAttribute('data-index'), 10)); startTimer(); }
    });

    var carousel = document.getElementById('reviewsCarousel');
    if (carousel) {
      carousel.addEventListener('mouseenter', stopTimer);
      carousel.addEventListener('mouseleave', startTimer);
      carousel.addEventListener('touchstart', stopTimer, { passive: true });
      carousel.addEventListener('touchend', startTimer, { passive: true });
      carousel.addEventListener('focusin', stopTimer);
      carousel.addEventListener('focusout', startTimer);
    }

    // \u0435\u0441\u043b\u0438 \u0432\u0441\u0435 \u043e\u0442\u0437\u044b\u0432\u044b \u0432\u043b\u0435\u0437\u0430\u044e\u0442 \u0431\u0435\u0437 \u043f\u0440\u043e\u043a\u0440\u0443\u0442\u043a\u0438 \u2014 \u0443\u043f\u0440\u0430\u0432\u043b\u0435\u043d\u0438\u0435 \u043d\u0435 \u043d\u0443\u0436\u043d\u043e
    function updateControls() {
      dots.style.display = scrollable ? '' : 'none';
      if (prev) prev.style.display = scrollable ? '' : 'none';
      if (next) next.style.display = scrollable ? '' : 'none';
      if (scrollable) startTimer(); else stopTimer();
    }
    updateControls();
    window.addEventListener('resize', function () {
      scrollToIndex(current, false);
      updateControls();
    });

    if (loopable) scrollToIndex(realStart, false);
  });
})();

(function () {
  var grid = document.getElementById('greetersGrid');
  if (!grid || !window.SNTStore) return;
  function initName(n) { return n.trim().charAt(0).toUpperCase(); }
  function esc(s) {
    return String(s).replace(/[&<>"]/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c];
    });
  }
  function pluralClients(n) {
    var n100 = Math.abs(n) % 100, n10 = n100 % 10;
    if (n100 > 10 && n100 < 20) return 'клиентов';
    if (n10 > 1 && n10 < 5) return 'клиента';
    if (n10 === 1) return 'клиент';
    return 'клиентов';
  }
  window.SNTStore.ready(function () {
    var workers = window.SNTStore.getWorkers();
    var html = workers.map(function (w, i) {
      var accent = ACCENTS[i % ACCENTS.length];
      var phoneClean = (w.phone || '').replace(/[\s\-]/g, '');
      var tgHref = w.telegram || 'https://t.me/' + phoneClean;
      var clientsLine = w.clients ? '<span class="p-role">' + w.clients + ' ' + pluralClients(w.clients) + '</span>' : '';
      return '<div class="card-person anim" style="--accent:' + accent + ';--d:' + (560 + i * 100) + 'ms">' +
        '<span class="avatar" style="--accent:' + accent + '">' + esc(initName(w.name)) + '</span>' +
        '<span class="p-body">' +
        '<span class="p-name">' + esc(w.name) + '</span>' +
        clientsLine +
        '<a href="tel:' + esc(phoneClean) + '" class="p-phone"><svg viewBox="0 0 24 24" fill="none"><path d="M6.6 10.8a15 15 0 006.6 6.6l2.2-2.2a1 1 0 011-.24 11.4 11.4 0 003.6.58 1 1 0 011 1V20a1 1 0 01-1 1A17 17 0 013 4a1 1 0 011-1h3.5a1 1 0 011 1c0 1.25.2 2.46.58 3.6a1 1 0 01-.24 1l-2.24 2.2z" fill="currentColor"/></svg> ' + esc(w.phone || '') + '</a>' +
        '<a href="' + esc(tgHref) + '" class="p-tg" target="_blank" rel="noopener"><svg viewBox="0 0 24 24" fill="currentColor"><path d="M11.944 0A12 12 0 000 12a12 12 0 0012 12 12 12 0 0012-12A12 12 0 0012 0a12 12 0 00-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 01.171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z"/></svg> Telegram</a>' +
        '</span>' +
        '<a href="tel:' + esc(phoneClean) + '" class="p-call" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none"><path d="M6.6 10.8a15 15 0 006.6 6.6l2.2-2.2a1 1 0 011-.24 11.4 11.4 0 003.6.58 1 1 0 011 1V20a1 1 0 01-1 1A17 17 0 013 4a1 1 0 011-1h3.5a1 1 0 011 1c0 1.25.2 2.46.58 3.6a1 1 0 01-.24 1l-2.24 2.2z" fill="currentColor"/></svg></a>' +
        '</div>';
    }).join('');
    grid.innerHTML = html;
  });
})();

(function () {
  var grid = document.getElementById('phonesGrid');
  if (!grid || !window.SNTStore) return;
  function initName(n) { return n.trim().charAt(0).toUpperCase(); }
  function esc(s) {
    return String(s).replace(/[&<>"]/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c];
    });
  }
  window.SNTStore.ready(function () {
    var workers = window.SNTStore.getWorkers();
    var html = workers.map(function (w, i) {
      var accent = ACCENTS[i % ACCENTS.length];
      var phoneClean = (w.phone || '').replace(/[\s\-]/g, '');
      var tgHref = w.telegram || 'https://t.me/' + phoneClean;
      return '<div class="phone-line reveal" data-stagger style="--accent:' + accent + ';--d:' + (i * 90) + 'ms">' +
        '<span class="pl-ava" style="--accent:' + accent + '">' + esc(initName(w.name)) + '</span>' +
        '<span class="pl-body"><span class="pl-name">' + esc(w.name) + '</span><span class="pl-num">' + esc(w.phone || '') + '</span></span>' +
        '<div class="pl-actions">' +
        '<a href="tel:' + esc(phoneClean) + '" class="pl-call" aria-label="\u041F\u043E\u0437\u0432\u043E\u043D\u0438\u0442\u044C"><svg viewBox="0 0 24 24" fill="none"><path d="M6.6 10.8a15 15 0 006.6 6.6l2.2-2.2a1 1 0 011-.24 11.4 11.4 0 003.6.58 1 1 0 011 1V20a1 1 0 01-1 1A17 17 0 013 4a1 1 0 011-1h3.5a1 1 0 011 1c0 1.25.2 2.46.58 3.6a1 1 0 01-.24 1l-2.24 2.2z" fill="currentColor"/></svg></a>' +
        '<a href="' + esc(tgHref) + '" class="pl-tg" target="_blank" rel="noopener" aria-label="Telegram"><svg viewBox="0 0 24 24" fill="currentColor"><path d="M11.944 0A12 12 0 000 12a12 12 0 0012 12 12 12 0 0012-12A12 12 0 0012 0a12 12 0 00-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 01.171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z"/></svg></a>' +
        '</div></div>';
    }).join('');
    grid.innerHTML = html;
    if ('IntersectionObserver' in window && window.__reviewIO) {
      grid.querySelectorAll('.reveal').forEach(function (el) { window.__reviewIO.observe(el); });
    }
  });
})();

var themeBtn = document.getElementById('themeBtn');
if (themeBtn) {
  themeBtn.addEventListener('click', function () {
    var root = document.documentElement;
    var next = root.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
    root.setAttribute('data-theme', next);
    try { localStorage.setItem('theme', next); } catch (e) {}
  });
}

(function () {
  var fab = document.querySelector('.fab');
  if (!fab || !('IntersectionObserver' in window)) return;
  // прячем плавающую кнопку звонка там, где рядом и так есть свои кнопки
  // звонка (карточки работников, блок "Звоните - договоримся", футер) -
  // иначе она перекрывает их на мобильных экранах
  var zones = document.querySelectorAll('.greeters, .reviews, .contact, footer.foot');
  if (!zones.length) return;
  var visible = new Set();
  var fabIO = new IntersectionObserver(function (entries) {
    entries.forEach(function (e) {
      if (e.isIntersecting) visible.add(e.target); else visible.delete(e.target);
    });
    fab.classList.toggle('fab-hide', visible.size > 0);
  }, { threshold: .01 });
  zones.forEach(function (z) { fabIO.observe(z); });
})();

if ('IntersectionObserver' in window) {
  var io = new IntersectionObserver(function (entries) {
    entries.forEach(function (e) {
      if (e.isIntersecting) {
        e.target.classList.add('in');
        io.unobserve(e.target);
      }
    });
  }, { threshold: .14, rootMargin: '0px 0px -8% 0px' });
  document.querySelectorAll('.reveal').forEach(function (el) { io.observe(el); });
  window.__reviewIO = io;
} else {
  document.querySelectorAll('.reveal').forEach(function (el) { el.classList.add('in'); });
}