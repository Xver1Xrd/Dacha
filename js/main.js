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
    var slidesHtml = reviews.map(function (r) {
      var stars = '';
      for (var j = 0; j < 5; j++) {
        stars += '<svg viewBox="0 0 24 24" fill="' + (j < r.stars ? 'currentColor' : 'none') + '" stroke="currentColor" stroke-width="2" style="width:18px;height:18px;color:var(--sun)"><path d="M12 2l2.4 7.4H22l-6 4.6 2.3 7.4L12 16.9 5.7 21.4 8 14 2 9.4h7.6z"/></svg>';
      }
      var colorStyle = r.color ? 'style="background:' + esc(r.color) + '"' : '';
      return '<div class="carousel-slide card-rev">' +
        '<span class="quote-mark">\u00AB</span>' +
        '<div class="stars" aria-label="' + r.stars + ' \u0438\u0437 5">' + stars + '</div>' +
        '<p class="txt">' + esc(r.text) + '</p>' +
        '<div class="rev-author"><span class="rev-ava" ' + colorStyle + '>' + esc(initName(r.name)) + '</span>' +
        '<span class="rev-meta"><b>' + esc(r.name) + '</b><span>\u0421\u041D\u0422 \u00AB\u041C\u0438\u0445\u0430\u0439\u043B\u043E\u0432\u0441\u043A\u043E\u0435\u00BB</span></span></div></div>';
    }).join('');
    track.innerHTML = slidesHtml;
    var dotHtml = reviews.map(function (_, i) {
      return '<button class="carousel-dot' + (i === 0 ? ' active' : '') + '" data-index="' + i + '" type="button" aria-label="\u041E\u0442\u0437\u044B\u0432 ' + (i + 1) + '"></button>';
    }).join('');
    dots.innerHTML = dotHtml;

    var current = 0, timer;

    function goTo(index) {
      var slides = track.children;
      if (!slides.length) return;
      if (index < 0) index = slides.length - 1;
      if (index >= slides.length) index = 0;
      current = index;
      track.style.transform = 'translateX(-' + (current * 100) + '%)';
      [].forEach.call(dots.children, function (d, i) {
        d.classList.toggle('active', i === current);
      });
    }

    function nextSlide() { goTo(current + 1); }
    function prevSlide() { goTo(current - 1); }

    function startTimer() {
      stopTimer();
      timer = setInterval(nextSlide, 5000);
    }
    function stopTimer() { clearInterval(timer); }

    if (prev) prev.addEventListener('click', function () { stopTimer(); prevSlide(); startTimer(); });
    if (next) next.addEventListener('click', function () { stopTimer(); nextSlide(); startTimer(); });
    dots.addEventListener('click', function (e) {
      var dot = e.target.closest('.carousel-dot');
      if (dot) { stopTimer(); goTo(parseInt(dot.getAttribute('data-index'))); startTimer(); }
    });

    var carousel = document.getElementById('reviewsCarousel');
    if (carousel) {
      carousel.addEventListener('mouseenter', stopTimer);
      carousel.addEventListener('mouseleave', startTimer);
    }

    if (reviews.length < 2) {
      if (dots) dots.style.display = 'none';
      if (prev) prev.style.display = 'none';
      if (next) next.style.display = 'none';
    } else {
      startTimer();
    }
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
  window.SNTStore.ready(function () {
    var workers = window.SNTStore.getWorkers();
    var html = workers.map(function (w, i) {
      var accent = ACCENTS[i % ACCENTS.length];
      var phoneClean = (w.phone || '').replace(/[\s\-]/g, '');
      var tgHref = w.telegram || 'https://t.me/' + phoneClean;
      return '<div class="card-person anim" style="--accent:' + accent + ';--d:' + (560 + i * 100) + 'ms">' +
        '<span class="avatar" style="--accent:' + accent + '">' + esc(initName(w.name)) + '</span>' +
        '<span class="p-body">' +
        '<span class="p-name">' + esc(w.name) + '</span>' +
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