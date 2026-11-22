<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { api, ApiError, type Me } from '$lib/api';
	import { session } from '$lib/session.svelte';

	let step = $state<'email' | 'code'>('email');
	let email = $state('');
	let code = $state('');
	let busy = $state(false);
	let error = $state('');

	function safeNext(): string {
		const n = page.url.searchParams.get('next') ?? '/';
		return n.startsWith('/') && !n.startsWith('//') ? n : '/';
	}

	async function sendCode(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			await api.post('/auth/request-code', { email });
			step = 'code';
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Algo deu errado.';
		} finally {
			busy = false;
		}
	}

	async function verify(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const me = await api.post<Me>('/auth/verify', { email, code });
			session.me = me;
			session.loaded = true;
			const next = safeNext();
			await goto(me.onboarded ? next : `/onboarding${next !== '/' ? `?next=${encodeURIComponent(next)}` : ''}`, {
				replaceState: true
			});
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Algo deu errado.';
		} finally {
			busy = false;
		}
	}
</script>

<main class="page no-nav">
	<div class="hero">
		<div class="logo">cotinha<span>.</span></div>
		<p>Divida contas com os amigos sem fazer conta de cabeça.</p>
	</div>

	{#if step === 'email'}
		<form onsubmit={sendCode} class="card form">
			<div class="field">
				<label for="email">Seu e-mail</label>
				<input id="email" class="input" type="email" autocomplete="email" inputmode="email" required bind:value={email} />
			</div>
			{#if error}<div class="error" role="alert">{error}</div>{/if}
			<button class="btn" disabled={busy || !email}>{busy ? 'Enviando…' : 'Receber código'}</button>
			<p class="muted">Sem senha: mandamos um código de 6 números para o seu e-mail.</p>
		</form>
	{:else}
		<form onsubmit={verify} class="card form">
			<p class="muted">Enviamos um código para <strong style="color: var(--text)">{email}</strong>.</p>
			<div class="field">
				<label for="code">Código</label>
				<input
					id="code"
					class="input code"
					inputmode="numeric"
					autocomplete="one-time-code"
					maxlength="6"
					required
					bind:value={code}
				/>
			</div>
			{#if error}<div class="error" role="alert">{error}</div>{/if}
			<button class="btn" disabled={busy || code.length !== 6}>{busy ? 'Entrando…' : 'Entrar'}</button>
			<button type="button" class="btn ghost" onclick={() => ((step = 'email'), (code = ''), (error = ''))}>Usar outro e-mail</button>
		</form>
	{/if}
</main>

<style>
	.hero {
		margin-top: 12vh;
		margin-bottom: 16px;
	}
	.logo {
		font-size: 48px;
		font-weight: 800;
		letter-spacing: -1px;
	}
	.logo span {
		color: var(--accent);
	}
	.hero p {
		color: var(--soft);
		font-size: 18px;
		margin: 8px 0 0;
	}
	.form {
		display: flex;
		flex-direction: column;
		gap: 14px;
	}
	.form p {
		margin: 0;
	}
	.code {
		font-size: 28px;
		letter-spacing: 10px;
		text-align: center;
		font-weight: 800;
	}
</style>
