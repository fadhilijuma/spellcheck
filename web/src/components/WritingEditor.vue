<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { highlightParts, tokenize, selectionAfterReplacement } from '../tokenize.js';
import { hitWord, popupPosition } from '../hover.js';

const props = defineProps({ text: { type: String, required: true }, issues: { type: Array, required: true } });
const emit = defineEmits(['edit', 'correct']);
const highlight = ref(null);
const input = ref(null);
const popup = ref(null);
const active = ref(null);
const position = ref({ left: 0, top: 0 });
const parts = computed(() => highlightParts(props.text, props.issues));
let closeTimer;

function keepOpen() {
  clearTimeout(closeTimer);
}
function close(focus = false) {
  keepOpen();
  active.value = null;
  if (focus) input.value?.focus();
}
function leave() {
  keepOpen();
  closeTimer = setTimeout(() => close(), 180);
}
function show(word, rect) {
  keepOpen();
  const issue = props.issues.find(item => item.word === word);
  if (!issue) return;
  active.value = issue;
  position.value = popupPosition(rect, { width: window.innerWidth, height: window.innerHeight });
}
function pointer(event) {
  const words = [...highlight.value.querySelectorAll('.spelling-mark')].map(element => ({
    word: element.dataset.word,
    rects: [...element.getClientRects()],
  }));
  const hit = hitWord(event.clientX, event.clientY, words);
  if (!hit) {
    leave();
    return;
  }
  const rect = hit.rects.find(rect => event.clientY >= rect.top && event.clientY <= rect.bottom);
  show(hit.word, rect);
}
function hover(event) {
  if (event.pointerType === 'mouse' && !event.buttons) pointer(event);
}
function tap(event) {
  if (event.pointerType !== 'mouse') pointer(event);
}
function syncScroll(event) {
  close();
  highlight.value.scrollTop = event.target.scrollTop;
  highlight.value.scrollLeft = event.target.scrollLeft;
}
async function keyboard(event) {
  if (event.key === 'Escape') {
    close();
    return;
  }
  if (!event.altKey || event.key !== 'ArrowDown') return;
  event.preventDefault();
  const caret = input.value.selectionStart;
  const token = tokenize(props.text).find(token => caret >= token.from && caret <= token.to);
  if (!token) return;
  const mark = [...highlight.value.querySelectorAll('.spelling-mark')]
    .find(element => element.dataset.word === token.word && Number(element.dataset.from) === token.from);
  if (!mark) return;
  show(token.word, mark.getBoundingClientRect());
  await nextTick();
  (popup.value.querySelector('button') || popup.value).focus();
}
function popupKey(event) {
  if (event.key === 'Escape') {
    event.preventDefault();
    close(true);
  } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault();
    const buttons = [...popup.value.querySelectorAll('button')];
    const current = buttons.indexOf(document.activeElement);
    const step = event.key === 'ArrowDown' ? 1 : -1;
    buttons[(current + step + buttons.length) % buttons.length]?.focus();
  }
}
async function correct(suggestion) {
  const field = input.value;
  const selection = selectionAfterReplacement(props.text, active.value.word, suggestion, {
    start: field.selectionStart,
    end: field.selectionEnd,
  });
  const direction = field.selectionDirection;
  const top = field.scrollTop;
  const left = field.scrollLeft;
  emit('correct', active.value.word, suggestion);
  close();
  await nextTick();
  field.focus({ preventScroll: true });
  field.setSelectionRange(selection.start, selection.end, direction);
  field.scrollTop = highlight.value.scrollTop = top;
  field.scrollLeft = highlight.value.scrollLeft = left;
}
function edit(event, composing = event.isComposing) {
  close();
  emit('edit', event.target.value, composing);
}
watch(() => [props.text, props.issues], () => close());
function outside(event) {
  if (event.target !== input.value && !popup.value?.contains(event.target)) close();
}
function pageScroll(event) {
  if (!popup.value?.contains(event.target)) close();
}
function resize() {
  close();
}
onMounted(() => {
  document.addEventListener('pointerdown', outside);
  window.addEventListener('scroll', pageScroll, true);
  window.addEventListener('resize', resize);
});
onBeforeUnmount(() => {
  keepOpen();
  document.removeEventListener('pointerdown', outside);
  window.removeEventListener('scroll', pageScroll, true);
  window.removeEventListener('resize', resize);
});
</script>

<template>
  <div class="editor-surface">
    <!-- No whitespace between the tags below: it would be rendered and shift the marks. -->
    <div ref="highlight" class="highlight-layer editor-text" aria-hidden="true"><span
      v-for="(part, index) in parts"
      :key="index"
      :class="{ 'spelling-mark': part.error }"
      :data-word="part.error ? part.word : undefined"
      :data-from="part.error ? part.from : undefined"
    >{{ part.text }}</span>{{ '\n' }}</div>
    <textarea
      ref="input"
      id="draft"
      class="editor-input editor-text"
      aria-label="Your draft"
      aria-describedby="editor-help"
      :value="text"
      spellcheck="false"
      autocomplete="off"
      placeholder="Start writing here…"
      @input="edit"
      @compositionend="edit($event, false)"
      @scroll="syncScroll"
      @pointermove="hover"
      @pointerdown="tap"
      @pointerleave="leave"
      @keydown="keyboard"
    ></textarea>
    <span id="editor-help" class="sr-only">Misspelled words are underlined. Put the caret in a word and press Alt+ArrowDown for corrections. Use the arrow keys to choose and Escape to return to writing.</span>
    <div
      v-if="active"
      ref="popup"
      class="correction-popup"
      role="dialog"
      :aria-label="'Corrections for ' + active.word"
      tabindex="-1"
      :style="{ left: position.left + 'px', top: position.top + 'px' }"
      @pointerenter="keepOpen"
      @pointerleave="leave"
      @keydown="popupKey"
      @focusout="!$event.currentTarget.contains($event.relatedTarget) && close()"
    >
      <strong>{{ active.word }}</strong>
      <div class="correction-options">
        <button v-for="suggestion in active.suggestions" :key="suggestion" type="button" @click="correct(suggestion)">{{ suggestion }}</button>
        <span v-if="!active.suggestions.length" class="no-suggestions">No close matches.</span>
      </div>
    </div>
  </div>
</template>
