(function () {

  // ---- Theme ----

  var VALID_THEMES = ['warm', 'blue', 'milk'];
  var THEME_LABELS = {
    warm: 'Warm',
    blue: 'Blue',
    milk: 'Milk'
  };
  var updateThemePickerDisplay = function () {};

  function initTheme() {
    var pickerButton = document.getElementById('theme-picker-button');
    var pickerMenu = document.getElementById('theme-picker-menu');
    var pickerLabel = document.querySelector('[data-theme-trigger-label]');
    var pickerSwatch = document.querySelector('[data-theme-trigger-swatch]');
    var saved = localStorage.getItem('clip-pad-theme') || 'warm';

    updateThemePickerDisplay = function (theme) {
      if (pickerLabel) pickerLabel.textContent = THEME_LABELS[theme] || 'Warm';
      if (pickerSwatch) pickerSwatch.setAttribute('data-theme-swatch', theme);
    };

    applyTheme(saved, false);

    if (!pickerButton || !pickerMenu) return;

    document.querySelectorAll('[data-theme-option]').forEach(function (option, index) {
      option.addEventListener('click', function () {
        applyTheme(option.dataset.themeOption, true);
        closeThemeMenu();
        pickerButton.focus();
      });

      option.addEventListener('keydown', function (event) {
        var options = Array.prototype.slice.call(document.querySelectorAll('[data-theme-option]'));
        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
          event.preventDefault();
          var nextIndex = index + (event.key === 'ArrowDown' ? 1 : -1);
          if (nextIndex < 0) nextIndex = options.length - 1;
          if (nextIndex >= options.length) nextIndex = 0;
          options[nextIndex].focus();
        } else if (event.key === 'Escape') {
          event.preventDefault();
          closeThemeMenu();
          pickerButton.focus();
        }
      });
    });

    pickerButton.addEventListener('click', function () {
      if (pickerMenu.classList.contains('hidden')) {
        openThemeMenu();
      } else {
        closeThemeMenu();
      }
    });

    pickerButton.addEventListener('keydown', function (event) {
      if (event.key === 'ArrowDown' || event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        openThemeMenu();
      } else if (event.key === 'Escape') {
        closeThemeMenu();
      }
    });

    document.addEventListener('click', function (event) {
      if (!event.target.closest('.theme-switcher')) closeThemeMenu();
    });

    document.addEventListener('keydown', function (event) {
      if (event.key === 'Escape') closeThemeMenu();
    });

    function openThemeMenu() {
      pickerMenu.classList.remove('hidden');
      pickerButton.setAttribute('aria-expanded', 'true');
      var activeOption = document.querySelector('[data-theme-option].is-active');
      if (activeOption) activeOption.focus();
    }

    function closeThemeMenu() {
      pickerMenu.classList.add('hidden');
      pickerButton.setAttribute('aria-expanded', 'false');
    }

  }

  function applyTheme(theme, save) {
    if (VALID_THEMES.indexOf(theme) === -1) theme = 'warm';
    document.documentElement.setAttribute('data-theme', theme);
    if (save) localStorage.setItem('clip-pad-theme', theme);
    document.querySelectorAll('[data-theme-option]').forEach(function (option) {
      var isActive = option.dataset.themeOption === theme;
      option.classList.toggle('is-active', isActive);
      option.setAttribute('aria-selected', isActive ? 'true' : 'false');
    });
    updateThemePickerDisplay(theme);
  }

  function currentTheme() {
    return document.documentElement.getAttribute('data-theme') || 'warm';
  }

  initTheme();

  // ---- Local time ----

  function initLocalTimes() {
    document.querySelectorAll('time[data-utc]').forEach(function (el) {
      var d = new Date(el.dataset.utc);
      if (isNaN(d.getTime())) return;
      el.textContent = d.toLocaleString(undefined, {
        year: 'numeric', month: 'short', day: 'numeric',
        hour: '2-digit', minute: '2-digit', second: '2-digit'
      });
    });
  }

  initLocalTimes();

  // ---- Page init ----

  var page = document.querySelector('[data-page]');
  if (!page) return;

  var p = page.dataset.page;
  if (p === 'index')   initPasteForm();
  if (p === 'burn')    initBurnReveal(page.dataset.revealUrl);
  if (p === 'notepad') initNotepad();
  if (p === 'paste')   initPasteView(page.dataset.pasteTheme);

  // ---- Paste form ----

  function initPasteForm() {
    var form       = document.getElementById('paste-form');
    var feedback   = document.getElementById('paste-feedback');
    var result     = document.getElementById('paste-result');
    var urlInput   = document.getElementById('paste-url');
    var copyBtn    = document.getElementById('copy-url-btn');
    var qrBtn      = document.getElementById('qr-url-btn');
    var textarea   = document.getElementById('paste-content');
    var charCount  = document.getElementById('paste-charcount');

    if (textarea && charCount) {
      textarea.addEventListener('input', function () {
        charCount.textContent = textarea.value.length.toLocaleString() + ' / 1,048,576';
      });
    }

    form.addEventListener('submit', async function (event) {
      event.preventDefault();
      setFeedback('Creating paste\u2026', '');
      result.classList.add('hidden');

      var payload = {
        content: textarea ? textarea.value : '',
        expire: document.getElementById('paste-expire').value,
        theme: currentTheme()
      };

      try {
        var response = await fetch('/api/pastes', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });
        var data = await response.json();
        if (!response.ok) {
          setFeedback(data.error || 'Unable to create paste.', 'is-error');
          return;
        }
        urlInput.value = window.location.origin + data.url;
        result.classList.remove('hidden');
        setFeedback('Paste created.', 'is-success');
        urlInput.select();
      } catch (_) {
        setFeedback('Unable to create paste right now.', 'is-error');
      }
    });

    if (copyBtn) {
      copyBtn.addEventListener('click', function () {
        copyText(urlInput.value, copyBtn, 'Copied!');
      });
    }

    if (qrBtn) {
      qrBtn.addEventListener('click', function () {
        showQRModal(urlInput.value);
      });
    }

    initQRModal();

    function setFeedback(msg, cls) {
      feedback.textContent = msg;
      feedback.className = 'feedback' + (cls ? ' ' + cls : '');
    }
  }

  // ---- Burn reveal ----

  function initBurnReveal(revealUrl) {
    var button       = document.getElementById('reveal-button');
    var feedback     = document.getElementById('reveal-feedback');
    var result       = document.getElementById('reveal-result');
    var content      = document.getElementById('reveal-content');
    var copyRevealBtn = document.getElementById('copy-reveal-btn');

    button.addEventListener('click', async function () {
      button.disabled = true;
      setFeedback('Revealing paste\u2026', '');

      try {
        var response = await fetch(revealUrl, { method: 'POST' });
        var data = await response.json();
        if (!response.ok) {
          if (response.status === 404) {
            window.location.reload();
            return;
          }
          setFeedback(data.error || 'Unable to reveal paste.', 'is-error');
          button.disabled = false;
          return;
        }
        content.textContent = data.content;
        result.classList.remove('hidden');
        button.classList.add('hidden');
        setFeedback('Paste revealed and permanently deleted.', 'is-success');

        if (copyRevealBtn) {
          copyRevealBtn.addEventListener('click', function () {
            copyText(data.content, copyRevealBtn, 'Copied!');
          });
        }
      } catch (_) {
        setFeedback('Unable to reveal paste right now.', 'is-error');
        button.disabled = false;
      }
    });

    function setFeedback(msg, cls) {
      feedback.textContent = msg;
      feedback.className = 'feedback' + (cls ? ' ' + cls : '');
    }
  }

  // ---- Paste view ----

  function initPasteView(pasteTheme) {
    // Apply the creator's stored theme for this page without overwriting the
    // visitor's own preference in localStorage.
    if (pasteTheme && VALID_THEMES.indexOf(pasteTheme) !== -1) {
      applyTheme(pasteTheme, false);
    }

    var copyBtn = document.getElementById('copy-content-btn');
    var pre     = document.getElementById('paste-content-text');
    if (copyBtn && pre) {
      copyBtn.addEventListener('click', function () {
        copyText(pre.textContent, copyBtn, 'Copied!');
      });
    }
  }

  // ---- Notepad ----

  function initNotepad() {
    var textarea   = document.getElementById('notepad-text');
    var characters = document.getElementById('stats-characters');
    var words      = document.getElementById('stats-words');
    var lines      = document.getElementById('stats-lines');
    var maxBtn     = document.getElementById('notepad-max-btn');
    var maxLabel   = document.getElementById('notepad-max-label');
    var maxIcon    = document.getElementById('notepad-max-icon');
    var restoreIcon = document.getElementById('notepad-restore-icon');

    function updateStats() {
      var value = textarea.value;
      characters.textContent = value.length.toLocaleString();
      words.textContent = value.trim() === '' ? '0' : value.trim().split(/\s+/).length.toLocaleString();
      lines.textContent = value === '' ? '0' : value.split(/\n/).length.toLocaleString();
    }

    function setMaximized(isMax) {
      document.documentElement.classList.toggle('notepad-max', isMax);
      maxLabel.textContent = isMax ? 'Restore' : 'Maximize';
      maxIcon.classList.toggle('hidden', isMax);
      restoreIcon.classList.toggle('hidden', !isMax);
    }

    // Sync state from URL hash on load and on back/forward navigation.
    function syncFromHash() {
      setMaximized(location.hash === '#max');
    }

    function updateHashState(isMax) {
      var url = location.pathname + location.search + (isMax ? '#max' : '');
      history.pushState({ notepadMax: isMax }, '', url);
      syncFromHash();
    }

    syncFromHash();
    window.addEventListener('hashchange', syncFromHash);
    window.addEventListener('popstate', syncFromHash);

    maxBtn.addEventListener('click', function () {
      updateHashState(!document.documentElement.classList.contains('notepad-max'));
    });

    textarea.addEventListener('input', updateStats);
    textarea.addEventListener('paste', function (event) {
      event.preventDefault();
      var text  = event.clipboardData.getData('text/plain');
      var start = textarea.selectionStart;
      var end   = textarea.selectionEnd;
      var value = textarea.value;
      textarea.value = value.slice(0, start) + text + value.slice(end);
      var cursor = start + text.length;
      textarea.selectionStart = cursor;
      textarea.selectionEnd   = cursor;
      textarea.dispatchEvent(new Event('input'));
    });

    updateStats();
  }

  // ---- QR Code ----

  function initQRModal() {
    var modal    = document.getElementById('qr-modal');
    var closeBtn = document.getElementById('qr-modal-close');
    var backdrop = modal && modal.querySelector('.qr-modal-backdrop');
    if (!modal) return;

    function closeModal() {
      modal.classList.add('hidden');
      var canvas = document.getElementById('qr-canvas');
      if (canvas) canvas.innerHTML = '';
    }

    if (closeBtn) closeBtn.addEventListener('click', closeModal);
    if (backdrop) backdrop.addEventListener('click', closeModal);

    document.addEventListener('keydown', function (event) {
      if (event.key === 'Escape' && !modal.classList.contains('hidden')) {
        closeModal();
      }
    });
  }

  function showQRModal(url) {
    var modal  = document.getElementById('qr-modal');
    var canvas = document.getElementById('qr-canvas');
    if (!modal || !canvas) return;
    canvas.innerHTML = '';
    new QRCode(canvas, {
      text: url,
      width: 200,
      height: 200,
      colorDark: '#2f2f2f',
      colorLight: '#ffffff',
      correctLevel: QRCode.CorrectLevel.M
    });
    modal.classList.remove('hidden');
    var closeBtn = document.getElementById('qr-modal-close');
    if (closeBtn) closeBtn.focus();
  }

  // ---- Copy helper ----

  function copyText(text, btn, successLabel) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text)
        .then(function () { flashButton(btn, successLabel); })
        .catch(function () { legacyCopy(text); flashButton(btn, successLabel); });
    } else {
      legacyCopy(text);
      flashButton(btn, successLabel);
    }
  }

  function legacyCopy(text) {
    var el = document.createElement('textarea');
    el.value = text;
    el.style.cssText = 'position:fixed;opacity:0;pointer-events:none';
    document.body.appendChild(el);
    el.focus();
    el.select();
    try { document.execCommand('copy'); } catch (_) {}
    document.body.removeChild(el);
  }

  function flashButton(btn, label) {
    var original = btn.textContent;
    btn.textContent = label;
    btn.disabled = true;
    setTimeout(function () {
      btn.textContent = original;
      btn.disabled = false;
    }, 1800);
  }
})();
