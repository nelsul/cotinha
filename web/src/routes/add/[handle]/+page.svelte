<script lang="ts">
	import { page } from '$app/state';
	import { api, ApiError, type PublicUser, type Relation } from '$lib/api';
	import Avatar from '$lib/components/Avatar.svelte';
	import Header from '$lib/components/Header.svelte';

	let user = $state<(PublicUser & { relation: Relation }) | null>(null);
	let error = $state('');
	let busy = $state(false);

	async function load() {
		try {
			user = await api.get(`/users/by-handle/${encodeURIComponent(page.params.handle ?? '')}`);
		} catch (e) {
			error = (e as Error).message;
		}
	}
	load();

	async function add() {
		if (!user) return;
		busy = true;
		try {
			const r = await api.post<{ relation: Relation }>('/friend-requests', { handle: user.handle });
			user.relation = r.relation;
		} catch (e) {
			error = e instanceof ApiError ? e.message : 'Algo deu errado.';
		} finally {
			busy = false;
		}
	}

	const text: Record<Relation, string> = {
		self: 'Esse é o seu código. Mostre para um amigo ler.',
		friend: 'Vocês já são amigos no Cotinha.',
		request_sent: 'Pedido enviado! Assim que aceitarem, vocês podem dividir cotinhas.',
		request_received: 'Essa pessoa já te mandou um pedido. Toque para aceitar.',
		none: 'Mande um pedido de amizade para dividir cotinhas.'
	};
</script>

<main class="page no-nav">
	<Header title="Adicionar amigo" back="/amigos" />
	{#if error}<div class="error" role="alert">{error}</div>{/if}
	{#if user}
		<section class="card who">
			<Avatar user={user} size={96} />
			<strong class="name">{user.name}</strong>
			<span class="muted">@{user.handle}</span>
			<p>{text[user.relation]}</p>
			{#if user.relation === 'none' || user.relation === 'request_received'}
				<button class="btn" onclick={add} disabled={busy}>{user.relation === 'none' ? 'Adicionar' : 'Aceitar pedido'}</button>
			{:else}
				<a class="btn ghost" href="/amigos">Ver amigos</a>
			{/if}
		</section>
	{/if}
</main>

<style>
	.who {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 8px;
		text-align: center;
		margin-top: 24px;
	}
	.name {
		font-size: 24px;
		font-weight: 800;
		margin-top: 8px;
	}
	.who p {
		color: var(--soft);
		margin: 8px 0;
	}
</style>
