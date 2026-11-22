<script lang="ts">
	import { goto } from '$app/navigation';
	import { api, ApiError, type PublicUser } from '$lib/api';
	import { money, parseMoney } from '$lib/format';
	import { appendImage, compressImage } from '$lib/image';
	import { session } from '$lib/session.svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import Header from '$lib/components/Header.svelte';
	import Icon from '$lib/components/Icon.svelte';

	type Mode = 'equal' | 'amount' | 'weight';
	interface Person {
		user: PublicUser;
		on: boolean;
		amount: string;
		weight: number;
	}

	let title = $state('');
	let totalText = $state('');
	let description = $state('');
	let mode = $state<Mode>('equal');
	let people = $state<Person[]>([]);
	let photos = $state<{ file: File; url: string }[]>([]);
	let busy = $state(false);
	let error = $state('');
	let loading = $state(true);

	const me = session.me!;

	async function load() {
		try {
			const friends = await api.get<PublicUser[]>('/friends');
			people = [me, ...friends].map((u) => ({ user: u, on: u.id === me.id, amount: '', weight: 1 }));
		} catch (e) {
			error = (e as Error).message;
		} finally {
			loading = false;
		}
	}
	load();

	const total = $derived(parseMoney(totalText) ?? 0);
	const chosen = $derived(people.filter((p) => p.on));

	// Mirrors the API's split so the preview matches what will be saved.
	const preview = $derived.by(() => {
		const n = chosen.length;
		if (!n || !total) return chosen.map(() => 0);
		if (mode === 'equal') {
			const base = Math.floor(total / n);
			const rem = total % n;
			return chosen.map((_, i) => base + (i < rem ? 1 : 0));
		}
		if (mode === 'amount') return chosen.map((p) => parseMoney(p.amount) ?? 0);
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

	function addPhotos(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		for (const file of Array.from(input.files ?? [])) {
			if (photos.length >= 6) break;
			photos.push({ file, url: URL.createObjectURL(file) });
		}
		input.value = '';
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
					value: mode === 'amount' ? (parseMoney(p.amount) ?? 0) : mode === 'weight' ? p.weight : 0
				}))
			});
			for (const p of photos) {
				const form = new FormData();
				appendImage(form, await compressImage(p.file));
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

	<form onsubmit={create} class="form">
		<section class="card main">
			<div class="field">
				<label for="title">Nome</label>
				<input id="title" class="input" maxlength="80" placeholder="Pizza de sexta" required bind:value={title} />
			</div>
			<div class="field">
				<label for="total">Valor total</label>
				<input id="total" class="input total" inputmode="decimal" placeholder="R$ 0,00" required bind:value={totalText} />
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
				{#if photos.length < 6}
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
			{#each people as p (p.user.id)}
				{@const idx = chosen.indexOf(p)}
				<div class="row person" class:off={!p.on}>
					<button type="button" class="tick" class:on={p.on} role="checkbox" aria-checked={p.on} aria-label="Incluir {p.user.name}" onclick={() => (p.on = !p.on)}>
						{#if p.on}<Icon name="check" size={14} stroke={3.5} />{/if}
					</button>
					<Avatar user={p.user} size={36} />
					<div class="grow">
						<strong>{p.user.id === me.id ? 'Você' : p.user.name}</strong>
						<span class="muted small">@{p.user.handle}</span>
					</div>
					{#if p.on && mode === 'amount'}
						<input class="input mini" inputmode="decimal" placeholder="0,00" aria-label="Valor de {p.user.name}" bind:value={p.amount} />
					{:else if p.on && mode === 'weight'}
						<div class="stepper">
							<button type="button" aria-label="Diminuir peso" onclick={() => (p.weight = Math.max(0, p.weight - 1))}>−</button>
							<span>{p.weight}</span>
							<button type="button" aria-label="Aumentar peso" onclick={() => (p.weight += 1)}>+</button>
						</div>
					{/if}
					{#if p.on && mode !== 'amount'}
						<strong class="share">{money(preview[idx] ?? 0)}</strong>
					{/if}
				</div>
			{/each}
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
			<button class="btn" disabled={!valid || busy}>{busy ? 'Criando…' : 'Criar cotinha'}</button>
		</div>
	</form>
</main>

<style>
	.form {
		display: flex;
		flex-direction: column;
		gap: 18px;
	}
	.main {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.total {
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
	.person.off {
		opacity: 0.6;
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
	.mini {
		width: 96px;
		height: 40px;
		text-align: right;
	}
	.stepper {
		display: flex;
		align-items: center;
		border: 1px solid var(--line-3);
		border-radius: 14px;
	}
	.stepper button {
		width: 40px;
		height: 40px;
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
		gap: 12px;
		position: sticky;
		bottom: 12px;
	}
</style>
