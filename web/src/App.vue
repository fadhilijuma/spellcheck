<script setup>
import { computed, onBeforeUnmount, ref } from 'vue';
import WritingEditor from './components/WritingEditor.vue';
import { createSession } from './session.js';
import { replaceWord } from './tokenize.js';

const text = ref('');
const previousText = ref(null);
const state = ref({ phase: 'empty', issues: [], skipped: 0 });
const session = createSession(result => { state.value = result; });
const wordCount = computed(() => text.value.match(/[\p{L}\p{N}][\p{L}\p{M}\p{N}'’]*/gu)?.length || 0);
const status = computed(() => {
  if (state.value.phase === 'checking') return 'Checking spelling';
  if (state.value.phase === 'offline') return 'Check unavailable';
  if (state.value.phase === 'empty') return 'Ready to write';
  if (state.value.issues.length) return `${state.value.issues.length} spelling ${state.value.issues.length === 1 ? 'issue' : 'issues'}`;
  return state.value.skipped ? 'Check complete' : 'All clear';
});

function edit(value, composing = false) {
  text.value = value;
  previousText.value = null;
  session.update(value, composing);
}

function correct(word, suggestion) {
  const issue = state.value.issues.find(item => item.word === word);
  if (!issue?.suggestions.includes(suggestion)) return;
  previousText.value = text.value;
  text.value = replaceWord(text.value, word, suggestion);
  session.update(text.value);
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

onBeforeUnmount(() => session.dispose());
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
            <span class="language-label">EN</span>
            <span>English</span>
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
                <v-btn v-if="previousText !== null" size="small" variant="text" @click="undo">Undo correction</v-btn>
                <v-btn size="small" variant="text" @click="example">Try an example</v-btn>
                <v-btn size="small" variant="text" :disabled="!text" @click="edit('')">Clear</v-btn>
              </div>
            </div>
            <WritingEditor :text="text" :issues="state.issues" @edit="edit" @correct="correct" />
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
          </section>

          <v-btn v-if="state.phase === 'offline'" size="small" variant="text" @click="session.update(text)">Retry check</v-btn>
          <p v-if="state.skipped" class="skipped-notice" role="status">
            {{ state.skipped }} {{ state.skipped === 1 ? 'word was' : 'words were' }} not checked: maximum 64 characters per word.
          </p>

      </main>
    </v-main>
  </v-app>
</template>
