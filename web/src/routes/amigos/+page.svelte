<script lang="ts">
	import { api, ApiError, type PublicUser, type Relation } from '$lib/api';
	import { money, untilText } from '$lib/format';
	import Avatar from '$lib/components/Avatar.svelte';
	import Icon from '$lib/components/Icon.svelte';

	type Found = PublicUser & { relation: Relation };
	interface Req {
		id: string;
		user: PublicUser;
		created_at: string;
	}

	let q = $state('');
	let results = $state<Found[]>([]);
	let searching = $state(false);
	let friends = $state<(PublicUser & { net_cents: number })[]>([]);
	let incoming = $state<Req[]>([]);
	let outgoing = $state<Req[]>([]);
	let blocks = $state<{ user: PublicUser; until: string }[]>([]);
	let error = $state('');

	async function load() {
		try {
			const [f, r] = await Promise.all([
				api.get<typeof friends>('/friends'),
				api.get<{ incoming: Req[]; outgoing: Req[]; blocks: typeof blocks }>('/friend-requests')
			]);
			friends = f;
			({ incoming, outgoing, blocks } = r);
		} catch (e) {
			error = (e as Error).message;
		}
	}
	load();

	$effect(() => {
		const term = q.trim();
		if (term.replace('@', '').length < 2) {
			results = [];
			return;
		}
		searching = true;
		const t = setTimeout(async () => {
			try {
				results = await api.get<Found[]>(`/users/search?q=${encodeURIComponent(term)}`);
			} catch (e) {
				error = (e as Error).message;
			} finally {
				searching = false;
			}
		}, 250);
		return () => clearTimeout(t);
	});

	async function add(u: Found) {
		try {
			const r = await api.post<{ relation: Relation }>('/friend-requests', { handle: u.handle });
			u.relation = r.relation;
			if (r.relation === 'friend') await load();
		} catch (e) {
			error = e instanceof ApiError ? e.message : 'Algo deu errado.';
		}
	}

	async function act(r: Req, action: 'accept' | 'decline' | 'block' | 'cancel') {
		try {
			await api.post(`/friend-requests/${r.id}/${action}`);
			await load();
		} catch (e) {
			error = (e as Error).message;
		}
	}

	async function unblock(userId: string) {
		await api.del(`/request-blocks/${userId}`).catch((e) => (error = e.message));
		await load();
	}

	const label: Record<Relation, string> = {
		none: 'Adicionar',
		request_sent: 'Pedido enviado',
		request_received: 'Aceitar',
		friend: 'Amigos',
		self: 'Você'
	};
</script>

<main class="page">
	<div class="top">
		<h1 class="title">Amigos</h1>
		<a class="qr-btn" href="/qr"><Icon name="qr" />Meu QR</a>
	</div>
	{#if error}<div class="error" role="alert">{error}</div>{/if}

	<section class="field">
		<label for="search">Buscar pelo @</label>
		<div class="search">
			<Icon name="search" size={18} />
			<input id="search" placeholder="@usuario" autocapitalize="none" autocomplete="off" bind:value={q} />
		</div>
		{#each results as u (u.id)}
			<div class="row">
				<Avatar user={u} />
				<div class="grow"><strong>{u.name}</strong><span class="muted small">@{u.handle}</span></div>
				<button
					class="chip"
					class:on={u.relation === 'none' || u.relation === 'request_received'}
					disabled={u.relation === 'request_sent' || u.relation === 'friend'}
					onclick={() => add(u)}
				>
					{label[u.relation]}
				</button>
			</div>
		{/each}
		{#if q.trim().length >= 3 && !searching && results.length === 0}
			<p class="muted">Ninguém encontrado com esse @.</p>
		{/if}
	</section>

	{#if incoming.length > 0}
		<section class="list">
			<h2 class="h2">Pedidos de amizade <span class="count">{incoming.length}</span></h2>
			{#each incoming as r (r.id)}
				<div class="card req">
					<div class="who">
						<Avatar user={r.user} />
						<div class="grow"><strong>{r.user.name}</strong><span class="muted small">@{r.user.handle}</span></div>
					</div>
					<div class="three">
						<button class="chip on" onclick={() => act(r, 'accept')}>Aceitar</button>
						<button class="chip" onclick={() => act(r, 'decline')}>Recusar</button>
						<button class="chip warn" onclick={() => act(r, 'block')}>Bloquear 24h</button>
					</div>
				</div>
			{/each}
		</section>
	{/if}

	{#each blocks as b (b.user.id)}
		<div class="blocked">
			<Icon name="block" size={18} />
			<span class="grow">@{b.user.handle} bloqueado até {untilText(b.until)}</span>
			<button onclick={() => unblock(b.user.id)}>Desfazer</button>
		</div>
	{/each}

	{#if outgoing.length > 0}
		<section class="list">
			<h2 class="h2">Pedidos enviados</h2>
			{#each outgoing as r (r.id)}
				<div class="row">
					<Avatar user={r.user} size={36} />
					<div class="grow"><strong>{r.user.name}</strong><span class="muted small">@{r.user.handle} · aguardando</span></div>
					<button class="chip" onclick={() => act(r, 'cancel')}>Cancelar</button>
				</div>
			{/each}
		</section>
	{/if}

	<section class="list">
		<div class="between"><h2 class="h2">Seus amigos</h2><span class="muted">{friends.length}</span></div>
		{#if friends.length === 0}
			<p class="muted">Busque seus amigos pelo @ ou mostre seu QR para eles.</p>
		{/if}
		{#each friends as f (f.id)}
			<a class="friend" href="/pagar/{f.id}">
				<Avatar user={f} />
				<div class="grow"><strong>{f.name}</strong><span class="muted small">@{f.handle}</span></div>
				{#if f.net_cents !== 0}
					<strong class:pos={f.net_cents > 0} class:neg={f.net_cents < 0}>{f.net_cents > 0 ? '+' : '−'}{money(Math.abs(f.net_cents))}</strong>
				{/if}
			</a>
		{/each}
	</section>
</main>

<style>
	.top {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}
	.qr-btn {
		height: 44px;
		padding: 0 14px;
		border-radius: 22px;
		background: var(--surface);
		color: var(--text);
		display: flex;
		align-items: center;
		gap: 8px;
		text-decoration: none;
		font-size: 14px;
		font-weight: 600;
	}
	.search {
		height: 50px;
		border-radius: 25px;
		background: var(--surface);
		border: 2px solid var(--line-2);
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 0 16px;
		color: var(--muted);
	}
	.search:focus-within {
		border-color: var(--accent);
	}
	.search input {
		border: none;
		background: transparent;
		color: var(--text);
		font-size: 16px;
		flex: 1;
		outline: none;
	}
	.list {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.count {
		background: var(--accent);
		color: var(--accent-ink);
		border-radius: 10px;
		padding: 1px 8px;
		font-size: 13px;
	}
	.req {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 12px 14px;
	}
	.who,
	.friend {
		display: flex;
		align-items: center;
		gap: 12px;
	}
	.grow {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.three {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 8px;
	}
	.three .chip {
		height: 40px;
		border-radius: 20px;
	}
	.chip.warn {
		color: var(--warn);
	}
	.blocked {
		background: var(--nav);
		border-radius: 16px;
		padding: 6px 14px;
		display: flex;
		align-items: center;
		gap: 10px;
		font-size: 13px;
		color: var(--muted);
	}
	.blocked button {
		border: none;
		background: none;
		color: var(--accent);
		font-weight: 600;
		padding: 10px 0;
	}
	.between {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
	}
	.friend {
		text-decoration: none;
		color: var(--text);
		padding: 6px 2px;
	}
</style>
