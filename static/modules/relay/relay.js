import { compress, decompress } from '/static/modules/relay/codec.js';

class RelayEditor {
	#pendingCompression = null;
	#pendingFileContent = null;
	#dragCounter = 0;
	#saveDialog = null;

	#textarea;
	#gutter;
	#charcount;
	#loadDialog;
	#dialogFilename;

	constructor(root) {
		this.#textarea = root.querySelector('#relay-input');
		this.#gutter = root.querySelector('#relay-gutter');
		this.#charcount = root.querySelector('#relay-charcount');
		this.#loadDialog = root.querySelector('#relay-load-dialog');
		this.#dialogFilename = root.querySelector('.relay-dialog-filename');
		this.#saveDialog = root.querySelector('#relay-save-dialog');

		this.#charcount.textContent = this.#textarea.value.length;
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

	#showDialog(filename, content) {
		this.#pendingFileContent = content;
		this.#dialogFilename.textContent = filename;
		this.#loadDialog.showModal();
		this.#loadDialog.querySelector('.relay-dialog-btn--confirm').focus();
	}

	#hideDialog() {
		this.#loadDialog.close();
	}

	#showSaveDialog() {
		this.#saveDialog.showModal();
		this.#saveDialog.querySelector('.relay-dialog-btn--confirm').focus();
	}

	#hideSaveDialog() {
		this.#saveDialog.close();
	}

	#confirmLoad() {
		this.#textarea.focus();
		this.#textarea.select();

		// TODO: This is deprecated, but is there even an alternative?
		document.execCommand('insertText', false, this.#pendingFileContent);

		this.#hideDialog();
		this.#textarea.dispatchEvent(new Event('input'));
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

		const container = this.#textarea.closest('.relay-container');

		container.addEventListener('dragenter', event => {
			event.preventDefault();
			this.#dragCounter++;
			container.classList.add('relay-container--dragover');
		});

		container.addEventListener('dragleave', () => {
			this.#dragCounter--;
			if (this.#dragCounter === 0) container.classList.remove('relay-container--dragover');
		});

		container.addEventListener('dragover', event => event.preventDefault());

		container.addEventListener('drop', event => {
			event.preventDefault();
			this.#dragCounter = 0;
			container.classList.remove('relay-container--dragover');

			const file = event.dataTransfer.files[0];
			if (!file) return;

			const reader = new FileReader();
			reader.onload = () => this.#showDialog(file.name, reader.result);
			reader.readAsText(file);
		});

		this.#loadDialog.querySelector('.relay-dialog-btn--confirm').addEventListener('click', () => this.#confirmLoad());
		this.#loadDialog.querySelector('.relay-dialog-btn--cancel').addEventListener('click', () => this.#hideDialog());

		this.#loadDialog.addEventListener('click', event => {
			if (event.target === this.#loadDialog) this.#hideDialog();
		});

		this.#loadDialog.addEventListener('keydown', event => {
			if (event.key === 'Enter') this.#confirmLoad();
		});

		this.#loadDialog.addEventListener('close', () => {
			this.#pendingFileContent = null;
			this.#textarea.focus();
		});

		container.addEventListener('click', event => {
			if (event.target.closest('.relay-panel-close, .relay-panel-backdrop')) {
				const panel = document.getElementById('relay-panel');
				if (panel) panel.outerHTML = '<div id="relay-panel" class="relay-panel"></div>';
			}
		});

		const saveTriggerBtn = container.querySelector('#relay-save-btn');
		if (saveTriggerBtn) {
			const saveConfirmBtn = this.#saveDialog.querySelector('.relay-dialog-btn--confirm');
			const saveCancelBtn = this.#saveDialog.querySelector('.relay-dialog-btn--cancel');

			saveTriggerBtn.addEventListener('click', () => this.#showSaveDialog());
			saveConfirmBtn.addEventListener('click', () => this.#hideSaveDialog());
			saveCancelBtn.addEventListener('click', () => this.#hideSaveDialog());

			this.#saveDialog.addEventListener('click', event => {
				if (event.target === this.#saveDialog) this.#hideSaveDialog();
			});

			this.#saveDialog.addEventListener('keydown', event => {
				if (event.key === 'Enter') saveConfirmBtn.click();
			});

			this.#saveDialog.addEventListener('close', () => {
				this.#textarea.focus();
			});
		}
	}
}

function init() {
	const container = document.querySelector('.relay-container');
	if (!container || container.dataset.relayInitialized) return;

	container.dataset.relayInitialized = 'true';
	new RelayEditor(container);
}

document.addEventListener('DOMContentLoaded', init);
document.addEventListener('htmx:afterSettle', init);

init();

