const LINES = [
	'$ ./lain --connect',
	'$ ssh lain@layer7.lan',
	'$ ping wired.local',
	'$ mount /dev/wired /mnt/layer7',
];

const GLITCH_CHARS = '/\\|-·×#%@!?';

class PageTransition {
	#typewriterTimer = null;
	#lineIndex = 0;
	#generation = 0;
	#textElement = null;

	constructor() {
		this.#loadWallpapers();
		this.#bindEvents();
	}

	#loadWallpapers() {
		document.querySelectorAll('[data-wallpaper-src]').forEach(element => {
			const image = new Image();
			image.src = element.dataset.wallpaperSrc;

			if (image.complete) {
				element.style.opacity = '1';
			} else {
				element.style.opacity = '0';

				image.onload = () => { element.style.opacity = '1'; };
				image.onerror = () => { element.style.opacity = '1'; };
			}
		});
	}

	#showMiddleman() {
		if (this.#typewriterTimer) {
			clearTimeout(this.#typewriterTimer);
			this.#typewriterTimer = null;
		}

		const generation = ++this.#generation;
		this.#lineIndex = 0;

		const pageContent = document.querySelector('.page-content');
		if (!pageContent) return;

		pageContent.innerHTML = '<div class="middleman"><span class="middleman-text"></span><span class="middleman-cursor">_</span></div>';
		this.#textElement = pageContent.querySelector('.middleman-text');

		this.#runTypewriter(0, generation);
	}

	#abortAnimation() {
		this.#generation++;

		if (this.#typewriterTimer) {
			clearTimeout(this.#typewriterTimer);
			this.#typewriterTimer = null;
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

			this.#showMiddleman();
		});

		document.addEventListener('htmx:afterSettle', () => {
			this.#abortAnimation();
			this.#loadWallpapers();
		});

		document.addEventListener('htmx:responseError', () => this.#abortAnimation());
		document.addEventListener('htmx:sendError', () => this.#abortAnimation());
	}
}

new PageTransition();

