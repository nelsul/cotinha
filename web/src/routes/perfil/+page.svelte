<script lang="ts">
	import { api, ApiError, type Me } from '$lib/api';
	import { session } from '$lib/session.svelte';
	import { appendImage } from '$lib/image';
	import Avatar from '$lib/components/Avatar.svelte';
	import AvatarCropper from '$lib/components/AvatarCropper.svelte';
	import HandleField from '$lib/components/HandleField.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import PixKeyFields from '$lib/components/PixKeyFields.svelte';

	const me = $derived(session.me!);
	let editing = $state(false);
	let name = $state('');
	let handle = $state('');
	let handleOk = $state(false);
	let picked = $state<File | null>(null);
	let kind = $state('cpf');
	let key = $state('');
	let busy = $state(false);
	let error = $state('');
	let uploading = $state(false);

	const kindLabel: Record<string, string> = { cpf: 'CPF', cnpj: 'CNPJ', email: 'E-mail', phone: 'Celular', random: 'Aleatória' };

	function startEdit() {
		name = me.name;
		handle = me.handle;
		kind = me.pix_key_type;
		key = me.pix_key;
		error = '';
		editing = true;
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			session.me = await api.patch<Me>('/me', { name, handle, pix_key_type: kind, pix_key: key });
			editing = false;
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Algo deu errado.';
		} finally {
			busy = false;
		}
	}

	function pick(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		picked = input.files?.[0] ?? null;
		input.value = '';
	}

	async function upload(blob: Blob) {
		uploading = true;
		error = '';
		try {
			const form = new FormData();
			appendImage(form, blob, 'perfil');
			session.me = await api.post<Me>('/me/avatar', form);
			picked = null;
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Não foi possível enviar a foto.';
		} finally {
			uploading = false;
		}
	}

	async function logout() {
		await api.post('/auth/logout').catch(() => {});
		location.href = '/login';
	}
</script>

<main class="page">
	<section class="head">
		<div class="pic">
			<Avatar user={me} size={112} ring />
			<label class="cam" aria-label="Trocar foto de perfil">
				<Icon name="camera" size={20} stroke={2.2} />
				<input type="file" accept="image/*" class="sr-only" onchange={pick} disabled={uploading} />
			</label>
		</div>
		<strong class="name">{me.name}</strong>
		<span class="muted" style="font-size: 15px">@{me.handle}{uploading ? ' · enviando foto…' : ''}</span>
	</section>
	{#if error}<div class="error" role="alert">{error}</div>{/if}

	{#if editing}
		<form class="form" onsubmit={save}>
			<div class="field">
				<label for="name">Nome</label>
				<input id="name" class="input" maxlength="40" required bind:value={name} />
			</div>
			<HandleField bind:handle bind:ok={handleOk} />
			<PixKeyFields bind:kind bind:key />
			<button class="btn" disabled={busy || !handleOk}>{busy ? 'Salvando…' : 'Salvar'}</button>
			<button type="button" class="btn ghost" onclick={() => (editing = false)}>Cancelar</button>
		</form>
	{:else}
		<section class="card group">
			<button class="item" onclick={startEdit}>
				<div><span class="muted small">Nome</span><strong>{me.name}</strong></div>
				<Icon name="chevron" size={18} />
			</button>
			<button class="item" onclick={startEdit}>
				<div><span class="muted small">Seu @ (como te acham)</span><strong>@{me.handle}</strong></div>
				<Icon name="chevron" size={18} />
			</button>
			<button class="item" onclick={startEdit}>
				<div><span class="muted small">Chave Pix para receber · {kindLabel[me.pix_key_type]}</span><strong>{me.pix_key}</strong></div>
				<Icon name="chevron" size={18} />
			</button>
		</section>
		<section class="card group">
			<a class="item" href="/qr"><span>Meu QR de amizade</span><Icon name="chevron" size={18} /></a>
			<a class="item" href="/amigos"><span>Amigos e pedidos bloqueados</span><Icon name="chevron" size={18} /></a>
			<div class="item"><span>E-mail</span><span class="muted">{me.email}</span></div>
			<button class="item out" onclick={logout}><span>Sair</span></button>
		</section>
	{/if}
</main>

{#if picked}
	<AvatarCropper file={picked} onsave={upload} oncancel={() => (picked = null)} />
{/if}

<style>
	.head {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 6px;
		margin-top: 8px;
	}
	.pic {
		position: relative;
	}
	.cam {
		position: absolute;
		right: -4px;
		bottom: -4px;
		width: 44px;
		height: 44px;
		border-radius: 22px;
		border: 3px solid var(--bg);
		background: var(--accent);
		color: var(--accent-ink);
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: pointer;
	}
	.name {
		font-size: 24px;
		font-weight: 800;
		margin-top: 8px;
	}
	.form {
		display: flex;
		flex-direction: column;
		gap: 14px;
	}
	.group {
		padding: 0;
		display: flex;
		flex-direction: column;
	}
	.item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 14px 16px;
		border: none;
		background: none;
		text-align: left;
		color: var(--text);
		text-decoration: none;
		border-bottom: 1px solid var(--line);
		font-size: 15px;
		width: 100%;
	}
	.item:last-child {
		border-bottom: none;
	}
	.item div {
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 0;
		overflow-wrap: anywhere;
	}
	.item strong {
		font-size: 16px;
		font-weight: 600;
	}
	.out {
		color: var(--warn);
	}
</style>
