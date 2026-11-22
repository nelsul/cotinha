<script lang="ts">
	import { api, type PublicUser } from '$lib/api';
	import { firstName, money } from '$lib/format';
	import Avatar from '$lib/components/Avatar.svelte';
	import Icon from '$lib/components/Icon.svelte';

	interface Row {
		user: PublicUser;
		cents: number;
	}
	interface Stats {
		sent_cents: number;
		received_cents: number;
		cotinhas_created: number;
		to_receive_cents: number;
		sent_to: Row[];
		owes_me: Row[];
		paid_me: Row[];
		fastest_payer: { user: PublicUser; days: number } | null;
		slowest_payer: { user: PublicUser; days: number } | null;
	}

	let period = $state<'month' | 'year' | 'all'>('month');
	let s = $state<Stats | null>(null);
	let error = $state('');

	$effect(() => {
		const p = period;
		api
			.get<Stats>(`/stats?period=${p}`)
			.then((x) => (s = x))
			.catch((e) => (error = e.message));
	});

	const monthName = new Intl.DateTimeFormat('pt-BR', { month: 'long' }).format(new Date());
	const days = (d: number) => (d < 1 ? 'menos de 1 dia' : `${Math.round(d)} ${Math.round(d) === 1 ? 'dia' : 'dias'}`);

	const sections = $derived(
		s
			? [
					{ title: 'Para quem você mais mandou dinheiro', sub: 'Pagamentos confirmados', rows: s.sent_to, color: 'var(--owe)' },
					{ title: 'Quem mais precisa te pagar', sub: 'Saldo em aberto agora', rows: s.owes_me, color: 'var(--accent)' },
					{ title: 'Quem mais te pagou', sub: 'Pagamentos confirmados', rows: s.paid_me, color: '#7DF9C2' }
				]
			: []
	);
</script>

<main class="page">
	<h1 class="title">Estatísticas</h1>
	<div class="segmented" style="grid-template-columns: repeat(3, minmax(0, 1fr))" role="radiogroup" aria-label="Período">
		<button role="radio" aria-checked={period === 'month'} class:on={period === 'month'} onclick={() => (period = 'month')} style="text-transform: capitalize">{monthName}</button>
		<button role="radio" aria-checked={period === 'year'} class:on={period === 'year'} onclick={() => (period = 'year')}>{new Date().getFullYear()}</button>
		<button role="radio" aria-checked={period === 'all'} class:on={period === 'all'} onclick={() => (period = 'all')}>Sempre</button>
	</div>
	{#if error}<div class="error" role="alert">{error}</div>{/if}

	{#if s}
		<div class="tiles">
			<div class="card tile"><span class="muted">Você pagou</span><strong>{money(s.sent_cents)}</strong></div>
			<div class="card tile"><span class="muted">Você recebeu</span><strong class="pos">{money(s.received_cents)}</strong></div>
			<div class="card tile"><span class="muted">Cotinhas criadas</span><strong>{s.cotinhas_created}</strong></div>
			<div class="card tile"><span class="muted">Ainda a receber</span><strong class="pos">{money(s.to_receive_cents)}</strong></div>
		</div>

		{#each sections as sec (sec.title)}
			<section class="card sec">
				<div>
					<h2 class="h2">{sec.title}</h2>
					<span class="muted small">{sec.sub}</span>
				</div>
				{#if sec.rows.length === 0}
					<span class="muted">Nada por aqui ainda.</span>
				{/if}
				{#each sec.rows as r (r.user.id)}
					<div class="bar-row">
						<Avatar user={r.user} size={32} />
						<div class="grow">
							<div class="line"><span>{r.user.name}</span><strong>{money(r.cents)}</strong></div>
							<div class="track"><div style="width: {(r.cents / sec.rows[0].cents) * 100}%; background: {sec.color}"></div></div>
						</div>
					</div>
				{/each}
			</section>
		{/each}

		{#if s.fastest_payer || s.slowest_payer}
			<div class="note">
				<Icon name="clock" size={26} />
				<div>
					{#if s.slowest_payer}
						<strong>{firstName(s.slowest_payer.user.name)} leva em média {days(s.slowest_payer.days)} para pagar</strong>
					{/if}
					{#if s.fastest_payer}
						<span>{firstName(s.fastest_payer.user.name)} é quem paga mais rápido: {days(s.fastest_payer.days)}</span>
					{/if}
				</div>
			</div>
		{/if}
	{/if}
</main>

<style>
	.tiles {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 10px;
	}
	.tile {
		padding: 14px;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.tile strong {
		font-size: 22px;
		font-weight: 800;
	}
	.sec {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.bar-row {
		display: flex;
		align-items: center;
		gap: 10px;
	}
	.grow {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.line {
		display: flex;
		justify-content: space-between;
		font-size: 14px;
	}
	.track {
		height: 8px;
		border-radius: 4px;
		background: #262e2a;
		overflow: hidden;
	}
	.track div {
		height: 100%;
		border-radius: 4px;
	}
	.note {
		background: var(--note-bg);
		border: 1px solid var(--note-line);
		border-radius: 20px;
		padding: 14px 16px;
		display: flex;
		gap: 12px;
		align-items: center;
		color: var(--accent);
	}
	.note div {
		display: flex;
		flex-direction: column;
		gap: 2px;
		color: var(--text);
	}
	.note span {
		color: var(--soft);
		font-size: 13px;
	}
</style>
