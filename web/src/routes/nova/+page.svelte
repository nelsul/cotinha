<script lang="ts">
	import { goto } from '$app/navigation';
	import { api, ApiError, type PublicUser } from '$lib/api';
	import { clearDraft, loadDraft, saveDraft } from '$lib/draft';
	import { money } from '$lib/format';
	import { appendImage, compressImage } from '$lib/image';
	import { session } from '$lib/session.svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import Header from '$lib/components/Header.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import MoneyInput from '$lib/components/MoneyInput.svelte';

	type Mode = 'equal' | 'amount' | 'weight';
	interface Person {
		user: PublicUser;
		on: boolean;
		amount: number; // cents
		weight: number;
	}
	interface Draft {
		title: string;
		total: number;
		description: string;
		mode: Mode;
		people: Record<string, { on: boolean; amount: number; weight: number }>;
		photos: Blob[];
		savedAt: number;
	}

	let title = $state('');
	let total = $state(0);
	let description = $state('');
	let mode = $state<Mode>('equal');
	let people = $state<Person[]>([]);
	let photos = $state<{ file: Blob; url: string }[]>([]);
	let preparing = $state(0); // photos being converted/shrunk right now
	let search = $state('');
	let busy = $state(false);
	let error = $state('');
	let loading = $state(true);
	let restored = $state(false);
	let ready = false; // drafts are only saved once the old one was loaded

	const me = session.me!;
	const draftKey = `nova:${me.id}`;
	const SEARCH_FROM = 6; // show the friend search once the list gets long

	async function load() {
		try {
			const friends = await api.get<PublicUser[]>('/friends');
			people = [me, ...friends].map((u) => ({ user: u, on: u.id === me.id, amount: 0, weight: 1 }));
			const d = await loadDraft<Draft>(draftKey);
			if (d && (d.title || d.total || d.description || d.photos?.length || Object.values(d.people).some((x) => x.on))) {
				title = d.title;
				total = d.total;
				description = d.description;
				mode = d.mode;
				for (const p of people) {
					const saved = d.people[p.user.id];
					if (saved) Object.assign(p, saved);
				}
				photos = (d.photos ?? []).map((file) => ({ file, url: URL.createObjectURL(file) }));
				restored = true;
			}
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
			ready = true;
		}
	}
	load();

	// Save the form as a draft whenever it changes (debounced).
	$effect(() => {
		const draft: Draft = {
			title,
			total,
			description,
			mode,
			people: Object.fromEntries(people.map((p) => [p.user.id, { on: p.on, amount: p.amount, weight: p.weight }])),
			photos: photos.map((p) => p.file),
			savedAt: Date.now()
		};
		if (!ready || busy) return;
		const t = setTimeout(() => saveDraft(draftKey, $state.snapshot(draft)), 400);
		return () => clearTimeout(t);
	});

	async function discardDraft() {
		await clearDraft(draftKey);
		title = '';
		total = 0;
		description = '';
		mode = 'equal';
		photos = [];
		for (const p of people) Object.assign(p, { on: p.user.id === me.id, amount: 0, weight: 1 });
		restored = false;
	}

	const chosen = $derived(people.filter((p) => p.on));
	const norm = (t: string) => t.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();
	// You first, then whoever is already in, then the rest; the search never hides someone already in.
	const visible = $derived.by(() => {
		const q = norm(search.trim().replace(/^@/, ''));
		const list = people.filter((p) => p.user.id === me.id || p.on || !q || norm(p.user.name).includes(q) || norm(p.user.handle).includes(q));
		const rank = (p: Person) => (p.user.id === me.id ? 0 : p.on ? 1 : 2);
		return list.sort((a, b) => rank(a) - rank(b) || a.user.name.localeCompare(b.user.name));
	});

	// Mirrors the API's split so the preview matches what will be saved.
	const preview = $derived.by(() => {
		const n = chosen.length;
		if (!n || !total) return chosen.map(() => 0);
		if (mode === 'equal') {
			const base = Math.floor(total / n);
			const rem = total % n;
			return chosen.map((_, i) => base + (i < rem ? 1 : 0));
		}
		if (mode === 'amount') return chosen.map((p) => p.amount);
		const wsum = chosen.reduce((s, p) => s + p.weight, 0);
		if (!wsum) return chosen.map(() => 0);
		const exact = chosen.map((p) => (total * p.weight) / wsum);
		const out = exact.map(Math.floor);
		let given = out.reduce((s, x) => s + x, 0);
		const order = exact.map((x, i) => ({ i, f: x - Math.floor(x) })).sort((a, b) => b.f - a.f);
		for (let k = 0; given < total; k++, given++) out[order[k % n].i]++;
		return out;
	});
	const previewSum = $derived(preview.reduce((s, x) => s + x, 0));
	const friendsOwe = $derived(chosen.reduce((s, p, i) => (p.user.id === me.id ? s : s + preview[i]), 0));
	const valid = $derived(
		title.trim() !== '' && total > 0 && chosen.some((p) => p.user.id !== me.id) && (mode !== 'amount' || previewSum === total)
	);

	// Photos are shrunk (and HEIC converted) as soon as they are picked, so the
	// previews work on every phone and the draft stays small.
	async function addPhotos(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const files = Array.from(input.files ?? []).slice(0, 6 - photos.length);
		input.value = '';
		preparing += files.length;
		for (const file of files) {
			try {
				const blob = await compressImage(file);
				if (photos.length < 6) photos.push({ file: blob, url: URL.createObjectURL(blob) });
			} catch {
				error = 'Não foi possível abrir uma das fotos.';
			} finally {
				preparing--;
			}
		}
	}

	async function create(e: SubmitEvent) {
		e.preventDefault();
		if (!valid) return;
		busy = true;
		error = '';
		try {
			const { id } = await api.post<{ id: string }>('/cotinhas', {
				title,
				description,
				total_cents: total,
				split_mode: mode,
				participants: chosen.map((p) => ({
					user_id: p.user.id,
					value: mode === 'amount' ? p.amount : mode === 'weight' ? p.weight : 0
				}))
			});
			await clearDraft(draftKey);
			for (const p of photos) {
				const form = new FormData();
				appendImage(form, p.file);
				await api.post(`/cotinhas/${id}/photos`, form).catch(() => {});
			}
			await goto(`/cotinha/${id}`, { replaceState: true });
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Algo deu errado.';
			busy = false;
		}
	}
</script>

<main class="page no-nav">
	<Header title="Nova cotinha" />

	{#if restored}
		<div class="draft">
			<span>Continuando o rascunho que você deixou.</span>
			<button type="button" onclick={discardDraft}>Descartar</button>
		</div>
	{/if}

	<form onsubmit={create} class="form">
		<section class="card main">
			<div class="field">
				<label for="title">Nome</label>
				<input id="title" class="input" maxlength="80" placeholder="Pizza de sexta" required bind:value={title} />
			</div>
			<div class="field">
				<label for="total">Valor total</label>
				<MoneyInput id="total" class="input total" required bind:cents={total} />
			</div>
			<p class="muted center">Pago por <span class="chip on">Você</span></p>
		</section>

		<div class="field">
			<label for="desc" class="h2" style="color: var(--text)">Descrição</label>
			<textarea id="desc" class="input" rows="3" maxlength="1000" placeholder="Onde foi, o que foi… para ninguém esquecer" bind:value={description}></textarea>
		</div>

		<section class="field">
			<div class="between">
				<span class="h2">Fotos</span>
				<span class="muted">{photos.length} de 6</span>
			</div>
			<div class="photos">
				{#each photos as p, i (p.url)}
					<div class="photo">
						<img src={p.url} alt="Foto {i + 1} da cotinha" />
						<button type="button" aria-label="Remover foto" onclick={() => photos.splice(i, 1)}><Icon name="close" size={14} stroke={3} /></button>
					</div>
				{/each}
				{#if preparing > 0}
					<div class="add-photo" aria-live="polite">Preparando…</div>
				{/if}
				{#if photos.length + preparing < 6}
					<label class="add-photo">
						<Icon name="camera" size={24} />
						Adicionar
						<input type="file" accept="image/*" multiple class="sr-only" onchange={addPhotos} />
					</label>
				{/if}
			</div>
		</section>

		<section class="field">
			<span class="h2">Como dividir?</span>
			<div class="segmented" style="grid-template-columns: repeat(3, minmax(0, 1fr))" role="radiogroup" aria-label="Modo de divisão">
				<button type="button" role="radio" aria-checked={mode === 'equal'} class:on={mode === 'equal'} onclick={() => (mode = 'equal')}>Igual</button>
				<button type="button" role="radio" aria-checked={mode === 'amount'} class:on={mode === 'amount'} onclick={() => (mode = 'amount')}>Valor</button>
				<button type="button" role="radio" aria-checked={mode === 'weight'} class:on={mode === 'weight'} onclick={() => (mode = 'weight')}>Peso</button>
			</div>
			<span class="muted">
				{mode === 'equal' ? 'Todo mundo paga a mesma coisa.' : mode === 'amount' ? 'Digite quanto cada um consumiu.' : 'Quem consumiu mais tem peso maior (ex.: número de cervejas).'}
			</span>
		</section>

		<section class="field">
			<div class="between">
				<span class="h2">Quem participou</span>
				<span class="muted">só seus amigos</span>
			</div>
			{#if loading}
				<p class="muted">Carregando amigos…</p>
			{:else if people.length === 1}
				<p class="muted">Você ainda não tem amigos no Cotinha. <a href="/amigos">Adicione alguém</a> para dividir.</p>
			{/if}
			{#if people.length > SEARCH_FROM}
				<div class="search">
					<Icon name="search" size={18} />
					<input placeholder="Buscar amigo pelo nome ou @" aria-label="Buscar amigo" autocapitalize="none" bind:value={search} />
					{#if search}<button type="button" aria-label="Limpar busca" onclick={() => (search = '')}><Icon name="close" size={16} /></button>{/if}
				</div>
			{/if}
			{#each visible as p (p.user.id)}
				{@const idx = chosen.indexOf(p)}
				<div class="row person" class:off={!p.on}>
					<div class="person-top">
						<button type="button" class="tick" class:on={p.on} role="checkbox" aria-checked={p.on} aria-label="Incluir {p.user.name}" onclick={() => (p.on = !p.on)}>
							{#if p.on}<Icon name="check" size={14} stroke={3.5} />{/if}
						</button>
						<Avatar user={p.user} size={40} />
						<div class="grow">
							<strong>{p.user.id === me.id ? 'Você' : p.user.name}</strong>
							<span class="muted small">@{p.user.handle}</span>
						</div>
						{#if p.on && mode !== 'amount'}
							<strong class="share">{money(preview[idx] ?? 0)}</strong>
						{/if}
					</div>
					{#if p.on && mode === 'amount'}
						<div class="person-control">
							<label class="muted" for="amount-{p.user.id}">Quanto consumiu</label>
							<MoneyInput id="amount-{p.user.id}" class="input mini" bind:cents={p.amount} />
						</div>
					{:else if p.on && mode === 'weight'}
						<div class="person-control">
							<span class="muted">Peso</span>
							<div class="stepper">
								<button type="button" aria-label="Diminuir peso de {p.user.name}" onclick={() => (p.weight = Math.max(0, p.weight - 1))}>−</button>
								<span>{p.weight}</span>
								<button type="button" aria-label="Aumentar peso de {p.user.name}" onclick={() => (p.weight += 1)}>+</button>
							</div>
						</div>
					{/if}
				</div>
			{/each}
			{#if search && visible.filter((p) => !p.on && p.user.id !== me.id).length === 0}
				<p class="muted">Nenhum amigo com “{search}”.</p>
			{/if}
			<a class="add-friend" href="/amigos"><Icon name="plus" size={18} stroke={2.4} />Adicionar amigo</a>
		</section>

		<div class="footer card">
			{#if mode === 'amount' && total > 0 && previewSum !== total}
				<div class="error">As partes somam {money(previewSum)}, mas o total é {money(total)}.</div>
			{/if}
			<div class="between">
				<span class="muted" style="font-size: 14px">Seus amigos vão te pagar</span>
				<strong class="pos" style="font-size: 24px">{money(friendsOwe)}</strong>
			</div>
			{#if error}<div class="error" role="alert">{error}</div>{/if}
			<button class="btn" disabled={!valid || busy || preparing > 0}>{busy ? 'Criando…' : 'Criar cotinha'}</button>
		</div>
	</form>
</main>

<style>
	.draft {
		background: var(--note-bg);
		border: 1px solid var(--note-line);
		border-radius: 16px;
		padding: 6px 6px 6px 14px;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		font-size: 14px;
		color: var(--soft);
	}
	.draft button {
		border: none;
		background: none;
		color: var(--accent);
		font-weight: 700;
		min-height: 44px;
		padding: 0 10px;
	}
	.search {
		height: 48px;
		border-radius: 24px;
		background: var(--surface);
		border: 1px solid var(--line-2);
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 0 6px 0 16px;
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
	.search button {
		width: 40px;
		height: 40px;
		border: none;
		background: none;
		color: var(--muted);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.form {
		display: flex;
		flex-direction: column;
		gap: 28px;
	}
	.main {
		display: flex;
		flex-direction: column;
		gap: 14px;
	}
	.main :global(input.total) {
		font-size: 34px;
		font-weight: 800;
		height: 64px;
		text-align: center;
	}
	.center {
		text-align: center;
		margin: 0;
	}
	.between {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
		gap: 8px;
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
	.photo img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		border-radius: 16px;
	}
	.photo button {
		position: absolute;
		top: 6px;
		right: 6px;
		width: 28px;
		height: 28px;
		border-radius: 14px;
		border: none;
		background: var(--bg);
		color: var(--text);
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
	.person {
		flex-direction: column;
		align-items: stretch;
		gap: 12px;
		padding: 14px 16px;
	}
	.person.off {
		opacity: 0.6;
	}
	.person-top {
		display: flex;
		align-items: center;
		gap: 12px;
	}
	.person-control {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding-left: 40px;
	}
	.tick {
		width: 28px;
		height: 28px;
		border-radius: 14px;
		border: 2px solid var(--line-3);
		background: transparent;
		color: var(--accent-ink);
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		padding: 0;
	}
	.tick.on {
		background: var(--accent);
		border-color: var(--accent);
	}
	.person :global(input.mini) {
		width: 128px;
		height: 44px;
		text-align: right;
	}
	.stepper {
		display: flex;
		align-items: center;
		border: 1px solid var(--line-3);
		border-radius: 14px;
	}
	.stepper button {
		width: 44px;
		height: 44px;
		border: none;
		background: transparent;
		font-size: 20px;
		font-weight: 700;
	}
	.stepper span {
		min-width: 24px;
		text-align: center;
		font-weight: 800;
	}
	.share {
		min-width: 76px;
		text-align: right;
	}
	.add-friend {
		height: 48px;
		border-radius: 18px;
		border: 2px dashed var(--line-3);
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		font-weight: 600;
		text-decoration: none;
	}
	.footer {
		display: flex;
		flex-direction: column;
		gap: 16px;
		margin-top: 8px;
	}
</style>
