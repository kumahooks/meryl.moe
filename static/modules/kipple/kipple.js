class KippleTusUploader {
	constructor(file, options) {
		this.file = file;
		this.options = options;
		this.uploadUrl = null;
		this.chunkSize = 8 * 1024 * 1024;
		this.abortController = null;
		this.storageKey = `kipple:${file.name}:${file.size}:${file.lastModified}`;
	}

	async start() {
		this.abortController = new AbortController();

		const stored = localStorage.getItem(this.storageKey);
		if (stored) {
			const resumed = await this.tryResume(stored);
			if (!resumed) await this.create();
		} else {
			await this.create();
		}

		let offset = await this.currentOffset();
		while (offset < this.file.size) {
			offset = await this.sendChunk(offset);
		}

		localStorage.removeItem(this.storageKey);
	}

	abort() {
		this.abortController?.abort();
	}

	async tryResume(uploadUrl) {
		try {
			const response = await fetch(uploadUrl, { method: 'HEAD', signal: this.abortController.signal });
			if (!response.ok) return false;

			this.uploadUrl = uploadUrl;

			return true;
		} catch {
			return false;
		}
	}

	async create() {
		const metadataHeader = Object.entries(this.options.metadata)
			.map(([key, value]) => `${key} ${btoa(String.fromCharCode(...new TextEncoder().encode(String(value))))}`)
			.join(', ');

		const response = await fetch('/kipple/upload', {
			method: 'POST',
			headers: {
				'Upload-Length': String(this.file.size),
				'Upload-Metadata': metadataHeader,
			},
			signal: this.abortController.signal,
		});

		if (!response.ok) throw new Error(`create: ${response.status}`);

		this.uploadUrl = response.headers.get('Location');
		if (!this.uploadUrl) throw new Error('create: no Location header');

		localStorage.setItem(this.storageKey, this.uploadUrl);
	}

	async currentOffset() {
		const response = await fetch(this.uploadUrl, { method: 'HEAD', signal: this.abortController.signal });
		if (!response.ok) throw new Error(`head: ${response.status}`);

		return parseInt(response.headers.get('Upload-Offset') || '0', 10);
	}

	async sendChunk(offset) {
		const chunk = this.file.slice(offset, offset + this.chunkSize);
		const buffer = await chunk.arrayBuffer();
		const hashBuffer = await crypto.subtle.digest('SHA-1', buffer);
		const hashBase64 = btoa(String.fromCharCode(...new Uint8Array(hashBuffer)));

		const response = await fetch(this.uploadUrl, {
			method: 'PATCH',
			headers: {
				'Content-Type': 'application/offset+octet-stream',
				'Upload-Offset': String(offset),
				'Content-Length': String(chunk.size),
				'Upload-Checksum': `sha1 ${hashBase64}`,
			},
			body: chunk,
			signal: this.abortController.signal,
		});

		if (response.status === 409) return await this.currentOffset();
		if (!response.ok) throw new Error(`patch: ${response.status}`);

		const newOffset = parseInt(response.headers.get('Upload-Offset') || '0', 10);

		this.options.onProgress?.(newOffset, this.file.size);

		return newOffset;
	}
}

class KippleUploader {
	constructor(dropZone) {
		this.dropZone = dropZone;
		this.pendingFiles = [];
		this.uploading = false;
		this.cancelled = false;
		this.currentUploader = null;

		this.fileInput = document.getElementById('kipple-file-input');
		this.uploadBtn = document.getElementById('kipple-upload-btn');
		this.uploadCancelBtn = document.getElementById('kipple-cancel-btn');
		this.dialog = document.getElementById('kipple-upload-dialog');
		this.dialogFilename = document.getElementById('kipple-upload-filename');
		this.confirmBtn = document.getElementById('kipple-upload-confirm');
		this.cancelBtn = this.dialog.querySelector('.relay-dialog-btn--cancel');
		this.pendingElement = document.getElementById('kipple-pending');
		this.listElement = document.getElementById('kipple-pending-list');
		this.countElement = document.getElementById('kipple-pending-count');
		this.totalElement = document.getElementById('kipple-pending-total');

		this.dropZone.addEventListener('dragover', event => {
			event.preventDefault();

			this.dropZone.classList.add('kipple-drop-zone--dragover');
		});

		this.dropZone.addEventListener('dragleave', () => {
			this.dropZone.classList.remove('kipple-drop-zone--dragover');
		});

		this.dropZone.addEventListener('drop', event => {
			event.preventDefault();

			this.dropZone.classList.remove('kipple-drop-zone--dragover');

			if (!this.uploading) this.addFiles(event.dataTransfer.files);
		});

		this.fileInput.addEventListener('change', () => {
			if (this.fileInput.files.length && !this.uploading) this.addFiles(this.fileInput.files);

			this.fileInput.value = '';
		});

		this.uploadBtn.addEventListener('click', () => {
			if (this.uploading || !this.pendingFiles.length) return;

			this.dialogFilename.textContent = this.pendingFiles.length === 1
				? this.pendingFiles[0].name
				: this.pendingFiles.length + ' files';

			this.dialog.showModal();
		});

		this.confirmBtn.addEventListener('click', () => {
			this.dialog.close('confirm');
		});

		this.cancelBtn.addEventListener('click', () => {
			this.dialog.close('cancel');
		});

		this.dialog.addEventListener('close', () => {
			if (this.dialog.returnValue !== 'confirm') return;

			this.startUpload();
		});

		this.uploadCancelBtn.addEventListener('click', () => {
			if (!this.uploading) return;

			this.cancelled = true;

			const uploader = this.currentUploader;
			uploader?.abort();

			if (uploader?.uploadUrl) {
				htmx.ajax('DELETE', uploader.uploadUrl, { swap: 'none' });
				if (uploader.storageKey) localStorage.removeItem(uploader.storageKey);
			}
		});
	}

	formatSize(bytes) {
		if (bytes < 1024) return bytes + ' B';
		if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
		if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';

		return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GB';
	}

	render() {
		const count = this.pendingFiles.length;

		if (count === 0) {
			this.pendingElement.classList.remove('kipple-pending--visible');
			return;
		}

		this.pendingElement.classList.add('kipple-pending--visible');

		const totalBytes = this.pendingFiles.reduce((sum, file) => sum + file.size, 0);
		this.countElement.textContent = count + ` file${count > 1 ? 's' : ''} to upload`;
		this.totalElement.textContent = this.formatSize(totalBytes) + ' total size';

		this.listElement.innerHTML = '';

		this.pendingFiles.forEach((file, index) => {
			const item = document.createElement('li');
			item.className = 'kipple-pending-item';

			const nameSpan = document.createElement('span');
			nameSpan.className = 'kipple-pending-name';
			nameSpan.textContent = file.name;

			const sizeSpan = document.createElement('span');
			sizeSpan.className = 'kipple-pending-size';
			sizeSpan.textContent = this.formatSize(file.size);

			const removeBtn = document.createElement('button');
			removeBtn.className = 'kipple-del';
			removeBtn.innerHTML = 'del<span class="kipple-marker">*</span>';
			removeBtn.addEventListener('click', () => {
				if (this.uploading) return;

				this.pendingFiles.splice(index, 1);
				this.render();
			});

			item.append(nameSpan, sizeSpan, removeBtn);
			this.listElement.appendChild(item);
		});
	}

	addFiles(fileList) {
		for (const file of fileList) this.pendingFiles.push(file);
		this.render();
	}

	renderUploadList(files, currentIndex, failed) {
		this.pendingElement.classList.add('kipple-pending--visible');
		this.countElement.textContent = `uploading ${currentIndex + 1} of ${files.length}`;
		this.totalElement.textContent = '';

		this.listElement.innerHTML = '';

		files.forEach((file, index) => {
			const item = document.createElement('li');
			item.className = 'kipple-pending-item';

			const nameSpan = document.createElement('span');
			nameSpan.className = 'kipple-pending-name';
			nameSpan.textContent = file.name;

			const statusSpan = document.createElement('span');
			statusSpan.className = 'kipple-pending-size';

			if (index < currentIndex) {
				statusSpan.textContent = failed.has(index) ? 'failed' : '100%';
			} else if (index === currentIndex) {
				statusSpan.textContent = '0%';
			} else {
				statusSpan.textContent = '\u2013';
			}

			item.append(nameSpan, statusSpan);
			this.listElement.appendChild(item);
		});
	}

	startUpload() {
		const visibility = this.dialog.querySelector('input[name="visibility"]:checked')?.value || 'link';
		const expireDays = this.dialog.querySelector('input[name="expire_days"]:checked')?.value || '1';

		const files = [...this.pendingFiles];

		this.pendingFiles = [];
		this.cancelled = false;
		this.uploading = true;
		this.uploadBtn.classList.add('kipple-pending-btn--hidden');
		this.uploadCancelBtn.classList.remove('kipple-pending-btn--hidden');

		this.uploadSequentially(files, visibility, expireDays).finally(() => {
			this.uploading = false;
			this.cancelled = false;
			this.currentUploader = null;
			this.uploadBtn.classList.remove('kipple-pending-btn--hidden');
			this.uploadCancelBtn.classList.add('kipple-pending-btn--hidden');
			this.pendingElement.classList.remove('kipple-pending--visible');
		});
	}

	async uploadSequentially(files, visibility, expireDays) {
		const failed = new Set();

		for (let index = 0; index < files.length; index++) {
			if (this.cancelled) break;

			const file = files[index];

			this.renderUploadList(files, index, failed);

			this.currentUploader = new KippleTusUploader(file, {
				metadata: { filename: file.name, visibility, expire_days: expireDays },
				onProgress: (uploaded, total) => {
					const items = this.listElement.querySelectorAll('.kipple-pending-item');

					const statusSpan = items[index]?.querySelector('.kipple-pending-size');
					if (statusSpan) statusSpan.textContent = Math.round(uploaded / total * 100) + '%';
				},
			});

			try {
				await this.currentUploader.start();
			} catch (error) {
				if (this.cancelled) break;

				failed.add(index);

				document.dispatchEvent(new CustomEvent('notify', {
					detail: { message: `upload failed: ${file.name}` },
				}));
			}
		}

		if (this.cancelled) {
			document.dispatchEvent(new CustomEvent('notify', {
				detail: { message: 'upload cancelled' },
			}));
		} else if (failed.size < files.length) {
			const uploaded = files.length - failed.size;

			document.dispatchEvent(new CustomEvent('notify', {
				detail: { message: `${uploaded} file${uploaded > 1 ? 's' : ''} uploaded` },
			}));
		}

		document.dispatchEvent(new CustomEvent('kippleUploaded'));
	}
}

function init() {
	const dropZone = document.getElementById('kipple-drop-zone');
	if (!dropZone || dropZone.dataset.initialized) return;

	dropZone.dataset.initialized = 'true';
	new KippleUploader(dropZone);
}

document.addEventListener('DOMContentLoaded', init);
document.addEventListener('htmx:afterSettle', init);

document.addEventListener('htmx:confirm', event => {
	if (!event.detail.question) return;

	const dialog = document.getElementById('kipple-delete-dialog');
	if (!dialog) {
		event.detail.issueRequest(true);
		return;
	}

	event.preventDefault();

	const filenameEl = document.getElementById('kipple-delete-filename');
	if (filenameEl) filenameEl.textContent = event.detail.elt.dataset.filename || '';

	dialog.returnValue = '';

	const confirmBtn = document.getElementById('kipple-delete-confirm');
	const cancelBtn = dialog.querySelector('.relay-dialog-btn--cancel');

	confirmBtn.addEventListener('click', () => dialog.close('confirm'), { once: true });
	cancelBtn.addEventListener('click', () => dialog.close('cancel'), { once: true });

	dialog.addEventListener('close', () => {
		if (dialog.returnValue === 'confirm') event.detail.issueRequest(true);
	}, { once: true });

	dialog.showModal();
});

init();

