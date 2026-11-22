<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { api, ApiError, type Me } from '$lib/api';
	import { session } from '$lib/session.svelte';
	import HandleField from '$lib/components/HandleField.svelte';
	import PixKeyFields from '$lib/components/PixKeyFields.svelte';

	let name = $state(session.me?.name ?? '');
	let handle = $state(session.me?.handle ?? '');
	let handleOk = $state(false);
	let kind = $state(session.me?.pix_key_type || 'cpf');
	let key = $state(session.me?.pix_key ?? '');
	let busy = $state(false);
	let error = $state('');

	function suggest() {
		if (handle || !name) return;
		handle = name
			.normalize('NFD')
			.replace(/[̀-ͯ]/g, '')
			.toLowerCase()
			.replace(/[^a-z0-9]+/g, '.')
			.replace(/^\.+|\.+$/g, '')
			.slice(0, 20);
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const me = await api.patch<Me>('/me', { name, handle, pix_key_type: kind, pix_key: key });
			session.me = me;
			const n = page.url.searchParams.get('next') ?? '/';
			await goto(n.startsWith('/') && !n.startsWith('//') ? n : '/', { replaceState: true });
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Algo deu errado.';
		} finally {
			busy = false;
		}
	}
</script>

<main class="page no-nav">
	<div>
		<h1 class="title">Bem-vindo ao Cotinha</h1>
		<p class="muted" style="font-size: 15px">Só três coisas e você já pode dividir contas.</p>
	</div>
	<form onsubmit={save} class="form">
		<div class="field">
			<label for="name">Seu nome</label>
			<input id="name" class="input" autocomplete="name" maxlength="40" required bind:value={name} onblur={suggest} />
		</div>
		<HandleField bind:handle bind:ok={handleOk} label="Seu @ (é assim que seus amigos te acham)" />
		<PixKeyFields bind:kind bind:key />
		{#if error}<div class="error" role="alert">{error}</div>{/if}
		<button class="btn" disabled={busy || !handleOk}>{busy ? 'Salvando…' : 'Começar'}</button>
	</form>
</main>

<style>
	.form {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
</style>
