<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	// Money typed like a bank app: digits fill in from the right, so the cents are
	// always there. Typing 5, 0, 0, 0 shows R$ 0,05 → 0,50 → 5,00 → 50,00.
	let {
		cents = $bindable(0),
		max = 100_000_000,
		...rest
	}: { cents: number; max?: number } & Omit<HTMLInputAttributes, 'value'> = $props();

	const fmt = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' });
	const text = $derived(cents > 0 ? fmt.format(cents / 100) : '');

	function oninput(e: Event) {
		const el = e.currentTarget as HTMLInputElement;
		const digits = el.value.replace(/\D/g, '').replace(/^0+/, '').slice(0, 12);
		cents = Math.min(max, Number(digits || '0'));
		el.value = cents > 0 ? fmt.format(cents / 100) : '';
		// Keep the caret at the end, where the next digit goes.
		el.setSelectionRange(el.value.length, el.value.length);
	}
</script>

<input {...rest} type="text" inputmode="numeric" autocomplete="off" placeholder={rest.placeholder ?? 'R$ 0,00'} value={text} {oninput} />
