<script lang="ts">
	import { page } from '$app/state';
	import { api, type Payment } from '$lib/api';
	import { firstName, money, shortDate } from '$lib/format';
	import { session } from '$lib/session.svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import Header from '$lib/components/Header.svelte';
	import Icon from '$lib/components/Icon.svelte';

	let p = $state<Payment | null>(null);
	let error = $state('');
	let busy = $state(false);
	let zoom = $state(false);
	const me = session.me!;
	const id = $derived(page.params.id);

	async function load() {
		try {
			p = await api.get<Payment>(`/payments/${id}`);
		} catch (e) {
			error = (e as Error).message;
		}
	}
	load();

	const iAmPayee = $derived(p?.payee.id === me.id);
	const other = $derived(p ? (iAmPayee ? p.payer : p.payee) : null);
	const titles = $derived(p?.cotinhas.map((c) => c.title).join(', ') ?? '');

	async function decide(action: 'confirm' | 'reject') {
		if (action === 'reject' && !confirm('Marcar que esse Pix não caiu na sua conta?')) return;
		busy = true;
		try {
			p = await api.post<Payment>(`/payments/${id}/${action}`);
		} catch (e) {
			error = (e as Error).message;
		} finally {
			busy = false;
		}
	}
</script>

<main class="page no-nav">
	<Header title={iAmPayee && p?.status === 'pending' ? 'Confirmar pagamento' : 'Pagamento'} back={p?.cotinhas.length === 1 ? `/cotinha/${p.cotinhas[0].cotinha_id}` : '/'} />
	{#if error}<div class="error" role="alert">{error}</div>{/if}

	{#if p && other}
		{#if iAmPayee}
			<section class="who">
				<Avatar user={p.payer} size={56} />
				<div>
					<div class="h2" style="font-weight: 600">{firstName(p.payer.name)} {p.status === 'pending' ? 'diz que te pagou' : 'te pagou'}</div>
					<div class="big">{money(p.amount_cents)}</div>
					<div class="muted">{titles} · {p.method === 'compensation' ? 'compensação' : 'Pix'} · {shortDate(p.created_at)}</div>
				</div>
			</section>
		{:else}
			<section class="status">
				<div class="badge" class:ok={p.status === 'confirmed'} class:bad={p.status === 'rejected'}>
					<Icon name={p.status === 'confirmed' ? 'check' : p.status === 'rejected' ? 'close' : 'clock'} size={40} stroke={2.4} />
				</div>
				<h1 class="title center">
					{p.status === 'confirmed' ? `${firstName(other.name)} confirmou!` : p.status === 'rejected' ? 'Pagamento recusado' : `Avisamos ${firstName(other.name)}`}
				</h1>
				<p class="muted center lead">
					{#if p.status === 'confirmed'}Os {money(p.amount_cents)} foram confirmados.
					{:else if p.status === 'rejected'}{firstName(other.name)} disse que o Pix de {money(p.amount_cents)} não caiu. Confira e tente de novo.
					{:else}A dívida fecha assim que {firstName(other.name)} confirmar que os {money(p.amount_cents)} caíram na conta.{/if}
				</p>
			</section>

			<section class="card steps">
				<div class="step done"><i><Icon name="check" size={14} stroke={3.5} /></i><div><strong>Você pagou via Pix</strong><span class="muted">{shortDate(p.created_at)} · {money(p.amount_cents)}</span></div></div>
				<div class="step" class:done={!!p.receipt_url}><i>{#if p.receipt_url}<Icon name="check" size={14} stroke={3.5} />{/if}</i><div><strong>{p.receipt_url ? 'Comprovante anexado' : 'Sem comprovante'}</strong><span class="muted">{titles}</span></div></div>
				<div class="step" class:done={p.status === 'confirmed'} class:bad={p.status === 'rejected'}>
					<i>{#if p.status === 'confirmed'}<Icon name="check" size={14} stroke={3.5} />{/if}</i>
					<div><strong>{firstName(other.name)} confirma o recebimento</strong><span class="muted">{p.status === 'pending' ? 'Aguardando' : p.status === 'confirmed' ? 'Confirmado' : 'Recusado'}</span></div>
				</div>
			</section>
		{/if}

		{#if p.receipt_url}
			<button class="receipt" onclick={() => (zoom = true)} aria-label="Ampliar comprovante">
				<img src={p.receipt_url} alt="Comprovante" />
				<span class="muted">Toque para ampliar</span>
			</button>
		{:else if iAmPayee && p.method !== 'compensation'}
			<p class="card muted">{firstName(p.payer.name)} não anexou comprovante.</p>
		{/if}

		{#if iAmPayee && p.status === 'pending'}
			<p class="tip">Confira no app do seu banco se o Pix caiu antes de confirmar.</p>
			<div class="actions">
				<button class="btn" onclick={() => decide('confirm')} disabled={busy}><Icon name="check" size={18} stroke={3} />Recebi, confirmar</button>
				<button class="btn warn" onclick={() => decide('reject')} disabled={busy}>Não caiu na minha conta</button>
			</div>
		{:else if iAmPayee}
			<p class="card center">{p.status === 'confirmed' ? 'Você confirmou esse pagamento.' : 'Você recusou esse pagamento.'}</p>
		{:else}
			<a class="btn" href="/">Voltar ao início</a>
		{/if}
	{/if}
</main>

{#if zoom && p?.receipt_url}
	<button class="viewer" onclick={() => (zoom = false)} aria-label="Fechar"><img src={p.receipt_url} alt="Comprovante" /></button>
{/if}

<style>
	.who {
		display: flex;
		align-items: center;
		gap: 14px;
	}
	.big {
		font-size: 32px;
		font-weight: 800;
		color: var(--accent);
	}
	.status {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 10px;
		margin-top: 12px;
	}
	.badge {
		width: 84px;
		height: 84px;
		border-radius: 42px;
		border: 3px solid var(--accent);
		color: var(--accent);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.badge.ok {
		background: var(--accent);
		color: var(--accent-ink);
	}
	.badge.bad {
		border-color: var(--warn);
		color: var(--warn);
	}
	.center {
		text-align: center;
	}
	.lead {
		font-size: 15px;
		max-width: 320px;
		margin: 0;
	}
	.steps {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	.step {
		display: flex;
		gap: 14px;
		align-items: flex-start;
	}
	.step i {
		width: 26px;
		height: 26px;
		border-radius: 13px;
		border: 2px solid var(--line-3);
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--accent-ink);
		flex-shrink: 0;
	}
	.step.done i {
		background: var(--accent);
		border-color: var(--accent);
	}
	.step.bad i {
		border-color: var(--warn);
	}
	.step div {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.receipt {
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: 24px;
		padding: 14px;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 8px;
	}
	.receipt img {
		max-width: 100%;
		max-height: 320px;
		border-radius: 12px;
	}
	.tip {
		background: var(--surface-2);
		border-radius: 18px;
		padding: 12px 14px;
		margin: 0;
		color: var(--soft);
		font-size: 14px;
	}
	.actions {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.viewer {
		position: fixed;
		inset: 0;
		z-index: 50;
		background: rgba(0, 0, 0, 0.92);
		border: none;
		padding: 20px;
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.viewer img {
		max-width: 100%;
		max-height: 100%;
	}
</style>
