<script lang="ts">
	import { api, type CotinhaSummary } from '$lib/api';
	import { firstName, money, shortDate } from '$lib/format';
	import Header from '$lib/components/Header.svelte';
	import Icon from '$lib/components/Icon.svelte';

	type Filter = 'all' | 'open' | 'settled';
	let all = $state<CotinhaSummary[] | null>(null);
	let filter = $state<Filter>('all');
	let search = $state('');
	let error = $state('');

	api
		.get<CotinhaSummary[]>('/cotinhas?status=all')
		.then((x) => (all = x))
		.catch((e) => (error = e.message));

	const norm = (t: string) => t.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
	const list = $derived(
		(all ?? []).filter(
			(c) =>
				(filter === 'all' || (filter === 'open' ? !c.settled : c.settled)) &&
				(!search.trim() || norm(c.title).includes(norm(search.trim())) || norm(c.creditor.name).includes(norm(search.trim())))
		)
	);
	const counts = $derived({
		all: all?.length ?? 0,
		open: all?.filter((c) => !c.settled).length ?? 0,
		settled: all?.filter((c) => c.settled).length ?? 0
	});

	function status(c: CotinhaSummary): string {
		if (c.settled) return 'Quitada';
		return c.role === 'creditor' ? `Falta receber ${money(c.outstanding_cents)}` : `Falta pagar ${money(c.outstanding_cents)}`;
	}
</script>

<main class="page no-nav">
	<Header title="Histórico de cotinhas" />
	{#if error}<div class="error" role="alert">{error}</div>{/if}

	<div class="segmented" style="grid-template-columns: repeat(3, minmax(0, 1fr))" role="radiogroup" aria-label="Filtrar cotinhas">
		<button role="radio" aria-checked={filter === 'all'} class:on={filter === 'all'} onclick={() => (filter = 'all')}>Todas · {counts.all}</button>
		<button role="radio" aria-checked={filter === 'open'} class:on={filter === 'open'} onclick={() => (filter = 'open')}>Abertas · {counts.open}</button>
		<button role="radio" aria-checked={filter === 'settled'} class:on={filter === 'settled'} onclick={() => (filter = 'settled')}>Quitadas · {counts.settled}</button>
	</div>

	<div class="search">
		<Icon name="search" size={18} />
		<input placeholder="Buscar pelo nome ou por quem pagou" aria-label="Buscar cotinha" bind:value={search} />
	</div>

	{#if all && list.length === 0}
		<p class="muted empty">{all.length === 0 ? 'Você ainda não participou de nenhuma cotinha.' : 'Nenhuma cotinha encontrada.'}</p>
	{/if}

	<ul class="list">
		{#each list as c (c.id)}
			<li>
				<a class="row" href="/cotinha/{c.id}">
					{#if c.photo_url}
						<img class="thumb" src={c.photo_url} alt="" />
					{:else}
						<span class="thumb icon"><Icon name="receipt" /></span>
					{/if}
					<div class="grow">
						<strong class="title-line">{c.title}</strong>
						<span class="muted small">
							{shortDate(c.created_at)} · {c.role === 'creditor' ? 'você pagou' : `${firstName(c.creditor.name)} pagou`} · {money(c.total_cents)}
						</span>
						<span class="tags">
							<span class="tag" class:done={c.settled}>{status(c)}</span>
							{#if c.hidden_from_stats}<span class="tag muted-tag">Fora das estatísticas</span>{/if}
						</span>
					</div>
				</a>
			</li>
		{/each}
	</ul>
</main>

<style>
	.segmented button {
		font-size: 13px;
	}
	.search {
		height: 48px;
		border-radius: 24px;
		background: var(--surface);
		border: 1px solid var(--line-2);
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
		flex: 1;
		min-width: 0;
		border: none;
		background: transparent;
		color: var(--text);
		font-size: 16px;
		outline: none;
	}
	.list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.title-line {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.thumb {
		width: 52px;
		height: 52px;
		border-radius: 14px;
		object-fit: cover;
		flex-shrink: 0;
	}
	.thumb.icon {
		background: var(--surface-2);
		color: var(--muted);
		display: inline-flex;
		align-items: center;
		justify-content: center;
	}
	.tags {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		margin-top: 4px;
	}
	.tag {
		font-size: 12px;
		font-weight: 600;
		border-radius: 10px;
		padding: 3px 8px;
		background: var(--surface-2);
		color: var(--warn);
	}
	.tag.done {
		color: var(--accent);
	}
	.tag.muted-tag {
		color: var(--muted);
	}
	.empty {
		margin: 0;
		font-size: 14px;
	}
</style>
