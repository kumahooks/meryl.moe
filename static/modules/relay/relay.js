import { compress, decompress } from '/static/js/codec.js';

class RelayEditor {
	#pendingCompression = null;

	#textarea;
	#gutter;
	#charcount;

	constructor(root) {
		this.#textarea = root.querySelector('#relay-input');
		this.#gutter = root.querySelector('#relay-gutter');
		this.#charcount = root.querySelector('#relay-charcount');

		this.#restoreFromUrl();
		this.#updateGutter();
		this.#bindEvents();
	}

	#updateGutter() {
		const lines = this.#textarea.value.split('\n');
		const currentLine = this.#textarea.value.slice(0, this.#textarea.selectionStart).split('\n').length;

		this.#gutter.innerHTML = lines.map((_, index) => {
			const distance = Math.abs(index + 1 - currentLine);
			const isCurrent = distance === 0;

			return `<span class="relay-line-number${isCurrent ? ' relay-line-number--current' : ''}">${isCurrent ? currentLine : distance}</span>`;
		}).join('');

		this.#gutter.scrollTop = this.#textarea.scrollTop;
	}

	#onKeydown(event) {
		if (event.key === 'Tab') {
			event.preventDefault();

			const start = this.#textarea.selectionStart;
			const end = this.#textarea.selectionEnd;

			this.#textarea.value = this.#textarea.value.slice(0, start) + '\t' + this.#textarea.value.slice(end);
			this.#textarea.selectionStart = this.#textarea.selectionEnd = start + 1;
			this.#textarea.dispatchEvent(new Event('input'));
		}
	}

	async #onInput() {
		this.#updateGutter();
		this.#charcount.textContent = this.#textarea.value.length;

		this.#pendingCompression?.abort();

		if (!this.#textarea.value) {
			history.replaceState(null, '', location.pathname);

			return;
		}

		this.#pendingCompression = new AbortController();
		const { signal } = this.#pendingCompression;

		const compressed = await compress(this.#textarea.value);
		if (!signal.aborted) {
			history.replaceState(null, '', `${location.pathname}?data=${compressed}`);
		}
	}

	#restoreFromUrl() {
		const inputData = new URLSearchParams(location.search).get('data');
		if (!inputData) return;

		decompress(inputData).then(data => {
			this.#textarea.value = data;
			this.#charcount.textContent = data.length;
			this.#updateGutter();
		});
	}

	#bindEvents() {
		this.#textarea.addEventListener('keydown', event => this.#onKeydown(event));

		this.#textarea.addEventListener('input', () => this.#onInput());

		this.#textarea.addEventListener('scroll', () => {
			this.#gutter.scrollTop = this.#textarea.scrollTop;
		});

		document.addEventListener('selectionchange', () => {
			if (document.activeElement === this.#textarea) this.#updateGutter();
		});
	}
}

function init() {
	const container = document.querySelector('.relay-container');
	if (!container) return;

	new RelayEditor(container);
}

document.addEventListener('DOMContentLoaded', init);
document.addEventListener('htmx:afterSettle', init);

