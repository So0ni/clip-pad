(function () {
  var page = document.querySelector('[data-page]');
  if (!page) return;

  var p = page.dataset.page;
  if (p === 'index')   initPasteForm();
  if (p === 'burn')    initBurnReveal(page.dataset.revealUrl);
  if (p === 'notepad') initNotepad();
  if (p === 'paste')   initPasteView();

  // ---- Paste form ----

  function initPasteForm() {
    var form       = document.getElementById('paste-form');
    var feedback   = document.getElementById('paste-feedback');
    var result     = document.getElementById('paste-result');
    var urlInput   = document.getElementById('paste-url');
    var copyBtn    = document.getElementById('copy-url-btn');
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
        expire: document.getElementById('paste-expire').value
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

  function initPasteView() {
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

    function updateStats() {
      var value = textarea.value;
      characters.textContent = value.length.toLocaleString();
      words.textContent = value.trim() === '' ? '0' : value.trim().split(/\s+/).length.toLocaleString();
      lines.textContent = value === '' ? '0' : value.split(/\n/).length.toLocaleString();
    }

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
