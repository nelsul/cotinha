<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import QRCode from 'qrcode';
	import { api, ApiError, type PublicUser } from '$lib/api';
	import { appendImage, compressImage } from '$lib/image';
	import { centsToInput, firstName, money, parseMoney } from '$lib/format';
	import Avatar from '$lib/components/Avatar.svelte';
	import Header from '$lib/components/Header.svelte';
	import Icon from '$lib/components/Icon.svelte';

	interface ShareOut {
		cotinha_id: string;
		title: string;
		amount_cents: number;
		remaining_cents: number;
		payable_cents: number;
		pending_cents: number;
	}
	interface With {
		user: PublicUser;
		i_owe_cents: number;
		they_owe_cents: number;
		i_can_pay_cents: number;
		can_compensate_cents: number;
		i_owe: ShareOut[];
		they_owe: ShareOut[];
	}

	const id = $derived(page.params.id);
	const cotinhaId = $derived(page.url.searchParams.get('cotinha') ?? '');

	let w = $state<With | null>(null);
	let error = $state('');
	let mode = $state<'all' | 'part'>('all');
	let partText = $state('');
	let pix = $state<{ payload: string; pix_key: string; name: string } | null>(null);
	let pixError = $state('');
	let qr = $state('');
	let copied = $state(false);
	let receipt = $state<{ file: File; url: string } | null>(null);
	let busy = $state(false);
	let compensating = $state(false);

	async function load() {
		try {
			w = await api.get<With>(`/balances/${id}`);
		} catch (e) {
			error = (e as Error).message;
		}
	}
	load();

	const payable = $derived.by(() => {
		if (!w) return 0;
		if (cotinhaId) return w.i_owe.find((s) => s.cotinha_id === cotinhaId)?.payable_cents ?? 0;
		return w.i_can_pay_cents;
	});
	const amount = $derived(mode === 'all' ? payable : Math.min(parseMoney(partText) ?? 0, payable));
	const scope = $derived(cotinhaId ? w?.i_owe.find((s) => s.cotinha_id === cotinhaId)?.title : '');

	// Refresh the Pix code whenever the amount changes.
	$effect(() => {
		const cents = amount;
		if (!cents) {
			pix = null;
			qr = '';
			return;
		}
		const t = setTimeout(async () => {
			try {
				pix = await api.get(`/users/${id}/pix?amount_cents=${cents}`);
				pixError = '';
				qr = await QRCode.toDataURL(pix!.payload, { margin: 1, width: 360, color: { dark: '#0B0F0D', light: '#F1F5F2' } });
			} catch (e) {
				pix = null;
				qr = '';
				pixError = (e as Error).message;
			}
		}, 250);
		return () => clearTimeout(t);
	});

	async function copy() {
		if (!pix) return;
		await navigator.clipboard.writeText(pix.payload);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}

	function pickReceipt(e: Event) {
		const file = (e.currentTarget as HTMLInputElement).files?.[0];
		if (file) receipt = { file, url: URL.createObjectURL(file) };
	}

	async function compensate() {
		compensating = true;
		try {
			await api.post(`/balances/${id}/compensate`);
			await load();
		} catch (e) {
			error = (e as Error).message;
		} finally {
			compensating = false;
		}
	}

	async function markPaid() {
		if (!amount) return;
		busy = true;
		error = '';
		try {
			const form = new FormData();
			form.append('payee_id', id!);
			form.append('amount_cents', String(amount));
			if (cotinhaId) form.append('cotinha_id', cotinhaId);
			if (receipt) appendImage(form, await compressImage(receipt.file), 'comprovante');
			const res = await api.post<{ id: string }>('/payments', form);
			await goto(`/pagamento/${res.id}`, { replaceState: true });
		} catch (e) {
			error = e instanceof ApiError ? e.message : 'Algo deu errado.';
			busy = false;
		}
	}
</script>

<main class="page no-nav">
	<Header title={w ? `Acertar com ${firstName(w.user.name)}` : 'Acertar'} sub={w ? `@${w.user.handle}${scope ? ' · ' + scope : ''}` : ''} />
	{#if error}<div class="error" role="alert">{error}</div>{/if}

	{#if w}
		<section class="card who">
			<Avatar user={w.user} size={52} />
			<div class="grow">
				{#if w.i_owe_cents > 0 && w.they_owe_cents > 0}
					<span class="muted">Você deve {money(w.i_owe_cents)} · {firstName(w.user.name)} te deve {money(w.they_owe_cents)}</span>
				{:else if w.i_owe_cents > 0}
					<span class="muted">Você deve</span>
				{:else if w.they_owe_cents > 0}
					<span class="muted">{firstName(w.user.name)} te deve</span>
				{:else}
					<span class="muted">Vocês estão quites</span>
				{/if}
				<strong class="big" class:neg={w.i_owe_cents > w.they_owe_cents}>{money(Math.abs(w.i_owe_cents - w.they_owe_cents))}</strong>
			</div>
		</section>

		{#if w.can_compensate_cents > 0 && !cotinhaId}
			<section class="card comp">
				<div class="label"><Icon name="swap" size={18} stroke={2.2} /> COMPENSAR DÍVIDAS</div>
				<p>
					Vocês devem um ao outro. Compensando, {money(w.can_compensate_cents)} somem dos dois lados e
					{w.i_owe_cents > w.they_owe_cents ? `você paga só ${money(w.i_owe_cents - w.they_owe_cents)}` : `${firstName(w.user.name)} paga só ${money(w.they_owe_cents - w.i_owe_cents)}`}.
				</p>
				<button class="btn" onclick={compensate} disabled={compensating}>{compensating ? 'Compensando…' : `Compensar ${money(w.can_compensate_cents)}`}</button>
			</section>
		{/if}

		{#if payable > 0}
			<section class="field">
				<span class="muted" style="font-size: 14px">Quanto você vai pagar?</span>
				<div class="amounts">
					<button class="amount" class:on={mode === 'all'} onclick={() => (mode = 'all')}>
						<strong>{money(payable)}</strong><span class="muted small">Tudo</span>
					</button>
					<button class="amount" class:on={mode === 'part'} onclick={() => ((mode = 'part'), (partText ||= centsToInput(Math.round(payable / 2))))}>
						<strong>Pagar parte</strong><span class="muted small">escolha o valor</span>
					</button>
				</div>
				{#if mode === 'part'}
					<input class="input" inputmode="decimal" aria-label="Valor a pagar" bind:value={partText} />
					{#if (parseMoney(partText) ?? 0) > payable}<span class="muted">O máximo é {money(payable)}.</span>{/if}
				{/if}
			</section>

			<div class="methods">
				<span class="chip on">Pix</span>
				<span class="chip soon">Débito · em breve</span>
				<span class="chip soon">Crédito · em breve</span>
				<span class="chip soon">Apple Pay · em breve</span>
			</div>

			<section class="card pix">
				{#if pixError}
					<p class="muted">{pixError}</p>
				{:else}
					{#if qr}<img src={qr} alt="QR code Pix para {w.user.name}" class="qr" />{:else}<div class="qr ph"></div>{/if}
					<div class="pix-info">
						<span class="muted">Chave Pix de {firstName(w.user.name)}</span>
						<strong>{pix?.pix_key ?? '…'}</strong>
						<span class="muted">Valor no código: <strong style="color: var(--text)">{money(amount)}</strong></span>
						<button class="btn ghost slim" onclick={copy} disabled={!pix}><Icon name="copy" size={16} />{copied ? 'Copiado!' : 'Copiar Pix copia e cola'}</button>
					</div>
				{/if}
			</section>

			<section class="field">
				<span class="h2">Comprovante</span>
				{#if receipt}
					<div class="row">
						<img src={receipt.url} alt="Comprovante" class="rthumb" />
						<div class="grow"><strong>{receipt.file.name}</strong><span class="muted small">Anexado</span></div>
						<button class="chip" onclick={() => (receipt = null)}>Trocar</button>
					</div>
				{:else}
					<label class="attach"><Icon name="clip" size={22} />Anexar print do comprovante<input type="file" accept="image/*" class="sr-only" onchange={pickReceipt} /></label>
				{/if}
			</section>

			<div class="card footer">
				<div class="between"><span class="muted">Pix direto, sem taxa</span><strong class="pos" style="font-size: 24px">{money(amount)}</strong></div>
				<button class="btn" onclick={markPaid} disabled={busy || !amount}>{busy ? 'Enviando…' : `Já paguei, avisar ${firstName(w.user.name)}`}</button>
				<span class="muted center">A dívida só fecha quando {firstName(w.user.name)} confirmar que recebeu.</span>
			</div>
		{:else if w.i_owe_cents > 0}
			<p class="card muted">Você já marcou tudo como pago. Falta {firstName(w.user.name)} confirmar.</p>
		{/if}

		{#if w.they_owe.length > 0}
			<section class="list">
				<h2 class="h2">{firstName(w.user.name)} te deve</h2>
				{#each w.they_owe as s (s.cotinha_id)}
					<a class="row" href="/cotinha/{s.cotinha_id}">
						<div class="grow"><strong>{s.title}</strong><span class="muted small">{s.pending_cents > 0 ? `${money(s.pending_cents)} aguardando você confirmar` : 'em aberto'}</span></div>
						<strong class="pos">{money(s.remaining_cents)}</strong>
					</a>
				{/each}
			</section>
		{/if}
		{#if w.i_owe.length > 0 && !cotinhaId}
			<section class="list">
				<h2 class="h2">Você deve</h2>
				{#each w.i_owe as s (s.cotinha_id)}
					<a class="row" href="/cotinha/{s.cotinha_id}">
						<div class="grow"><strong>{s.title}</strong><span class="muted small">{s.pending_cents > 0 ? `${money(s.pending_cents)} aguardando confirmação` : 'em aberto'}</span></div>
						<strong class="neg">{money(s.remaining_cents)}</strong>
					</a>
				{/each}
			</section>
		{/if}
	{/if}
</main>

<style>
	.who {
		display: flex;
		align-items: center;
		gap: 14px;
	}
	.who .grow {
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 0;
	}
	.big {
		font-size: 30px;
		font-weight: 800;
		color: var(--accent);
	}
	.big.neg {
		color: var(--owe);
	}
	.comp {
		display: flex;
		flex-direction: column;
		gap: 12px;
		border: 1.5px solid var(--line-3);
		background: transparent;
	}
	.comp .label {
		display: flex;
		align-items: center;
		gap: 8px;
		color: var(--accent);
		font-size: 13px;
		font-weight: 700;
		letter-spacing: 1px;
	}
	.comp p {
		margin: 0;
		color: var(--soft);
		line-height: 1.45;
	}
	.amounts {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 8px;
	}
	.amount {
		height: 64px;
		border-radius: 18px;
		border: 2px solid var(--surface-2);
		background: var(--surface);
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 2px;
	}
	.amount strong {
		font-size: 18px;
	}
	.amount.on {
		border-color: var(--accent);
	}
	.methods {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
	.soon {
		color: #7f8c85;
		border-color: var(--line-2);
		font-weight: 400;
	}
	.pix {
		display: flex;
		gap: 14px;
		align-items: center;
	}
	.qr {
		width: 132px;
		height: 132px;
		border-radius: 12px;
		flex-shrink: 0;
	}
	.qr.ph {
		background: var(--surface-2);
	}
	.pix-info {
		display: flex;
		flex-direction: column;
		gap: 6px;
		min-width: 0;
	}
	.slim {
		height: 42px;
		font-size: 14px;
		padding: 0 12px;
	}
	.attach {
		height: 80px;
		border-radius: 20px;
		border: 2px dashed var(--line-3);
		color: var(--accent);
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 10px;
		font-weight: 600;
		cursor: pointer;
	}
	.rthumb {
		width: 48px;
		height: 60px;
		object-fit: cover;
		border-radius: 10px;
	}
	.footer {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.between {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
	}
	.center {
		text-align: center;
	}
	.list {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
</style>
