const OVERLAY_ENABLED = true;

class PageLoader {
	// TODO: increase this text with multiple lines
	// maybe show more real commands, or actual code?
	static #text = '/ connecting to the wired...';
	static #glitchChars = '/\\|-·×#%@!?';

	#typewriterTimer = null;
	#debounceTimer = null;
	#visible = false;
	#mediaReady = false;

	#overlay = document.getElementById('page-overlay');
	#textElement = document.getElementById('page-overlay-text');

	constructor() {
		this.#bindEvents();
		this.#onInitialLoad();
	}

	#onInitialLoad() {
		if (!OVERLAY_ENABLED || window.location.pathname === '/') {
			this.#loadWallpapers(null);

			return;
		}

		this.#show();
		this.#loadWallpapers(() => {
			this.#mediaReady = true;
			this.#tryHide();
		});
	}

	#show() {
		if (!this.#overlay) return;

		if (this.#typewriterTimer) {
			clearTimeout(this.#typewriterTimer);

			this.#typewriterTimer = null;
		}

		this.#mediaReady = false;
		this.#visible = true;
		this.#overlay.classList.add('is-loading');

		if (this.#textElement) {
			this.#textElement.textContent = '';
			this.#runTypewriter(0);
		}
	}

	#hide() {
		if (!this.#overlay) return;

		if (this.#typewriterTimer) {
			clearTimeout(this.#typewriterTimer);
			this.#typewriterTimer = null;
		}

		this.#visible = false;
		this.#overlay.classList.add('is-hiding');
		this.#overlay.addEventListener('transitionend', () => {
			this.#overlay.classList.remove('is-loading', 'is-hiding');
		}, { once: true });
	}

	#tryHide() {
		if (this.#mediaReady && this.#visible) this.#hide();
	}

	#scheduleShow() {
		this.#cancelSchedule();

		this.#debounceTimer = setTimeout(() => {
			this.#debounceTimer = null;
			this.#show();
		}, 150);
	}

	#cancelSchedule() {
		if (this.#debounceTimer) {
			clearTimeout(this.#debounceTimer);
			this.#debounceTimer = null;
		}
	}

	#runTypewriter(index) {
		if (index <= PageLoader.#text.length) {
			this.#textElement.textContent = PageLoader.#text.slice(0, index);
			this.#typewriterTimer = setTimeout(() => this.#runTypewriter(index + 1), 38);
		} else {
			this.#typewriterTimer = setTimeout(() => {
				let glitchCount = 0;

				const glitchInterval = setInterval(() => {
					this.#textElement.textContent = PageLoader.#glitchString(PageLoader.#text);

					glitchCount++;

					if (glitchCount >= 6) {
						clearInterval(glitchInterval);

						this.#textElement.textContent = '';
						this.#typewriterTimer = setTimeout(() => this.#runTypewriter(0), 200);
					}
				}, 60);
			}, 800);
		}
	}

	static #glitchString(text) {
		return text.split('').map(character =>
			Math.random() < 0.3
				? PageLoader.#glitchChars[Math.floor(Math.random() * PageLoader.#glitchChars.length)]
				: character
		).join('');
	}

	#loadWallpapers(onReady) {
		const wallpapers = document.querySelectorAll('[data-wallpaper-src]');

		if (wallpapers.length === 0) {
			onReady?.();

			return;
		}

		let remaining = wallpapers.length;

		wallpapers.forEach(wallpaperElement => {
			const image = new Image();
			image.src = wallpaperElement.dataset.wallpaperSrc;

			if (image.complete) {
				wallpaperElement.style.opacity = '1';
				remaining--;

				if (remaining === 0) onReady?.();
			} else {
				wallpaperElement.style.opacity = '0';

				image.onload = () => {
					wallpaperElement.style.opacity = '1';
					remaining--;

					if (remaining === 0) onReady?.();
				};

				image.onerror = () => {
					remaining--;

					if (remaining === 0) onReady?.();
				};
			}
		});
	}

	#bindEvents() {
		document.addEventListener('htmx:beforeRequest', event => {
			if (!OVERLAY_ENABLED) return;

			const path = event.detail.requestConfig?.path;
			if (path === '/') return;

			this.#scheduleShow();
		});

		document.addEventListener('htmx:afterSettle', () => {
			this.#loadWallpapers(() => {
				this.#cancelSchedule();
				this.#mediaReady = true;
				this.#tryHide();
			});
		});

		document.addEventListener('htmx:responseError', () => this.#hide());
		document.addEventListener('htmx:sendError', () => this.#hide());
	}
}

new PageLoader();

