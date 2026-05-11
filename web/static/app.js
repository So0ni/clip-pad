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
    var textarea    = document.getElementById('notepad-text');
    var characters  = document.getElementById('stats-characters');
    var words       = document.getElementById('stats-words');
    var lines       = document.getElementById('stats-lines');
    var status      = document.getElementById('notepad-save-status');
    var listPanel   = document.getElementById('notepad-list-panel');
    var listEl      = document.getElementById('notepad-list');
    var listBtn     = document.getElementById('notepad-list-btn');
    var newBtn      = document.getElementById('notepad-new-btn');
    var maxBtn      = document.getElementById('notepad-max-btn');
    var maxLabel    = document.getElementById('notepad-max-label');
    var maxIcon     = document.getElementById('notepad-max-icon');
    var restoreIcon = document.getElementById('notepad-restore-icon');
    var db;
    var activeNote;
    var saveTimer;
    var applyingNote = false;
    var dbName = 'clip-pad-notepad';
    var dbVersion = 1;

    function updateStats() {
      var value = textarea.value;
      var charCount = value.length;
      var wordCount = value.trim() === '' ? 0 : value.trim().split(/\s+/).length;
      var lineCount = value === '' ? 0 : value.split(/\n/).length;
      characters.textContent = formatCount(charCount, 'char');
      words.textContent = formatCount(wordCount, 'word');
      lines.textContent = formatCount(lineCount, 'line');
    }

    function formatCount(value, label) {
      return value.toLocaleString() + ' ' + label + (value === 1 ? '' : 's');
    }

    function setStatus(text) {
      if (status) status.textContent = text;
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

    textarea.addEventListener('input', function () {
      updateStats();
      queueSave();
    });
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

    if (listBtn && listPanel) {
      listBtn.addEventListener('click', function () {
        toggleListPanel(listPanel.classList.contains('hidden'));
      });

      document.addEventListener('click', function (event) {
        if (listPanel.classList.contains('hidden')) return;
        if (event.target.closest('.notepad-list-panel') || event.target.closest('#notepad-list-btn')) return;
        toggleListPanel(false);
      });

      document.addEventListener('keydown', function (event) {
        if (event.key === 'Escape') toggleListPanel(false);
      });
    }

    if (newBtn) {
      newBtn.addEventListener('click', async function () {
        if (!db) return;
        await flushSave();
        if (activeNote && textarea.value.trim() === '') {
          textarea.focus();
          toggleListPanel(false);
          setStatus('Ready');
          return;
        }
        var note = await createNote('');
        await openNote(note.id);
        toggleListPanel(false);
        textarea.focus();
      });
    }

    updateStats();
    bootNotes();

    async function bootNotes() {
      if (!window.indexedDB) {
        setStatus('Autosave unavailable');
        textarea.focus();
        return;
      }

      try {
        db = await openDatabase();
        var lastID = await getSetting('lastOpenedNoteId');
        var note = lastID ? await getNote(lastID) : null;
        if (!note) {
          var notes = await getAllNotes();
          note = notes[0] || await createNote('');
        }
        await openNote(note.id);
        textarea.focus();
      } catch (_) {
        setStatus('Autosave unavailable');
        textarea.focus();
      }
    }

    function toggleListPanel(show) {
      if (!listPanel || !listBtn) return;
      listPanel.classList.toggle('hidden', !show);
      listBtn.setAttribute('aria-expanded', show ? 'true' : 'false');
      if (show) renderNotesList();
    }

    function queueSave() {
      if (!db || !activeNote || applyingNote) return;
      clearTimeout(saveTimer);
      setStatus('Saving...');
      saveTimer = setTimeout(function () {
        saveActiveNote();
      }, 350);
    }

    async function flushSave() {
      clearTimeout(saveTimer);
      await saveActiveNote();
    }

    async function saveActiveNote() {
      if (!db || !activeNote || applyingNote) return;
      var now = new Date().toISOString();
      activeNote.content = textarea.value;
      activeNote.title = titleFromContent(activeNote.content);
      activeNote.updatedAt = now;
      try {
        await putNote(activeNote);
        setStatus('Saved');
        renderNotesList();
      } catch (_) {
        setStatus('Unable to save');
      }
    }

    async function openNote(id) {
      if (!db) return;
      await flushSave();
      var note = await getNote(id);
      if (!note) return;
      applyingNote = true;
      activeNote = note;
      textarea.value = note.content || '';
      updateStats();
      applyingNote = false;
      await setSetting('lastOpenedNoteId', note.id);
      setStatus('Saved');
      renderNotesList();
    }

    async function createNote(content) {
      var now = new Date().toISOString();
      var note = {
        id: createNoteID(),
        content: content,
        title: titleFromContent(content),
        createdAt: now,
        updatedAt: now
      };
      await putNote(note);
      return note;
    }

    async function removeNote(id) {
      if (!db) return;
      var notes = await getAllNotes();
      var deletingLastNote = notes.length <= 1;
      await deleteNote(id);
      if (activeNote && activeNote.id === id) {
        activeNote = null;
        var next = deletingLastNote ? await createNote('') : (await getAllNotes())[0];
        await openNote(next.id);
      } else {
        renderNotesList();
      }
    }

    async function renderNotesList() {
      if (!listEl || !db) return;
      var notes = await getAllNotes();
      listEl.replaceChildren();

      notes.forEach(function (note) {
        var row = document.createElement('div');
        row.className = 'notepad-list-item' + (activeNote && activeNote.id === note.id ? ' is-active' : '');

        var openBtn = document.createElement('button');
        openBtn.type = 'button';
        openBtn.className = 'notepad-list-open';

        var title = document.createElement('strong');
        title.textContent = note.title || 'Untitled';

        var meta = document.createElement('span');
        meta.textContent = formatNoteTime(note.updatedAt);

        openBtn.appendChild(title);
        openBtn.appendChild(meta);
        openBtn.addEventListener('click', async function () {
          await openNote(note.id);
          toggleListPanel(false);
          textarea.focus();
        });

        var deleteBtn = document.createElement('button');
        deleteBtn.type = 'button';
        deleteBtn.className = 'notepad-list-delete';
        deleteBtn.textContent = 'Delete';
        deleteBtn.addEventListener('click', async function () {
          if (window.confirm('Delete this note from this browser?')) {
            await removeNote(note.id);
          }
        });

        row.appendChild(openBtn);
        row.appendChild(deleteBtn);
        listEl.appendChild(row);
      });
    }

    function openDatabase() {
      return new Promise(function (resolve, reject) {
        var request = indexedDB.open(dbName, dbVersion);
        request.onupgradeneeded = function () {
          var database = request.result;
          if (!database.objectStoreNames.contains('notes')) {
            var notes = database.createObjectStore('notes', { keyPath: 'id' });
            notes.createIndex('updatedAt', 'updatedAt');
          }
          if (!database.objectStoreNames.contains('settings')) {
            database.createObjectStore('settings', { keyPath: 'key' });
          }
        };
        request.onsuccess = function () { resolve(request.result); };
        request.onerror = function () { reject(request.error); };
      });
    }

    function txComplete(tx) {
      return new Promise(function (resolve, reject) {
        tx.oncomplete = function () { resolve(); };
        tx.onerror = function () { reject(tx.error); };
        tx.onabort = function () { reject(tx.error); };
      });
    }

    function requestResult(request) {
      return new Promise(function (resolve, reject) {
        request.onsuccess = function () { resolve(request.result); };
        request.onerror = function () { reject(request.error); };
      });
    }

    async function getAllNotes() {
      var tx = db.transaction('notes', 'readonly');
      var notes = await requestResult(tx.objectStore('notes').getAll());
      notes.sort(function (a, b) {
        return String(b.updatedAt).localeCompare(String(a.updatedAt));
      });
      return notes;
    }

    function getNote(id) {
      var tx = db.transaction('notes', 'readonly');
      return requestResult(tx.objectStore('notes').get(id));
    }

    async function putNote(note) {
      var tx = db.transaction('notes', 'readwrite');
      tx.objectStore('notes').put(note);
      await txComplete(tx);
    }

    async function deleteNote(id) {
      var tx = db.transaction('notes', 'readwrite');
      tx.objectStore('notes').delete(id);
      await txComplete(tx);
    }

    async function getSetting(key) {
      var tx = db.transaction('settings', 'readonly');
      var row = await requestResult(tx.objectStore('settings').get(key));
      return row ? row.value : null;
    }

    async function setSetting(key, value) {
      var tx = db.transaction('settings', 'readwrite');
      tx.objectStore('settings').put({ key: key, value: value });
      await txComplete(tx);
    }

    function createNoteID() {
      return 'note-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 9);
    }

    function titleFromContent(content) {
      var firstLine = String(content || '').split(/\n/).map(function (line) {
        return line.trim();
      }).filter(Boolean)[0];
      if (!firstLine) return 'Untitled';
      return firstLine.length > 52 ? firstLine.slice(0, 49) + '...' : firstLine;
    }

    function formatNoteTime(value) {
      var time = new Date(value);
      if (isNaN(time.getTime())) return 'Saved';
      var diff = Date.now() - time.getTime();
      if (diff < 60000) return 'Just now';
      if (diff < 3600000) return Math.floor(diff / 60000) + ' min ago';
      return time.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
    }
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
