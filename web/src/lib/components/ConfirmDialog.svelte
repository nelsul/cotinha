<script lang="ts">
	import type { Snippet } from 'svelte';

	// A small bottom sheet that asks before something that can't be undone.
	let {
		title,
		confirmLabel = 'Confirmar',
		danger = false,
		busy = false,
		onconfirm,
		oncancel,
		children
	}: {
		title: string;
		confirmLabel?: string;
		danger?: boolean;
		busy?: boolean;
		onconfirm: () => void;
		oncancel: () => void;
		children?: Snippet;
	} = $props();

	let cancelButton = $state<HTMLButtonElement | null>(null);
	$effect(() => cancelButton?.focus());

	function onkeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && !busy) oncancel();
	}
</script>

<svelte:window {onkeydown} />

<div class="backdrop" role="presentation" onclick={(e) => e.target === e.currentTarget && !busy && oncancel()}>
	<div class="sheet" role="alertdialog" aria-modal="true" aria-labelledby="confirm-title" aria-describedby="confirm-body">
		<h2 id="confirm-title" class="h2">{title}</h2>
		<div id="confirm-body" class="body">{@render children?.()}</div>
		<div class="actions">
			<button bind:this={cancelButton} class="btn ghost" type="button" onclick={oncancel} disabled={busy}>Cancelar</button>
			<button class="btn" class:danger type="button" onclick={onconfirm} disabled={busy}>{busy ? 'Aguarde…' : confirmLabel}</button>
		</div>
	</div>
</div>

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		z-index: 70;
		background: rgba(0, 0, 0, 0.7);
		display: flex;
		align-items: flex-end;
		justify-content: center;
	}
	.sheet {
		width: 100%;
		max-width: 520px;
		background: var(--surface);
		border-radius: 28px 28px 0 0;
		padding: 24px 20px calc(24px + env(safe-area-inset-bottom));
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	.body {
		color: var(--soft);
		line-height: 1.45;
	}
	.actions {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 10px;
	}
	.btn.danger {
		background: var(--owe);
		color: var(--accent-ink);
	}
</style>
