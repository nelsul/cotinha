<script lang="ts">
	import '../app.css';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { loadMe, session } from '$lib/session.svelte';
	import Nav from '$lib/components/Nav.svelte';

	let { children } = $props();

	const publicPaths = ['/login'];
	const navPaths = ['/', '/amigos', '/estatisticas', '/perfil'];

	let ready = $state(false);

	$effect(() => {
		const path = page.url.pathname;
		(async () => {
			if (!session.loaded) await loadMe();
			const me = session.me;
			if (!me && !publicPaths.includes(path)) {
				const next = path === '/' ? '' : `?next=${encodeURIComponent(path + page.url.search)}`;
				await goto('/login' + next, { replaceState: true });
			} else if (me && !me.onboarded && path !== '/onboarding') {
				const next = path === '/' ? '' : `?next=${encodeURIComponent(path + page.url.search)}`;
				await goto('/onboarding' + next, { replaceState: true });
			} else if (me && me.onboarded && (path === '/login' || path === '/onboarding')) {
				await goto('/', { replaceState: true });
			}
			ready = true;
		})();
	});
</script>

{#if ready}
	{@render children()}
	{#if session.me?.onboarded && navPaths.includes(page.url.pathname)}
		<Nav />
	{/if}
{/if}
