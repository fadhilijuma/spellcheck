<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import WritingEditor from './components/WritingEditor.vue';
import { createSession } from './session.js';
import { replaceWord } from './tokenize.js';
import { personalWord, parsePersonalWords } from './dictionary.js';
import { getConfiguration } from './api.js';

const text = ref('');
const language = ref('');
const configuration = ref(null);
const personalWords = ref([]);
const dictionaryOpen = ref(false);
const storageAvailable = ref(true);
try {
  const savedLanguage = localStorage.getItem('spellcheck.language');
  if (['en-US', 'en-GB'].includes(savedLanguage)) language.value = savedLanguage;
  personalWords.value = parsePersonalWords(localStorage.getItem('spellcheck.dictionary'));
} catch {
  storageAvailable.value = false;
}
let composing = false;
let configurationRequest;
const previousText = ref(null);
const state = ref({ phase: 'empty', issues: [], skipped: 0 });
const session = createSession(result => { state.value = result; });
const acceptedWords = computed(() => new Set(personalWords.value));
const issues = computed(() => state.value.issues.filter(issue => !acceptedWords.value.has(personalWord(issue.word))));
const wordCount = computed(() => text.value.match(/[\p{L}\p{N}][\p{L}\p{M}\p{N}'’]*/gu)?.length || 0);
const status = computed(() => {
  if (state.value.phase === 'checking') return 'Checking spelling';
  if (state.value.phase === 'offline') return 'Check unavailable';
  if (state.value.phase === 'empty') return 'Ready to write';
  if (issues.value.length) return `${issues.value.length} spelling ${issues.value.length === 1 ? 'issue' : 'issues'}`;
  return state.value.skipped ? 'Check complete' : 'All clear';
});

function recheck() {
  if (!configuration.value) return;
  session.update(text.value, { ...configuration.value, composing, language: language.value });
}

async function loadConfiguration() {
  configurationRequest?.abort();
  const request = new AbortController();
  configurationRequest = request;
  const deadline = setTimeout(() => request.abort(), 10000);
  state.value = { phase: 'checking', issues: [], skipped: 0 };
  try {
    const data = await getConfiguration({ signal: request.signal });
    if (configurationRequest !== request) return;
    configuration.value = data;
    if (!language.value) language.value = data.defaultLanguage;
    recheck();
  } catch {
    if (configurationRequest === request) state.value = { phase: 'offline', issues: [], skipped: 0 };
  } finally {
    clearTimeout(deadline);
    if (configurationRequest === request) configurationRequest = null;
  }
}

function saveLanguage() {
  try {
    localStorage.setItem('spellcheck.language', language.value);
  } catch {
    // Preferences are optional; checking does not depend on storage.
  }
}

watch(language, recheck);

watch(personalWords, words => {
  try {
    localStorage.setItem('spellcheck.dictionary', JSON.stringify(words));
    storageAvailable.value = true;
  } catch {
    storageAvailable.value = false;
  }
});

function acceptWord(word) {
  if (!issues.value.some(issue => issue.word === word)) return;
  const value = personalWord(word, configuration.value.maxWordLen);
  if (value && !acceptedWords.value.has(value)) personalWords.value = [...personalWords.value, value];
}

function removeWord(word) {
  personalWords.value = personalWords.value.filter(value => value !== word);
}

function edit(value, isComposing = false) {
  text.value = value;
  composing = isComposing;
  previousText.value = null;
  recheck();
}

function correct(word, suggestion) {
  const issue = state.value.issues.find(item => item.word === word);
  if (!issue?.suggestions.includes(suggestion)) return;
  previousText.value = text.value;
  text.value = replaceWord(text.value, word, suggestion);
  recheck();
}

function undo() {
  if (previousText.value === null) return;
  const value = previousText.value;
  edit(value);
}

function example() {
  edit([
    'Good writing starts with a simple idea.',
    'Give yourself room to think, then find the words that bring your story to life. A little attention to speling can make a big difference.',
    'I hope you recieve this message and enjoy the journey.',
  ].join('\n\n'));
}

onMounted(() => {
  loadConfiguration();
});

onBeforeUnmount(() => {
  configurationRequest?.abort();
  configurationRequest = null;
  session.dispose();
});
</script>

<template>
  <v-app>
    <v-main>
      <header class="app-header">
        <div class="header-inner">
          <div class="brand">
            <svg class="brand-mark" viewBox="0 0 40 40" aria-hidden="true"><rect width="40" height="40" rx="12"/><path d="M12 20l6 6 11-13"/></svg>
            <span>Spellcheck</span>
          </div>
          <div class="header-meta">
            <label for="english-language" class="sr-only">English convention</label>
            <select id="english-language" v-model="language" class="language-select" :disabled="!configuration" @change="saveLanguage">
              <option v-if="!language" value="" disabled>English</option>
              <option value="en-US">English (US)</option>
              <option value="en-GB">English (UK)</option>
            </select>
            <span class="header-divider"></span>
            <span class="live-dot"></span>
            <span>Live checking</span>
          </div>
        </div>
      </header>

      <main class="workspace">
          <section class="draft-panel" aria-label="Writing workspace">
            <div class="draft-toolbar">
              <div class="draft-title"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 3h7l4 4v14H7zM14 3v5h4M10 12h5M10 16h5"/></svg><h2>Your draft</h2></div>
              <div class="draft-actions">
                <v-btn size="small" variant="text" :aria-expanded="dictionaryOpen" aria-controls="personal-dictionary" @click="dictionaryOpen = !dictionaryOpen">Dictionary</v-btn>
                <v-btn v-if="previousText !== null" size="small" variant="text" @click="undo">Undo correction</v-btn>
                <v-btn size="small" variant="text" @click="example">Try an example</v-btn>
                <v-btn size="small" variant="text" :disabled="!text" @click="edit('')">Clear</v-btn>
              </div>
            </div>
            <WritingEditor :text="text" :issues="issues" :language="language" @edit="edit" @correct="correct" @accept="acceptWord" />
            <div class="draft-footer">
              <span>
                {{ wordCount }} {{ wordCount === 1 ? 'word' : 'words' }}<span class="stat-separator">·</span>{{ [...text].length }} characters
              </span>
              <span class="check-status" :class="state.phase" role="status" aria-live="polite">
                <v-progress-circular v-if="state.phase === 'checking'" indeterminate :size="12" :width="2"/>
                <span v-else class="status-dot"></span>
                {{ status }}
              </span>
            </div>
            <section v-if="dictionaryOpen" id="personal-dictionary" class="personal-dictionary" aria-label="Personal dictionary">
              <p v-if="!storageAvailable">Browser storage unavailable. Words are kept for this session only.</p>
              <p v-if="!personalWords.length">No saved words.</p>
              <ul v-else>
                <li v-for="word in personalWords" :key="word">
                  <span>{{ word }}</span>
                  <v-btn size="small" variant="text" :aria-label="'Remove ' + word" @click="removeWord(word)">Remove</v-btn>
                </li>
              </ul>
            </section>
          </section>

          <v-btn v-if="state.phase === 'offline'" size="small" variant="text" @click="configuration ? recheck() : loadConfiguration()">Retry check</v-btn>
          <p v-if="state.skipped" class="skipped-notice" role="status">
            {{ state.skipped }} {{ state.skipped === 1 ? 'word was' : 'words were' }} not checked: maximum {{ configuration.maxWordLen }} characters per word.
          </p>

      </main>
    </v-main>
  </v-app>
</template>
