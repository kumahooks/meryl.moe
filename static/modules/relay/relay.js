import { compress, decompress } from '/static/js/codec.js';

const Mode = Object.freeze({
	NORMAL: 'NORMAL',
	INSERT: 'INSERT',
});

class RelayEditor {
	#mode = Mode.NORMAL;
	#pendingCompression = null;

	#textarea;
	#gutter;
	#charcount;
	#modeDisplay;
	#container;

	constructor(root) {
		this.#textarea = root.querySelector('#relay-input');
		this.#gutter = root.querySelector('#relay-gutter');
		this.#charcount = root.querySelector('#relay-charcount');
		this.#modeDisplay = root.querySelector('.relay-mode');
		this.#container = root;

		this.#restoreFromUrl();
		this.#updateGutter();
		this.#bindEvents();
	}

	#setMode(mode) {
		this.#mode = mode;
		this.#modeDisplay.textContent = mode;
		this.#container.classList.toggle('relay-container--insert', mode === Mode.INSERT);
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

	#moveCursor(direction) {
		const position = this.#textarea.selectionStart;
		const value = this.#textarea.value;
		const lines = value.split('\n');
		const textBefore = value.slice(0, position);
		const currentLineIndex = textBefore.split('\n').length - 1;

		let newPosition = position;

		switch (direction) {
			case 'left':
				newPosition = Math.max(0, position - 1);
				break;
			case 'right':
				newPosition = Math.min(value.length, position + 1);
				break;
			case 'up':
			case 'down': {
				const targetLineIndex = direction === 'up' ? currentLineIndex - 1 : currentLineIndex + 1;
				if (targetLineIndex < 0 || targetLineIndex >= lines.length) return;

				const currentLineStart = textBefore.lastIndexOf('\n') + 1;
				const column = position - currentLineStart;

				let targetLineStart = 0;
				for (let i = 0; i < targetLineIndex; i++) {
					targetLineStart += lines[i].length + 1;
				}

				newPosition = targetLineStart + Math.min(column, lines[targetLineIndex].length);
				break;
			}
		}

		this.#textarea.selectionStart = this.#textarea.selectionEnd = newPosition;
	}

	#onKeydown(event) {
		if (this.#mode === Mode.NORMAL) {
			if (event.ctrlKey || event.metaKey || event.altKey) return;

			event.preventDefault();

			switch (event.key) {
				case 'i': this.#setMode(Mode.INSERT); break;
				case 'h': case 'ArrowLeft': this.#moveCursor('left'); break;
				case 'j': case 'ArrowDown': this.#moveCursor('down'); break;
				case 'k': case 'ArrowUp': this.#moveCursor('up'); break;
				case 'l': case 'ArrowRight': this.#moveCursor('right'); break;
			}

			return;
		}

		if (event.key === 'Escape') {
			event.preventDefault();

			this.#setMode(Mode.NORMAL);

			return;
		}

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

