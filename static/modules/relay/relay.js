function init() {
	const textarea = document.getElementById('relay-input');
	if (!textarea) return;

	const charcount = document.getElementById('relay-charcount');
	const gutter = document.getElementById('relay-gutter');

	function updateGutter() {
		const lines = textarea.value.split('\n');
		const currentLine = textarea.value.slice(0, textarea.selectionStart).split('\n').length;

		gutter.innerHTML = lines.map((_, index) => {
			const distance = Math.abs(index + 1 - currentLine);
			const isCurrent = distance === 0;

			return `<span class="relay-line-number${isCurrent ? ' relay-line-number--current' : ''}">${isCurrent ? currentLine : distance}</span>`;
		}).join('');

		gutter.scrollTop = textarea.scrollTop;
	}

	const inputData = new URLSearchParams(location.search).get('data');
	if (inputData) {
		decompress(inputData).then(data => {
			textarea.value = data;
			charcount.textContent = data.length;
			updateGutter();
		});
	}

	updateGutter();

	textarea.addEventListener('keydown', (event) => {
		if (event.key !== 'Tab') return;

		event.preventDefault();

		const start = textarea.selectionStart;
		const end = textarea.selectionEnd;

		textarea.value = textarea.value.slice(0, start) + '\t' + textarea.value.slice(end);
		textarea.selectionStart = textarea.selectionEnd = start + 1;
		textarea.dispatchEvent(new Event('input'));
	});

	textarea.addEventListener('scroll', () => {
		gutter.scrollTop = textarea.scrollTop;
	});

	document.addEventListener('selectionchange', () => {
		if (document.activeElement === textarea) updateGutter();
	});


	let pendingCompression = null;

	textarea.addEventListener('input', async () => {
		updateGutter();
		charcount.textContent = textarea.value.length;

		pendingCompression?.abort();

		if (!textarea.value) {
			history.replaceState(null, '', location.pathname);
			return;
		}

		pendingCompression = new AbortController();
		const { signal } = pendingCompression;

		const compressed = await compress(textarea.value);
		if (!signal.aborted) {
			history.replaceState(null, '', `${location.pathname}?data=${compressed}`);
		}
	});
}

document.addEventListener('DOMContentLoaded', init);
document.addEventListener('htmx:afterSettle', init);

async function compress(text) {
	const stream = new CompressionStream('deflate-raw');
	const writer = stream.writable.getWriter();

	writer.write(new TextEncoder().encode(text));
	writer.close();

	const buffer = await new Response(stream.readable).arrayBuffer();

	return toBase64url(new Uint8Array(buffer));
}

async function decompress(base64url) {
	const stream = new DecompressionStream('deflate-raw');
	const writer = stream.writable.getWriter();

	writer.write(fromBase64url(base64url));
	writer.close();

	return new Response(stream.readable).text();
}

function toBase64url(bytes) {
	return btoa(Array.from(bytes, b => String.fromCharCode(b)).join(''))
		.replaceAll('+', '-').replaceAll('/', '_').replaceAll('=', '');
}

function fromBase64url(base64url) {
	return Uint8Array.from(atob(base64url.replaceAll('-', '+').replaceAll('_', '/')), c => c.charCodeAt(0));
}

