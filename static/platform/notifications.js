class ToastService {
	#container;
	#notifications = [];
	#maxNotifications = 3;
	#timers = new WeakMap();

	constructor() {
		this.#container = this.#getContainer();
		this.#bindEvents();
	}

	#getContainer() {
		if (!this.#container || !document.body.contains(this.#container)) {
			this.#container = document.getElementById('notifications');
		}

		return this.#container;
	}

	#push(detail) {
		const container = this.#container;
		if (!container) return;

		if (this.#notifications.length >= this.#maxNotifications) {
			this.#dismiss(this.#notifications[this.#notifications.length - 1]);
		}

		const element = this.#build(detail);
		if (!element) return;

		container.prepend(element);

		this.#notifications.unshift(element);

		const activeElement = document.activeElement;
		element.show();
		activeElement?.focus();

		requestAnimationFrame(() => {
			requestAnimationFrame(() => element.classList.add('notification--visible'));
		});

		const timeoutId = setTimeout(() => this.#dismiss(element), 5000);
		this.#timers.set(element, timeoutId);
	}

	#removeElement(element) {
		element.close();
		element.remove();

		this.#notifications = this.#notifications.filter(n => n !== element);
	}

	#dismiss(element) {
		if (!this.#notifications.includes(element)) return;

		const timer = this.#timers.get(element);
		if (timer) {
			clearTimeout(timer);
			this.#timers.delete(element);
		}

		element.classList.remove('notification--visible');

		let removed = false;
		const remove = () => {
			if (removed) return;

			removed = true;
			this.#removeElement(element);
		};

		element.addEventListener('transitionend', remove, { once: true });
		setTimeout(remove, 500);
	}

	#build(detail) {
		if (!detail || typeof detail !== 'object') return;

		const { message, link, linkDescription } = detail;
		if (typeof message !== 'string') return;

		const element = document.createElement('dialog');
		element.className = 'notification';

		const text = document.createElement('span');
		text.className = 'notification-text';
		text.textContent = message;

		element.appendChild(text);

		if (link) {
			const anchor = document.createElement('a');
			anchor.href = link;
			anchor.className = 'notification-link';
			anchor.target = '_blank';
			anchor.rel = 'noopener noreferrer';
			anchor.textContent = linkDescription ?? link ?? '';

			element.appendChild(anchor);
		}

		const closeBtn = document.createElement('button');
		closeBtn.type = 'button';
		closeBtn.className = 'notification-close';
		closeBtn.textContent = 'x';
		closeBtn.addEventListener('click', () => this.#dismiss(element));

		element.appendChild(closeBtn);

		return element;
	}

	#bindEvents() {
		document.addEventListener('notify', event => this.#push(event.detail));
		document.addEventListener('htmx:responseError', () => this.#push({ message: 'request failed' }));
	}
}

new ToastService();

