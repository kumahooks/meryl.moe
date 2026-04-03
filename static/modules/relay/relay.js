const textarea = document.getElementById('relay-input');
const charcount = document.getElementById('relay-charcount');

const inputData = new URLSearchParams(location.search).get('data');
if (inputData) {
	decompress(inputData).then(data => {
		textarea.value = data;
		charcount.textContent = data.length;
	});
}

let pendingCompression = null;

textarea.addEventListener('input', async () => {
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

