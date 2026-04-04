function initWallpapers() {
	document.querySelectorAll('[data-wallpaper-src]').forEach(function(wallpaperElement) {
		var image = new Image();

		image.onload = function() { wallpaperElement.style.opacity = '1'; };
		image.src = wallpaperElement.dataset.wallpaperSrc;
	});
}

document.addEventListener('DOMContentLoaded', initWallpapers);
document.addEventListener('htmx:afterSettle', initWallpapers);

