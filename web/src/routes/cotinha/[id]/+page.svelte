<script lang="ts">
	import { page } from '$app/state';
	import { api, ApiError, type PublicUser } from '$lib/api';
	import { firstName, money, shortDate } from '$lib/format';
	import { appendImage, compressImage } from '$lib/image';
	import { session } from '$lib/session.svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import Header from '$lib/components/Header.svelte';
	import Icon from '$lib/components/Icon.svelte';

	interface Detail {
		cotinha: { id: string; title: string; description: string; total_cents: number; split_mode: string; created_at: string };
		creditor: PublicUser;
		shares: {
			user: PublicUser;
			amount_cents: number;
			confirmed_cents: number;
			pending_cents: number;
			remaining_cents: number;
			is_creditor: boolean;
		}[];
		expected_cents: number;
		received_cents: number;
		pending_cents: number;
		settled: boolean;
		photos: { id: string; url: string; uploaded_by: string }[];
		history: { payment_id: string; payer: PublicUser; amount_cents: number; method: string; status: string; created_at: string }[];
	}

	let d = $state<Detail | null>(null);
	let error = $state('');
	let viewing = $state<string | null>(null);
	let uploading = $state(false);
	const me = session.me!;
	const id = $derived(page.params.id);

	async function load() {
		try {
			d = await api.get<Detail>(`/cotinhas/${id}`);
		} catch (e) {
			error = (e as Error).message;
		}
	}
	load();

	const isCreditor = $derived(d?.creditor.id === me.id);
	const myShare = $derived(d?.shares.find((s) => s.user.id === me.id && !s.is_creditor));
	const pct = (x: number) => (d && d.expected_cents ? `${(x / d.expected_cents) * 100}%` : '0%');
	const modeText: Record<string, string> = { equal: 'igual', amount: 'por valor', weight: 'por peso' };

	async function addPhoto(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file) return;
		uploading = true;
		try {
			const form = new FormData();
			appendImage(form, await compressImage(file));
			await api.post(`/cotinhas/${id}/photos`, form);
			await load();
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Não foi possível enviar a foto.';
		} finally {
			uploading = false;
		}
	}

	async function removePhoto(photoId: string) {
		if (!confirm('Remover esta foto?')) return;
		await api.del(`/cotinhas/${id}/photos/${photoId}`).catch((e) => (error = e.message));
		await load();
	}

	function shareStatus(s: Detail['shares'][number]) {
		if (s.remaining_cents === 0) return { text: 'Pago', cls: 'done' };
		if (s.pending_cents > 0) return { text: isCreditor ? 'Conferir' : 'Aguardando', cls: isCreditor ? 'on' : '' };
		return { text: 'Em aberto', cls: '' };
	}

	function historyText(h: Detail['history'][number]) {
		const who = h.payer.id === me.id ? 'Você' : firstName(h.payer.name);
		if (h.method === 'compensation') return `${who} compensou ${money(h.amount_cents)} com outra dívida`;
		if (h.status === 'pending') return `${who} marcou ${money(h.amount_cents)} como pago`;
		if (h.status === 'rejected') return `Pagamento de ${money(h.amount_cents)} de ${who} foi recusado`;
		return `Pagamento de ${money(h.amount_cents)} de ${who} confirmado`;
	}
</script>

<main class="page no-nav">
	<Header title="" back="/" />
	{#if error}<div class="error" role="alert">{error}</div>{/if}

	{#if d}
		<section class="intro">
			<div class="muted">{shortDate(d.cotinha.created_at)} · {isCreditor ? 'criada por você' : `${firstName(d.creditor.name)} pagou`}</div>
			<h1 class="title">{d.cotinha.title}</h1>
			{#if d.cotinha.description}<p class="desc">{d.cotinha.description}</p>{/if}
		</section>

		<section class="photos" aria-label="Fotos">
			{#each d.photos as p (p.id)}
				<div class="photo">
					<button class="open" onclick={() => (viewing = p.url)} aria-label="Ampliar foto"><img src={p.url} alt="" /></button>
					{#if p.uploaded_by === me.id || isCreditor}
						<button class="remove" aria-label="Remover foto" onclick={() => removePhoto(p.id)}><Icon name="close" size={14} stroke={3} /></button>
					{/if}
				</div>
			{/each}
			{#if d.photos.length < 6}
				<label class="add-photo">
					<Icon name="camera" size={22} />
					{uploading ? 'Enviando…' : 'Foto'}
					<input type="file" accept="image/*" class="sr-only" onchange={addPhoto} disabled={uploading} />
				</label>
			{/if}
		</section>

		<section class="card progress">
			<div class="between muted">
				<span>{isCreditor ? 'Recebido' : 'Total'}</span>
				<span>{money(d.cotinha.total_cents)} · {modeText[d.cotinha.split_mode]}</span>
			</div>
			<div class="amount">
				<span class="pos">{money(d.received_cents)}</span>
				<span class="of">de {money(d.expected_cents)}</span>
			</div>
			<div class="bar" aria-hidden="true">
				<div class="ok" style="width: {pct(d.received_cents)}"></div>
				<div class="wait" style="width: {pct(d.pending_cents)}"></div>
			</div>
			<div class="legend">
				<span><i class="dot ok"></i>Confirmado</span>
				<span><i class="dot wait"></i>Aguardando confirmação</span>
			</div>
			{#if d.settled}<p class="done-note"><Icon name="check" size={16} stroke={3} /> Cotinha quitada!</p>{/if}
		</section>

		{#if myShare && myShare.remaining_cents > 0}
			<a class="btn" href="/pagar/{d.creditor.id}?cotinha={d.cotinha.id}">
				Pagar minha parte · {money(myShare.remaining_cents - myShare.pending_cents)}
			</a>
		{/if}

		<section class="list">
			<h2 class="h2">Quem participou</h2>
			{#each d.shares as s (s.user.id)}
				{@const st = shareStatus(s)}
				<div class="row">
					<Avatar user={s.user} />
					<div class="grow">
						<strong>{s.user.id === me.id ? 'Você' : s.user.name} · {money(s.amount_cents)}</strong>
						<span class="muted small">
							{#if s.is_creditor}Pagou a conta
							{:else if s.remaining_cents === 0}Quitado
							{:else if s.pending_cents > 0}Pagou {money(s.pending_cents)}, falta confirmar
							{:else if s.confirmed_cents > 0}Pagou {money(s.confirmed_cents)}, falta {money(s.remaining_cents)}
							{:else}Ainda não pagou{/if}
						</span>
					</div>
					{#if !s.is_creditor}
						{#if st.cls === 'on'}
							<a class="chip on" href="/pagamento/{d.history.find((h) => h.payer.id === s.user.id && h.status === 'pending')?.payment_id}">{st.text}</a>
						{:else}
							<span class="chip" class:done={st.cls === 'done'}>{st.text}</span>
						{/if}
					{/if}
				</div>
			{/each}
		</section>

		{#if d.history.length > 0}
			<section class="list">
				<h2 class="h2">Histórico</h2>
				{#each d.history as h (h.payment_id)}
					<a class="log" href="/pagamento/{h.payment_id}">
						<span class="muted when">{shortDate(h.created_at)}</span>
						<span>{historyText(h)}</span>
					</a>
				{/each}
			</section>
		{/if}
	{/if}
</main>

{#if viewing}
	<button class="viewer" onclick={() => (viewing = null)} aria-label="Fechar foto"><img src={viewing} alt="" /></button>
{/if}

<style>
	.intro {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.desc {
		margin: 0;
		color: var(--soft);
		font-size: 15px;
		line-height: 1.45;
		white-space: pre-line;
	}
	.photos {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 8px;
	}
	.photo {
		position: relative;
		aspect-ratio: 1;
	}
	.photo .open {
		padding: 0;
		border: none;
		background: none;
		width: 100%;
		height: 100%;
	}
	.photo img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		border-radius: 16px;
		display: block;
	}
	.photo .remove {
		position: absolute;
		top: 6px;
		right: 6px;
		width: 28px;
		height: 28px;
		border-radius: 14px;
		border: none;
		background: var(--bg);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.add-photo {
		aspect-ratio: 1;
		border-radius: 16px;
		border: 2px dashed var(--line-3);
		color: var(--accent);
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 6px;
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
	}
	.progress {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.between {
		display: flex;
		justify-content: space-between;
		gap: 8px;
		font-size: 14px;
	}
	.amount .pos {
		font-size: 28px;
		font-weight: 800;
	}
	.of {
		color: var(--muted);
		font-weight: 600;
	}
	.bar {
		height: 12px;
		border-radius: 6px;
		background: #262e2a;
		display: flex;
		overflow: hidden;
	}
	.bar .ok {
		background: var(--accent);
	}
	.bar .wait {
		background: repeating-linear-gradient(45deg, #6b8a1e 0 6px, #4d6b12 6px 12px);
	}
	.legend {
		display: flex;
		gap: 14px;
		flex-wrap: wrap;
		font-size: 12px;
		color: var(--muted);
	}
	.legend span {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.dot {
		width: 10px;
		height: 10px;
		border-radius: 5px;
	}
	.dot.ok {
		background: var(--accent);
	}
	.dot.wait {
		background: #6b8a1e;
	}
	.done-note {
		margin: 0;
		color: var(--accent);
		font-weight: 600;
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.list {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.chip.done {
		color: var(--accent);
		border-color: var(--line);
		background: var(--line);
	}
	.log {
		display: flex;
		gap: 10px;
		font-size: 14px;
		color: var(--soft);
		text-decoration: none;
		padding: 4px 0;
	}
	.when {
		width: 84px;
		flex-shrink: 0;
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
		border-radius: 12px;
	}
</style>
