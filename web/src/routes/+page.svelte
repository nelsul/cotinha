<script lang="ts">
	import { api, type Balance, type CotinhaSummary, type Payment } from '$lib/api';
	import { firstName, money } from '$lib/format';
	import { session } from '$lib/session.svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import Icon from '$lib/components/Icon.svelte';

	let balances = $state<{ to_receive_cents: number; to_pay_cents: number; people: Balance[] } | null>(null);
	let toConfirm = $state<Payment[]>([]);
	let cotinhas = $state<CotinhaSummary[]>([]);
	let error = $state('');
	let unread = $state(0);

	async function load() {
		try {
			[balances, toConfirm, cotinhas] = await Promise.all([
				api.get<typeof balances>('/balances'),
				api.get<Payment[]>('/payments/to-confirm'),
				api.get<CotinhaSummary[]>('/cotinhas?status=open')
			]);
			unread = (await api.get<{ unread: number }>('/notifications/unread')).unread;
		} catch (e) {
			error = (e as Error).message;
		}
	}
	load();

	const net = $derived(balances ? balances.to_receive_cents - balances.to_pay_cents : 0);

	function subline(b: Balance): string {
		if (b.pending_in_cents > 0) return `Pagou ${money(b.pending_in_cents)}, falta você confirmar`;
		if (b.pending_out_cents > 0) return `Você pagou ${money(b.pending_out_cents)}, aguardando`;
		if (b.they_owe_cents > 0 && b.i_owe_cents > 0) return 'Vocês devem um ao outro: dá pra compensar';
		return '';
	}
</script>

<main class="page">
	<div class="top">
		<div class="logo">cotinha<span>.</span></div>
		{#if session.me}
			<div class="top-actions">
				<a href="/notificacoes" class="bell" aria-label={unread ? `Notificações, ${unread} novas` : 'Notificações'}>
					<Icon name="bell" size={22} />
					{#if unread}<span class="badge">{unread > 9 ? '9+' : unread}</span>{/if}
				</a>
				<a href="/perfil" aria-label="Seu perfil"><Avatar user={session.me} size={44} ring /></a>
			</div>
		{/if}
	</div>

	{#if error}<div class="error" role="alert">{error}</div>{/if}

	<section class="card balance" aria-label="Saldo geral">
		<div class="muted" style="font-size: 14px">Saldo geral</div>
		<div class="big" class:neg={net < 0}>{net >= 0 ? '+ ' : '− '}{money(Math.abs(net))}</div>
		<div class="split">
			<div><span class="muted">A receber</span><strong>{money(balances?.to_receive_cents ?? 0)}</strong></div>
			<div><span class="muted">A pagar</span><strong>{money(balances?.to_pay_cents ?? 0)}</strong></div>
		</div>
	</section>

	{#each toConfirm as p (p.id)}
		<a class="notice" href="/pagamento/{p.id}">
			<Avatar user={p.payer} />
			<div class="grow">
				<strong>{firstName(p.payer.name)} diz que pagou {money(p.amount_cents)}</strong>
				<span class="soft">{p.cotinhas.map((c) => c.title).join(', ')}{p.receipt_url ? ' · comprovante anexado' : ''}</span>
			</div>
			<span class="chip on">Conferir</span>
		</a>
	{/each}

	<div class="actions">
		<a class="btn" href="/nova"><Icon name="plus" stroke={2.5} size={18} />Nova cotinha</a>
		<a class="btn ghost" href="/amigos">Amigos</a>
	</div>

	<section class="list" aria-labelledby="saldos">
		<div class="head">
			<h2 class="h2" id="saldos">Saldos com amigos</h2>
		</div>
		{#if balances && balances.people.length === 0}
			<p class="muted empty">Tudo quitado por aqui. Crie uma cotinha para dividir algo com seus amigos.</p>
		{/if}
		{#each balances?.people ?? [] as b (b.user.id)}
			{@const n = b.they_owe_cents - b.i_owe_cents}
			<a class="row" href="/pagar/{b.user.id}">
				<Avatar user={b.user} />
				<div class="grow">
					<strong>{b.user.name}</strong>
					<span class="muted small">{subline(b) || '@' + b.user.handle}</span>
				</div>
				<div class="amount">
					<span class="muted small">{n >= 0 ? 'te deve' : 'você deve'}</span>
					<strong class:pos={n >= 0} class:neg={n < 0}>{money(Math.abs(n))}</strong>
				</div>
			</a>
		{/each}
	</section>

	<section class="list" aria-labelledby="abertas">
		<div class="head">
			<h2 class="h2" id="abertas">Cotinhas em aberto</h2>
			<a href="/cotinhas" class="more">Histórico</a>
		</div>
		{#if cotinhas.length === 0}
			<p class="muted empty">Nenhuma cotinha em aberto.</p>
		{/if}
			{#each cotinhas as c (c.id)}
				<a class="row" href="/cotinha/{c.id}">
					{#if c.photo_url}
						<img class="thumb" src={c.photo_url} alt="" />
					{:else}
						<span class="thumb icon"><Icon name="receipt" /></span>
					{/if}
					<div class="grow">
						<strong>{c.title}</strong>
						<span class="muted small">
							{c.role === 'creditor' ? `Você pagou ${money(c.total_cents)}` : `${firstName(c.creditor.name)} pagou`} · {c.people} pessoas
						</span>
					</div>
					<div class="amount">
						<span class="muted small">{c.role === 'creditor' ? 'falta receber' : 'falta pagar'}</span>
						<strong class:pos={c.role === 'creditor'} class:neg={c.role === 'debtor'}>{money(c.outstanding_cents)}</strong>
					</div>
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
	.top-actions {
		display: flex;
		align-items: center;
		gap: 14px;
	}
	.top-actions a {
		display: flex;
	}
	.bell {
		position: relative;
		width: 44px;
		height: 44px;
		border-radius: 22px;
		background: var(--surface);
		color: var(--text);
		align-items: center;
		justify-content: center;
	}
	.badge {
		position: absolute;
		top: -2px;
		right: -2px;
		min-width: 20px;
		height: 20px;
		padding: 0 5px;
		border-radius: 10px;
		background: var(--accent);
		color: var(--accent-ink);
		font-size: 12px;
		font-weight: 800;
		display: flex;
		align-items: center;
		justify-content: center;
		box-shadow: 0 0 0 3px var(--bg);
	}
	.logo {
		font-size: 28px;
		font-weight: 800;
		letter-spacing: -0.5px;
	}
	.logo span {
		color: var(--accent);
	}
	.balance {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.big {
		font-size: 42px;
		font-weight: 800;
		letter-spacing: -1px;
		color: var(--accent);
	}
	.big.neg {
		color: var(--owe);
	}
	.split {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 10px;
		margin-top: 10px;
	}
	.split div {
		background: var(--surface-2);
		border-radius: 16px;
		padding: 10px 14px;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.split strong {
		font-size: 18px;
		font-weight: 600;
	}
	.notice {
		background: var(--note-bg);
		border: 1px solid var(--note-line);
		border-radius: 20px;
		padding: 12px 14px;
		display: flex;
		align-items: center;
		gap: 12px;
		text-decoration: none;
		color: var(--text);
	}
	.notice .grow {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.soft {
		color: var(--soft);
		font-size: 13px;
	}
	.actions {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 10px;
	}
	.list {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.head {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
	}
	.more {
		font-size: 14px;
		font-weight: 600;
		min-height: 44px;
		display: inline-flex;
		align-items: center;
	}
	.amount {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 1px;
	}
	.empty {
		margin: 0;
		font-size: 14px;
	}
	.thumb {
		width: 40px;
		height: 40px;
		border-radius: 12px;
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
</style>
