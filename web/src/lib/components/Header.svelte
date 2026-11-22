<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';

	let { title, back = '/', sub = '', right }: { title: string; back?: string; sub?: string; right?: Snippet } = $props();
</script>

<header>
	<a href={back} class="icon-btn" aria-label="Voltar"><Icon name="back" stroke={2.5} /></a>
	<div class="text">
		<h1>{title}</h1>
		{#if sub}<div class="muted">{sub}</div>{/if}
	</div>
	{#if right}{@render right()}{/if}
</header>

<style>
	header {
		display: flex;
		align-items: center;
		gap: 12px;
	}
	.text {
		flex: 1;
		min-width: 0;
	}
	h1 {
		font-size: 22px;
		font-weight: 800;
		margin: 0;
	}
</style>
