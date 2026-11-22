<script lang="ts">
	import type { PublicUser } from '$lib/api';

	let { user, size = 40, ring = false }: { user: Pick<PublicUser, 'id' | 'name' | 'avatar_url'>; size?: number; ring?: boolean } =
		$props();

	const palette = ['#F9A8D4', '#93C5FD', '#FCD34D', '#C4B5FD', '#86EFAC', '#FDBA74', '#A5F3FC', '#FDE68A'];
	const color = $derived(palette[[...user.id].reduce((n, c) => n + c.charCodeAt(0), 0) % palette.length]);
	const initial = $derived((user.name || '?').trim().charAt(0).toUpperCase());
</script>

{#if user.avatar_url}
	<img
		src={user.avatar_url}
		alt={user.name}
		class:ring
		style="width: {size}px; height: {size}px"
	/>
{:else}
	<span
		class="initial"
		class:ring
		role="img"
		aria-label={user.name}
		style="width: {size}px; height: {size}px; background: {color}; font-size: {Math.round(size * 0.42)}px"
	>
		{initial}
	</span>
{/if}

<style>
	img,
	.initial {
		border-radius: 50%;
		flex-shrink: 0;
		object-fit: cover;
	}
	.initial {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		color: #0b0f0d;
		font-weight: 800;
	}
	.ring {
		box-shadow:
			0 0 0 3px var(--bg),
			0 0 0 5px var(--accent);
	}
</style>
