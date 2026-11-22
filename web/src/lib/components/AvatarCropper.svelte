<script lang="ts">
	import { AVATAR_SIZE, encodeCanvas, loadImage } from '$lib/image';

	// Shows the picked photo under a circle: drag to move, pinch or use the slider to zoom.
	// What is inside the circle is exactly what becomes the profile picture.
	let { file, onsave, oncancel }: { file: File; onsave: (blob: Blob) => void | Promise<void>; oncancel: () => void } =
		$props();

	const V = 280; // viewport size in CSS px
	let img = $state<HTMLImageElement | null>(null);
	let error = $state('');
	let zoom = $state(1); // 1 = the photo just covers the circle
	let x = $state(0); // image top-left inside the viewport
	let y = $state(0);
	let saving = $state(false);
	let viewport = $state<HTMLDivElement | null>(null);

	// Registered by hand so preventDefault works (the page must not scroll while zooming).
	$effect(() => {
		const el = viewport;
		if (!el) return;
		el.addEventListener('wheel', wheel, { passive: false });
		return () => el.removeEventListener('wheel', wheel);
	});

	const base = $derived(img ? Math.max(V / img.naturalWidth, V / img.naturalHeight) : 1);
	const s = $derived(base * zoom);
	const w = $derived(img ? img.naturalWidth * s : 0);
	const h = $derived(img ? img.naturalHeight * s : 0);

	$effect(() => {
		let cancelled = false;
		loadImage(file)
			.then((i) => {
				if (cancelled) return;
				img = i;
				const b = Math.max(V / i.naturalWidth, V / i.naturalHeight);
				x = (V - i.naturalWidth * b) / 2;
				y = (V - i.naturalHeight * b) / 2;
			})
			.catch((e) => (error = e.message));
		return () => {
			cancelled = true;
			if (img) URL.revokeObjectURL(img.src);
		};
	});

	function clamp() {
		x = Math.min(0, Math.max(V - w, x));
		y = Math.min(0, Math.max(V - h, y));
	}

	/** Zooms keeping the viewport point (px, py) still. */
	function setZoom(next: number, px = V / 2, py = V / 2) {
		next = Math.min(4, Math.max(1, next));
		const before = s;
		const ix = (px - x) / before;
		const iy = (py - y) / before;
		zoom = next;
		const after = base * next;
		x = px - ix * after;
		y = py - iy * after;
		clamp();
	}

	// Pointer handling: one finger drags, two fingers pinch.
	const pointers = new Map<number, { x: number; y: number }>();
	let pinchStart = 0;
	let zoomStart = 1;

	function down(e: PointerEvent) {
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
		pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
		if (pointers.size === 2) {
			const [a, b] = [...pointers.values()];
			pinchStart = Math.hypot(a.x - b.x, a.y - b.y);
			zoomStart = zoom;
		}
	}

	function move(e: PointerEvent) {
		const prev = pointers.get(e.pointerId);
		if (!prev) return;
		const cur = { x: e.clientX, y: e.clientY };
		pointers.set(e.pointerId, cur);
		if (pointers.size === 1) {
			x += cur.x - prev.x;
			y += cur.y - prev.y;
			clamp();
		} else if (pointers.size === 2 && pinchStart > 0) {
			const [a, b] = [...pointers.values()];
			const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
			const mid = { x: (a.x + b.x) / 2 - rect.left, y: (a.y + b.y) / 2 - rect.top };
			setZoom((zoomStart * Math.hypot(a.x - b.x, a.y - b.y)) / pinchStart, mid.x, mid.y);
		}
	}

	function up(e: PointerEvent) {
		pointers.delete(e.pointerId);
		if (pointers.size < 2) pinchStart = 0;
	}

	function wheel(e: WheelEvent) {
		e.preventDefault();
		const rect = viewport!.getBoundingClientRect();
		setZoom(zoom * (e.deltaY < 0 ? 1.08 : 1 / 1.08), e.clientX - rect.left, e.clientY - rect.top);
	}

	function key(e: KeyboardEvent) {
		const step = 10;
		const moves: Record<string, [number, number]> = {
			ArrowLeft: [step, 0],
			ArrowRight: [-step, 0],
			ArrowUp: [0, step],
			ArrowDown: [0, -step]
		};
		if (moves[e.key]) {
			e.preventDefault();
			x += moves[e.key][0];
			y += moves[e.key][1];
			clamp();
		} else if (e.key === '+' || e.key === '=') setZoom(zoom * 1.1);
		else if (e.key === '-') setZoom(zoom / 1.1);
	}

	async function save() {
		if (!img) return;
		saving = true;
		try {
			const canvas = document.createElement('canvas');
			canvas.width = canvas.height = AVATAR_SIZE;
			const ctx = canvas.getContext('2d')!;
			ctx.imageSmoothingQuality = 'high';
			ctx.drawImage(img, -x / s, -y / s, V / s, V / s, 0, 0, AVATAR_SIZE, AVATAR_SIZE);
			await onsave(await encodeCanvas(canvas, 0.82));
		} catch (e) {
			error = (e as Error).message;
		} finally {
			saving = false;
		}
	}
</script>

<div class="backdrop" role="dialog" aria-modal="true" aria-labelledby="crop-title">
	<div class="sheet">
		<h2 id="crop-title" class="h2">Ajustar foto</h2>
		<p class="muted">Arraste para posicionar e use o zoom. O que estiver no círculo vira sua foto.</p>

		{#if error}
			<div class="error" role="alert">{error}</div>
		{:else}
			<div
				bind:this={viewport}
				class="viewport"
				style="width: {V}px; height: {V}px"
				role="slider"
				tabindex="0"
				aria-label="Posição da foto: use as setas para mover e + ou − para zoom"
				aria-valuemin={1}
				aria-valuemax={4}
				aria-valuenow={Math.round(zoom * 100) / 100}
				onpointerdown={down}
				onpointermove={move}
				onpointerup={up}
				onpointercancel={up}
				onkeydown={key}
			>
				{#if img}
					<img src={img.src} alt="" draggable="false" style="width: {w}px; height: {h}px; transform: translate({x}px, {y}px)" />
				{/if}
				<div class="mask" aria-hidden="true"></div>
			</div>

			<label class="zoom">
				<span class="muted">Zoom</span>
				<input type="range" min="1" max="4" step="0.01" value={zoom} oninput={(e) => setZoom(Number(e.currentTarget.value))} />
			</label>
		{/if}

		<div class="actions">
			<button class="btn ghost" type="button" onclick={oncancel} disabled={saving}>Cancelar</button>
			<button class="btn" type="button" onclick={save} disabled={!img || saving}>{saving ? 'Salvando…' : 'Usar foto'}</button>
		</div>
	</div>
</div>

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		z-index: 60;
		background: rgba(0, 0, 0, 0.75);
		display: flex;
		align-items: flex-end;
		justify-content: center;
	}
	.sheet {
		width: 100%;
		max-width: 520px;
		background: var(--surface);
		border-radius: 28px 28px 0 0;
		padding: 22px 20px calc(22px + env(safe-area-inset-bottom));
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 14px;
	}
	.sheet p {
		margin: 0;
		text-align: center;
	}
	.viewport {
		position: relative;
		overflow: hidden;
		border-radius: 20px;
		background: #000;
		touch-action: none;
		cursor: grab;
		user-select: none;
	}
	.viewport:active {
		cursor: grabbing;
	}
	.viewport img {
		position: absolute;
		left: 0;
		top: 0;
		transform-origin: 0 0;
		max-width: none;
		pointer-events: none;
	}
	/* Darkens everything outside the circle and outlines it. */
	.mask {
		position: absolute;
		inset: 0;
		border-radius: 50%;
		box-shadow: 0 0 0 999px rgba(11, 15, 13, 0.62);
		outline: 2px solid var(--accent);
		outline-offset: -2px;
		pointer-events: none;
	}
	.zoom {
		width: 100%;
		display: flex;
		align-items: center;
		gap: 12px;
	}
	.zoom input {
		flex: 1;
		accent-color: var(--accent);
		height: 44px;
	}
	.actions {
		width: 100%;
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 10px;
	}
</style>
