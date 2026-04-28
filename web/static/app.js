(function () {
  const page = document.querySelector('[data-page]');
  if (!page) {
    return;
  }

  if (page.dataset.page === 'index') {
    initPasteForm();
  }

  if (page.dataset.page === 'burn') {
    initBurnReveal(page.dataset.revealUrl);
  }

  if (page.dataset.page === 'notepad') {
    initNotepad();
  }

  function initPasteForm() {
    const form = document.getElementById('paste-form');
    const feedback = document.getElementById('paste-feedback');
    const result = document.getElementById('paste-result');
    const urlInput = document.getElementById('paste-url');

    form.addEventListener('submit', async function (event) {
      event.preventDefault();
      setFeedback('Creating paste...', false);
      result.classList.add('hidden');

      const payload = {
        content: document.getElementById('paste-content').value,
        expire: document.getElementById('paste-expire').value
      };

      try {
        const response = await fetch('/api/pastes', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });
        const data = await response.json();
        if (!response.ok) {
          setFeedback(data.error || 'Unable to create paste.', true);
          return;
        }
        urlInput.value = window.location.origin + data.url;
        result.classList.remove('hidden');
        setFeedback('Paste created successfully.', false);
      } catch (error) {
        setFeedback('Unable to create paste right now.', true);
      }
    });

    function setFeedback(message, isError) {
      feedback.textContent = message;
      feedback.classList.toggle('is-error', Boolean(isError));
    }
  }

  function initBurnReveal(revealUrl) {
    const button = document.getElementById('reveal-button');
    const feedback = document.getElementById('reveal-feedback');
    const result = document.getElementById('reveal-result');
    const content = document.getElementById('reveal-content');

    button.addEventListener('click', async function () {
      button.disabled = true;
      feedback.textContent = 'Revealing paste...';
      feedback.classList.remove('is-error');

      try {
        const response = await fetch(revealUrl, { method: 'POST' });
        const data = await response.json();
        if (!response.ok) {
          if (response.status === 404) {
            window.location.reload();
            return;
          }
          feedback.textContent = data.error || 'Unable to reveal paste.';
          feedback.classList.add('is-error');
          button.disabled = false;
          return;
        }
        content.textContent = data.content;
        result.classList.remove('hidden');
        button.classList.add('hidden');
        feedback.textContent = 'Paste revealed.';
      } catch (error) {
        feedback.textContent = 'Unable to reveal paste right now.';
        feedback.classList.add('is-error');
        button.disabled = false;
      }
    });
  }

  function initNotepad() {
    const textarea = document.getElementById('notepad-text');
    const characters = document.getElementById('stats-characters');
    const words = document.getElementById('stats-words');
    const lines = document.getElementById('stats-lines');

    function updateStats() {
      const value = textarea.value;
      characters.textContent = String(value.length);
      words.textContent = value.trim() === '' ? '0' : String(value.trim().split(/\s+/).length);
      lines.textContent = value === '' ? '0' : String(value.split(/\n/).length);
    }

    textarea.addEventListener('input', updateStats);
    textarea.addEventListener('paste', function (event) {
      event.preventDefault();
      const text = event.clipboardData.getData('text/plain');
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const value = textarea.value;
      textarea.value = value.slice(0, start) + text + value.slice(end);
      const cursor = start + text.length;
      textarea.selectionStart = cursor;
      textarea.selectionEnd = cursor;
      textarea.dispatchEvent(new Event('input'));
    });

    updateStats();
  }
})();
