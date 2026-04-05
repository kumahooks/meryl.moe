const LINES = [
	'$ ./lain --connect',
	'$ ssh lain@layer7.lan',
	'$ ping wired.local',
	'$ mount /dev/wired /mnt/layer7',
];

const GLITCH_CHARS = '/\\|-·×#%@!?';

class PageTransition {
	#typewriterTimer = null;
	#pendingLoaderTimer = null;
	#requestInFlight = false;
	#lineIndex = 0;
	#generation = 0;
	#textElement = null;

	constructor() {
		this.#bindEvents();
	}

	#log(...args) {
		console.log('[PageTransition]', ...args);
	}

	#minimumDelay(ms) {
		return new Promise(resolve => setTimeout(resolve, ms));
	}

	#loadWallpapers() {
		const elements = Array.from(document.querySelectorAll('[data-wallpaper-src]'));

		let allComplete = true;

		const promises = elements.map(element => {
			return new Promise(resolve => {
				const image = new Image();
				image.src = element.dataset.wallpaperSrc;

				if (image.complete) {
					resolve();
					return;
				}

				allComplete = false;

				image.onload = resolve;
				image.onerror = resolve;
			});
		});

		return {
			promise: Promise.all(promises),
			allComplete
		};
	}

	#showMiddleman() {
		if (this.#typewriterTimer) {
			clearTimeout(this.#typewriterTimer);
			this.#typewriterTimer = null;
		}

		const generation = this.#generation;
		this.#lineIndex = 0;

		const container = document.getElementById('transition');
		if (!container) {
			return;
		}

		container.classList.remove('hidden');
		container.innerHTML = `
			<div class="middleman">
				<span class="middleman-text"></span>
				<span class="middleman-cursor">_</span>
			</div>
		`;

		this.#textElement = container.querySelector('.middleman-text');

		this.#runTypewriter(0, generation);
	}

	async #handleInitialLoad() {
		const generation = ++this.#generation;

		this.#showMiddleman();

		const { promise, allComplete } = this.#loadWallpapers();
		const delay = allComplete ? this.#minimumDelay(50) : this.#minimumDelay(500);
		await Promise.all([promise, delay]);

		if (generation !== this.#generation) return;

		this.#abortAnimation();
	}

	#abortAnimation() {
		this.#generation++;

		if (this.#typewriterTimer) {
			clearTimeout(this.#typewriterTimer);

			this.#typewriterTimer = null;
		}

		const container = document.getElementById('transition');
		if (container) {
			container.classList.add('hidden');
			container.innerHTML = '';
		}
	}

	#runTypewriter(index, generation) {
		if (generation !== this.#generation) return;

		const currentLine = LINES[this.#lineIndex];

		if (index <= currentLine.length) {
			this.#textElement.textContent = currentLine.slice(0, index);
			this.#typewriterTimer = setTimeout(() => this.#runTypewriter(index + 1, generation), 28);
		} else {
			this.#typewriterTimer = setTimeout(() => {
				let glitchCount = 0;

				const glitchInterval = setInterval(() => {
					if (generation !== this.#generation) {
						clearInterval(glitchInterval);
						return;
					}

					this.#textElement.textContent = PageTransition.#glitchString(currentLine);
					glitchCount++;

					if (glitchCount >= 6) {
						clearInterval(glitchInterval);

						this.#lineIndex = (this.#lineIndex + 1) % LINES.length;
						this.#textElement.textContent = '';
						this.#typewriterTimer = setTimeout(() => this.#runTypewriter(0, generation), 200);
					}
				}, 60);
			}, 200);
		}
	}

	static #glitchString(text) {
		return text.split('').map(character =>
			Math.random() < 0.3
				? GLITCH_CHARS[Math.floor(Math.random() * GLITCH_CHARS.length)]
				: character
		).join('');
	}

	#bindEvents() {
		document.addEventListener('htmx:beforeRequest', event => {
			const path = event.detail.requestConfig?.path;

			if (path === window.location.pathname) {
				event.preventDefault();
				return;
			}

			this.#generation++;
			this.#requestInFlight = true;

			const generation = this.#generation;

			this.#pendingLoaderTimer = setTimeout(() => {
				if (generation !== this.#generation) {
					return;
				}

				if (!this.#requestInFlight) {
					return;
				}

				this.#showMiddleman();
			}, 70);
		});

		document.addEventListener('htmx:afterSettle', async () => {
			this.#requestInFlight = false;

			if (this.#pendingLoaderTimer) {
				clearTimeout(this.#pendingLoaderTimer);
				this.#pendingLoaderTimer = null;
			}

			const generation = this.#generation;

			const { promise, allComplete } = this.#loadWallpapers();

			const delay = allComplete ? 0 : 500;

			await Promise.all([promise, this.#minimumDelay(delay)]);
			if (generation !== this.#generation) {
				return;
			}

			this.#abortAnimation();
		});

		document.addEventListener('DOMContentLoaded', () => {
			this.#handleInitialLoad();
		});

		document.addEventListener('htmx:responseError', () => this.#abortAnimation());
		document.addEventListener('htmx:sendError', () => this.#abortAnimation());
	}
}

new PageTransition();

