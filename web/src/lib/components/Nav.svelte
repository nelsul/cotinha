<script lang="ts">
	import { page } from '$app/state';
	import Icon from './Icon.svelte';

	const items = [
		{ href: '/', label: 'Início', icon: 'home' },
		{ href: '/amigos', label: 'Amigos', icon: 'friends' },
		{ href: '/estatisticas', label: 'Estatísticas', icon: 'stats' },
		{ href: '/perfil', label: 'Perfil', icon: 'user' }
	];
</script>

<nav aria-label="Navegação principal">
	{#each items as item (item.href)}
		<a href={item.href} class:on={page.url.pathname === item.href} aria-current={page.url.pathname === item.href ? 'page' : undefined}>
			<Icon name={item.icon} size={22} />
			{item.label}
		</a>
	{/each}
</nav>

<style>
	nav {
		position: fixed;
		left: 0;
		right: 0;
		bottom: 0;
		z-index: 10;
		background: var(--nav);
		border-top: 1px solid var(--line);
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		padding-bottom: env(safe-area-inset-bottom);
	}
	a {
		height: 70px;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 4px;
		font-size: 12px;
		text-decoration: none;
		color: var(--muted);
	}
	a.on {
		color: var(--accent);
		font-weight: 600;
	}
</style>
