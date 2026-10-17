import { expect, test } from '@playwright/test';

const cases = [
  {
    name: 'real API checking respects word count and the internal UTF-8 body cap',
    async run(page, editor) {
      const limits = await page.request.get('/v1/config').then(response => response.json());
      expect(limits.maxBodyBytes).toBe(1 << 20);
      const batches = [];
      page.on('request', request => {
        if (request.method() === 'POST' && request.url().endsWith('/v1/check')) {
          const body = request.postData();
          expect(Buffer.byteLength(body)).toBeLessThanOrEqual(limits.maxBodyBytes);
          const { words } = JSON.parse(body);
          expect(words.length).toBeLessThanOrEqual(limits.maxWords);
          batches.push(words);
        }
      });
      const words = ['teh', 'speling', 'recieve', 'definately', '𐐀'.repeat(limits.maxWordLen), '𐐀'.repeat(limits.maxWordLen - 1), 'z'.repeat(limits.maxWordLen)];
      await editor.fill(words.join(' '));
      await expect(page.locator('.spelling-mark')).toHaveText(words);
      expect(batches.flat()).toEqual(words);
      expect(batches.length).toBeGreaterThan(1);
      await expect(editor).toHaveValue(words.join(' '));
    },
  },
  {
    name: 'configured word length governs checking and personal dictionary acceptance',
    async run(page, editor) {
      const limit = Number(process.env.SPELL_MAX_WORD_LEN || 80);
      const word = 'z'.repeat(limit);
      await editor.fill(`${word} ${word}z`);
      await expect(page.locator('.spelling-mark')).toHaveText([word]);
      await expect(page.getByText(`1 word was not checked: maximum ${limit} characters per word.`)).toBeVisible();
      await editor.focus();
      await editor.evaluate(element => element.setSelectionRange(1, 1));
      await editor.press('Alt+ArrowDown');
      await page.getByRole('dialog').getByRole('button', { name: 'Add to dictionary', exact: true }).click();
      await expect(page.locator('.spelling-mark')).toHaveCount(0);
      await page.reload();
      await editor.fill(word);
      await expect(page.getByRole('status')).toHaveText('All clear');
      await page.getByRole('button', { name: 'Dictionary', exact: true }).click();
      await page.getByRole('button', { name: `Remove ${word}`, exact: true }).click();
      await expect(page.locator('.spelling-mark')).toHaveText([word]);
    },
  },
  {
    name: 'saved language does not bypass loading server input limits',
    async run(page, editor) {
      await page.evaluate(() => localStorage.setItem('spellcheck.language', 'en-GB'));
      await page.route('**/v1/config', route => route.abort());
      await page.reload();
      await editor.fill('speling');
      await expect(page.getByRole('status')).toHaveText('Check unavailable');
      await expect(page.getByRole('combobox', { name: 'English convention' })).toBeDisabled();
      await expect(page.locator('.spelling-mark')).toHaveCount(0);
      await page.unroute('**/v1/config');
      await page.getByRole('button', { name: 'Retry check' }).click();
      await expect(page.getByRole('combobox', { name: 'English convention' })).toHaveValue('en-GB');
      await expect(page.locator('.spelling-mark')).toHaveText(['speling']);
    },
  },
  {
    name: 'configuration failure preserves typing and retries against the real API',
    async run(page, editor) {
      await page.route('**/v1/config', route => route.abort());
      await page.reload();
      await editor.fill('speling');
      await expect(page.getByRole('status')).toHaveText('Check unavailable');
      await expect(page.getByRole('combobox', { name: 'English convention' })).toBeDisabled();
      await page.unroute('**/v1/config');
      await page.getByRole('button', { name: 'Retry check' }).click();
      await expect(page.locator('.spelling-mark')).toHaveText(['speling']);
      await expect(editor).toHaveValue('speling');
    },
  },
  {
    name: 'configured English default initializes the editor without saving a user preference',
    async run(page, editor) {
      const expected = process.env.SPELL_DEFAULT_LANGUAGE || 'en-US';
      await expect(page.getByRole('combobox', { name: 'English convention' })).toHaveValue(expected);
      expect(await page.evaluate(() => localStorage.getItem('spellcheck.language'))).toBeNull();
      await editor.fill('color colour');
      await expect(page.locator('.spelling-mark')).toHaveText([expected === 'en-GB' ? 'color' : 'colour']);
    },
  },
  {
    name: 'an explicit saved US preference overrides the configured English default',
    async run(page, editor) {
      const selector = page.getByRole('combobox', { name: 'English convention' });
      await expect(selector).toBeEnabled();
      await selector.selectOption('en-US');
      await page.reload();
      await expect(selector).toHaveValue('en-US');
      await editor.fill('color colour');
      await expect(page.locator('.spelling-mark')).toHaveText(['colour']);
    },
  },
  {
    name: 'hover accepts a personal word in both languages and removal restores warnings',
    async run(page, editor) {
      await editor.fill('Fadhili FADHILI speling');
      await expect(page.locator('.spelling-mark')).toHaveText(['Fadhili', 'FADHILI', 'speling']);
      const mark = await page.locator('.spelling-mark').first().boundingBox();
      await page.mouse.move(mark.x + mark.width / 2, mark.y + mark.height / 2);
      await page.getByRole('dialog', { name: 'Corrections for Fadhili' })
        .getByRole('button', { name: 'Add to dictionary', exact: true }).click();
      await expect(editor).toHaveValue('Fadhili FADHILI speling');
      await expect(editor).toBeFocused();
      await expect(editor).toHaveJSProperty('selectionStart', 23);
      await expect(page.locator('.spelling-mark')).toHaveText(['speling']);
      await page.getByRole('combobox', { name: 'English convention' }).selectOption('en-GB');
      await expect(page.locator('.spelling-mark')).toHaveText(['speling']);
      await page.reload();
      await expect(editor).toHaveValue('');
      await editor.fill('Fadhili speling');
      await expect(page.locator('.spelling-mark')).toHaveText(['speling']);
      await page.getByRole('button', { name: 'Dictionary', exact: true }).click();
      await page.getByRole('button', { name: 'Remove fadhili', exact: true }).click();
      await expect(page.locator('.spelling-mark')).toHaveText(['Fadhili', 'speling']);
      await page.reload();
      await editor.fill('Fadhili');
      await expect(page.locator('.spelling-mark')).toHaveText(['Fadhili']);
    },
  },
  {
    name: 'keyboard adds a word without changing the selection',
    async run(page, editor) {
      await editor.fill('Fadhili writes');
      await expect(page.locator('.spelling-mark')).toHaveText(['Fadhili']);
      await editor.evaluate(element => element.setSelectionRange(2, 4));
      await editor.press('Alt+ArrowDown');
      await page.getByRole('dialog').getByRole('button', { name: 'Add to dictionary' }).click();
      await expect(editor).toBeFocused();
      await expect(editor).toHaveJSProperty('selectionStart', 2);
      await expect(editor).toHaveJSProperty('selectionEnd', 4);
      await expect(page.getByRole('status')).toHaveText('All clear');
    },
  },
  {
    name: 'blocked browser storage keeps personal words usable for the current session',
    async run(page, editor) {
      await page.addInitScript(() => {
        Object.defineProperty(window, 'localStorage', { get() { throw new Error('blocked'); } });
      });
      await page.reload();
      await editor.fill('Fadhili');
      await expect(page.locator('.spelling-mark')).toHaveText(['Fadhili']);
      await editor.press('Alt+ArrowDown');
      await page.getByRole('dialog').getByRole('button', { name: 'Add to dictionary' }).click();
      await expect(page.getByRole('status')).toHaveText('All clear');
      await page.getByRole('button', { name: 'Dictionary', exact: true }).click();
      await expect(page.getByText('Browser storage unavailable. Words are kept for this session only.')).toBeVisible();
      await page.getByRole('button', { name: 'Remove fadhili', exact: true }).click();
      await expect(page.locator('.spelling-mark')).toHaveText(['Fadhili']);
    },
  },
  {
    name: 'switching English convention rechecks the draft and closes stale corrections',
    async run(page, editor) {
      const language = page.getByRole('combobox', { name: 'English convention' });
      await expect(language).toHaveValue('en-US');
      await editor.fill('color colour JWT DNS TEH');
      await expect(page.locator('.spelling-mark')).toHaveText(['colour', 'TEH']);
      await language.selectOption('en-GB');
      await expect(editor).toHaveValue('color colour JWT DNS TEH');
      await expect(page.locator('.spelling-mark')).toHaveText(['color', 'TEH']);
      await editor.focus();
      await editor.evaluate(element => element.setSelectionRange(3, 3));
      await editor.press('Alt+ArrowDown');
      await expect(page.getByRole('dialog').getByRole('button', { name: 'colour', exact: true })).toBeVisible();
      await language.selectOption('en-US');
      await expect(page.getByRole('dialog')).toHaveCount(0);
      await expect(page.locator('.spelling-mark')).toHaveText(['colour', 'TEH']);
    },
  },
  {
    name: 'the language preference survives reload without storing the draft',
    async run(page, editor) {
      const language = page.getByRole('combobox', { name: 'English convention' });
      await language.selectOption('en-GB');
      await editor.fill('colour center');
      await expect(page.locator('.spelling-mark')).toHaveText(['center']);
      await page.reload();
      await expect(language).toHaveValue('en-GB');
      await expect(editor).toHaveValue('');
      await editor.fill('colour center');
      await expect(page.locator('.spelling-mark')).toHaveText(['center']);
    },
  },
  {
    name: 'typing automatically checks uppercase typos',
    async run(page, editor) {
      await editor.pressSequentially('TEH RECIEVE SPELING');
      await expect(page.getByRole('status')).toHaveText('3 spelling issues');
      await expect(page.locator('.spelling-mark')).toHaveText(['TEH', 'RECIEVE', 'SPELING']);
    },
  },
  ...[
    { name: 'early correction keeps the caret near the start', text: 'teh quick fox', offset: 3,
      word: 'teh', suggestion: 'the', corrected: 'the quick fox', caret: 3, typed: 'the! quick fox' },
    { name: 'longer correction maps a caret inside the word', text: 'speling is hard', offset: 3,
      word: 'speling', suggestion: 'spelling', corrected: 'spelling is hard', caret: 8, typed: 'spelling! is hard' },
    { name: 'repeated corrections map the caret through both replacements', text: 'speling and speling', offset: 19,
      word: 'speling', suggestion: 'spelling', corrected: 'spelling and spelling', caret: 21, typed: 'spelling and spelling!' },
  ].map(row => ({
    name: row.name,
    async run(page, editor) {
      await editor.fill(row.text);
      await expect(page.getByRole('status')).toHaveText('1 spelling issue');
      await editor.press(process.platform === 'darwin' ? 'Meta+ArrowLeft' : 'Control+Home');
      await expect(editor).toHaveJSProperty('selectionStart', 0);
      for (let index = 0; index < row.offset; index++) await editor.press('ArrowRight');
      await editor.press('Alt+ArrowDown');
      const popup = page.getByRole('dialog', { name: `Corrections for ${row.word}` });
      await popup.getByRole('button', { name: row.suggestion, exact: true }).click();
      await expect(editor).toHaveValue(row.corrected);
      await expect(editor).toBeFocused();
      await expect(editor).toHaveJSProperty('selectionStart', row.caret);
      await expect(editor).toHaveJSProperty('selectionEnd', row.caret);
      await editor.pressSequentially('!');
      await expect(editor).toHaveValue(row.typed);
    },
  })),
  {
    name: 'hover offers an inline correction without moving the caret',
    async run(page, editor) {
      await editor.fill('speling is hard');
      await expect(page.getByRole('status')).toHaveText('1 spelling issue');
      const mark = await page.locator('.spelling-mark').boundingBox();
      await page.mouse.move(mark.x + mark.width / 2, mark.y + mark.height / 2);
      await page.getByRole('dialog', { name: 'Corrections for speling' })
        .getByRole('button', { name: 'spelling', exact: true }).click();
      await expect(editor).toHaveValue('spelling is hard');
      await expect(editor).toHaveJSProperty('selectionStart', 16);
      await expect(page.getByRole('status')).toHaveText('All clear');
    },
  },
  {
    name: 'Undo restores every corrected occurrence and its marks',
    async run(page, editor) {
      await editor.fill('speling and speling');
      await expect(page.locator('.spelling-mark')).toHaveCount(2);
      await editor.press('Alt+ArrowDown');
      await page.getByRole('dialog').getByRole('button', { name: 'spelling', exact: true }).click();
      await expect(page.getByRole('status')).toHaveText('All clear');
      await page.getByRole('button', { name: 'Undo correction' }).click();
      await expect(editor).toHaveValue('speling and speling');
      await expect(page.getByRole('status')).toHaveText('1 spelling issue');
      await expect(page.locator('.spelling-mark')).toHaveCount(2);
    },
  },
  {
    name: 'Clear removes the draft, popup and stale marks',
    async run(page, editor) {
      await editor.fill('speling');
      await expect(page.locator('.spelling-mark')).toHaveCount(1);
      await editor.press('Alt+ArrowDown');
      await expect(page.getByRole('dialog')).toBeVisible();
      await page.getByRole('button', { name: 'Clear', exact: true }).click();
      await expect(editor).toHaveValue('');
      await expect(page.getByRole('status')).toHaveText('Ready to write');
      await expect(page.locator('.spelling-mark')).toHaveCount(0);
      await expect(page.getByRole('dialog')).toHaveCount(0);
    },
  },
  {
    name: 'keyboard corrections expose instructions and Escape restores focus',
    async run(page, editor) {
      await expect(editor).toHaveAccessibleDescription(/Alt\+ArrowDown/);
      await editor.fill('speling');
      await expect(page.getByRole('status')).toHaveText('1 spelling issue');
      await editor.press('Alt+ArrowDown');
      const popup = page.getByRole('dialog', { name: 'Corrections for speling' });
      await expect(popup.getByRole('button', { name: 'spelling', exact: true })).toBeFocused();
      await page.keyboard.press('ArrowDown');
      await page.keyboard.press('ArrowUp');
      await expect(popup.getByRole('button', { name: 'spelling', exact: true })).toBeFocused();
      await page.keyboard.press('Escape');
      await expect(popup).toHaveCount(0);
      await expect(editor).toBeFocused();
    },
  },
  {
    name: 'composition waits for committed input before checking',
    async run(page, editor) {
      const requests = [];
      page.on('request', request => { if (request.url().endsWith('/v1/check')) requests.push(request); });
      await editor.focus();
      await editor.evaluate(element => {
        element.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }));
        element.value = 'speling';
        element.dispatchEvent(new InputEvent('input', { bubbles: true, isComposing: true, inputType: 'insertCompositionText', data: 'speling' }));
      });
      // Allow an entire debounce interval to elapse while composition is active.
      await page.waitForTimeout(500);
      expect(requests).toHaveLength(0);
      await expect(page.locator('.spelling-mark')).toHaveCount(0);
      await editor.evaluate(element => element.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: 'speling' })));
      await expect(page.getByRole('status')).toHaveText('1 spelling issue');
      expect(requests).toHaveLength(1);
    },
  },
  {
    name: 'reduced motion keeps keyboard editing functional',
    async run(page, editor) {
      await page.emulateMedia({ reducedMotion: 'reduce' });
      await editor.fill('speling');
      await expect(page.getByRole('status')).toHaveText('1 spelling issue');
      await editor.press('Alt+ArrowDown');
      await page.keyboard.press('Enter');
      await expect(editor).toHaveValue('spelling');
      await expect(page.getByRole('status')).toHaveText('All clear');
    },
  },
  {
    name: 'failed checks preserve typing and Retry uses the real API',
    async run(page, editor) {
      await editor.fill('speling');
      await expect(page.locator('.spelling-mark')).toHaveCount(1);
      await page.route('**/v1/check', route => route.abort());
      await editor.pressSequentially(' is hard');
      await expect(page.getByRole('status')).toHaveText('Check unavailable');
      await expect(editor).toHaveValue('speling is hard');
      await expect(page.locator('.spelling-mark')).toHaveCount(0);
      await page.unroute('**/v1/check');
      await page.getByRole('button', { name: 'Retry check' }).click();
      await expect(page.getByRole('status')).toHaveText('1 spelling issue');
      await expect(page.locator('.spelling-mark')).toHaveText(['speling']);
    },
  },
];

for (const row of cases) {
  test(row.name, async ({ page }) => {
    await page.goto('/');
    await row.run(page, page.getByRole('textbox', { name: 'Your draft' }));
  });
}

test('@touch touch opens inline corrections', async ({ page }) => {
  await page.goto('/');
  const editor = page.getByRole('textbox', { name: 'Your draft' });
  await editor.fill('speling');
  await expect(page.getByRole('status')).toHaveText('1 spelling issue');
  const mark = await page.locator('.spelling-mark').boundingBox();
  await page.touchscreen.tap(mark.x + mark.width / 2, mark.y + mark.height / 2);
  await page.getByRole('dialog').getByRole('button', { name: 'spelling', exact: true }).tap();
  await expect(editor).toHaveValue('spelling');
});

test('@contrast forced colors make native text and underlines visible', async ({ page }) => {
  await page.emulateMedia({ forcedColors: 'active' });
  await page.goto('/');
  const editor = page.getByRole('textbox', { name: 'Your draft' });
  await editor.fill('speling');
  await expect(page.getByRole('status')).toHaveText('1 spelling issue');
  const colors = await editor.evaluate(element => {
    const mark = document.querySelector('.spelling-mark');
    return { text: getComputedStyle(element).color, underline: getComputedStyle(mark).textDecorationColor,
      overlay: getComputedStyle(mark).color, forced: matchMedia('(forced-colors: active)').matches };
  });
  expect(colors.forced).toBe(true);
  expect(colors.text).not.toBe('rgba(0, 0, 0, 0)');
  expect(colors.underline).toBe(colors.text);
  expect(colors.overlay).toBe('rgba(0, 0, 0, 0)');
});
