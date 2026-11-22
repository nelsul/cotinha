<script lang="ts">
	import { api } from '$lib/api';
	import Icon from './Icon.svelte';

	// `ok` is true only once the server confirmed the @ is valid and free (or already yours).
	let { handle = $bindable(''), ok = $bindable(false), label = 'Seu @' }: { handle: string; ok: boolean; label?: string } =
		$props();

	type Status = 'idle' | 'checking' | 'free' | 'taken' | 'invalid' | 'error';
	let status = $state<Status>('idle');

	$effect(() => {
		const h = handle;
		ok = false;
		if (!h) {
			status = 'idle';
			return;
		}
		if (!/^[a-z0-9._]{3,20}$/.test(h)) {
			status = 'invalid';
			return;
		}
		status = 'checking';
		const t = setTimeout(async () => {
			try {
				const r = await api.get<{ handle: string; valid: boolean; available: boolean }>(
					`/handles/available?handle=${encodeURIComponent(h)}`
				);
				if (r.handle !== handle) return; // typed more meanwhile
				status = !r.valid ? 'invalid' : r.available ? 'free' : 'taken';
				ok = status === 'free';
			} catch {
				status = 'error';
			}
		}, 350);
		return () => clearTimeout(t);
	});

	const messages: Record<Status, string> = {
		idle: 'De 3 a 20 caracteres: letras minúsculas, números, ponto ou _.',
		checking: 'Verificando se o @ está livre…',
		free: 'Disponível! Esse @ é seu.',
		taken: 'Esse @ já está em uso. Tente outro.',
		invalid: 'Use de 3 a 20 caracteres: letras minúsculas, números, ponto ou _.',
		error: 'Não foi possível verificar agora.'
	};
</script>

<div class="field">
	<label for="handle">{label}</label>
	<div class="at" class:good={status === 'free'} class:bad={status === 'taken' || status === 'invalid'}>
		<span class="prefix">@</span>
		<input
			id="handle"
			class="input"
			autocapitalize="none"
			autocomplete="username"
			spellcheck="false"
			maxlength="20"
			required
			aria-describedby="handle-status"
			aria-invalid={status === 'taken' || status === 'invalid'}
			bind:value={handle}
			oninput={() => (handle = handle.toLowerCase().replace(/[^a-z0-9._]/g, ''))}
		/>
		<span class="state" aria-hidden="true">
			{#if status === 'checking'}<span class="spinner"></span>
			{:else if status === 'free'}<Icon name="check" size={18} stroke={3} />
			{:else if status === 'taken' || status === 'invalid'}<Icon name="close" size={18} stroke={3} />{/if}
		</span>
	</div>
	<span id="handle-status" class="msg {status}" aria-live="polite">{messages[status]}</span>
</div>

<style>
	.at {
		position: relative;
	}
	.prefix {
		position: absolute;
		left: 14px;
		top: 50%;
		transform: translateY(-50%);
		color: var(--muted);
		font-size: 16px;
	}
	.at .input {
		padding-left: 32px;
		padding-right: 44px;
	}
	.at.good .input {
		border-color: var(--accent);
	}
	.at.bad .input {
		border-color: var(--owe);
	}
	.state {
		position: absolute;
		right: 14px;
		top: 50%;
		transform: translateY(-50%);
		display: flex;
		color: var(--accent);
	}
	.at.bad .state {
		color: var(--owe);
	}
	.spinner {
		width: 16px;
		height: 16px;
		border-radius: 50%;
		border: 2px solid var(--line-3);
		border-top-color: var(--accent);
		animation: spin 0.8s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	.msg {
		font-size: 13px;
		color: var(--muted);
	}
	.msg.free {
		color: var(--accent);
	}
	.msg.taken,
	.msg.invalid {
		color: var(--warn);
	}
</style>
