class KippleUploader {
	constructor(dropZone) {
		this.dropZone = dropZone;
		this.pendingFiles = [];

		this.fileInput = document.getElementById('kipple-file-input');
		this.uploadBtn = document.getElementById('kipple-upload-btn');
		this.dialog = document.getElementById('kipple-upload-dialog');
		this.dialogFilename = document.getElementById('kipple-upload-filename');
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
			this.addFiles(event.dataTransfer.files);
		});

		this.fileInput.addEventListener('change', () => {
			if (this.fileInput.files.length) this.addFiles(this.fileInput.files);
			this.fileInput.value = '';
		});

		this.uploadBtn.addEventListener('click', () => {
			this.dialogFilename.textContent = this.pendingFiles.length === 1
				? this.pendingFiles[0].name
				: this.pendingFiles.length + ' files';
			this.dialog.showModal();
		});

		this.cancelBtn.addEventListener('click', () => {
			this.dialog.close('cancel');
		});

		this.dialog.addEventListener('close', () => {
			if (this.dialog.returnValue !== 'confirm') return;

			this.pendingFiles = [];
			this.render();
		});
	}

	formatSize(bytes) {
		if (bytes < 1024) return bytes + ' B';
		if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
		if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';

		return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GB';
	}

	render() {
		const totalLength = this.pendingFiles.length;

		if (totalLength === 0) {
			this.pendingElement.classList.remove('kipple-pending--visible');
			return;
		}

		this.pendingElement.classList.add('kipple-pending--visible');

		const totalBytes = this.pendingFiles.reduce((sum, file) => sum + file.size, 0);
		this.countElement.textContent = totalLength + ` file${totalLength > 1 ? 's' : ''} to upload`;
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
}

function init() {
	const dropZone = document.getElementById('kipple-drop-zone');
	if (!dropZone || dropZone.dataset.initialized) return;

	dropZone.dataset.initialized = 'true';
	new KippleUploader(dropZone);
}

document.addEventListener('DOMContentLoaded', init);
document.addEventListener('htmx:afterSettle', init);

init();

