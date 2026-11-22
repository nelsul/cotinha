<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	// Money typed like a bank app: digits fill in from the right, so the cents are
	// always there. It always shows a value (R$ 0,00 when empty) and the caret
	// always sits at the right end, where the next digit goes.
	// Typing 5, 0, 6, 4 shows R$ 0,05 → 0,50 → 5,06 → 50,64.
	let {
		cents = $bindable(0),
		max = 100_000_000,
		class: cls = '',
		...rest
	}: { cents: number; max?: number } & Omit<HTMLInputAttributes, 'value'> = $props();

	const fmt = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' });
	const text = $derived(fmt.format(cents / 100));

	function toEnd(el: HTMLInputElement) {
		const end = el.value.length;
		if (el.selectionStart !== end || el.selectionEnd !== end) el.setSelectionRange(end, end);
	}

	function oninput(e: Event) {
		const el = e.currentTarget as HTMLInputElement;
		const digits = el.value.replace(/\D/g, '').replace(/^0+/, '').slice(0, 12);
		cents = Math.min(max, Number(digits || '0'));
		el.value = fmt.format(cents / 100);
		toEnd(el);
	}

	// Some phones place the caret where the finger tapped, after focusing; put it back.
	function onfocus(e: FocusEvent) {
		const el = e.currentTarget as HTMLInputElement;
		requestAnimationFrame(() => toEnd(el));
	}
	const keep = (e: Event) => toEnd(e.currentTarget as HTMLInputElement);
</script>

<input
	{...rest}
	class="{cls} money"
	class:zero={cents === 0}
	type="text"
	inputmode="numeric"
	autocomplete="off"
	value={text}
	{oninput}
	{onfocus}
	onclick={keep}
	onkeyup={keep}
	onselect={keep}
/>

<style>
	.money.zero {
		color: var(--muted);
	}
</style>
