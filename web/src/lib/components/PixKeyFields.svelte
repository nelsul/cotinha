<script lang="ts">
	let { kind = $bindable('cpf'), key = $bindable('') }: { kind: string; key: string } = $props();

	const kinds = [
		{ value: 'cpf', label: 'CPF', placeholder: '000.000.000-00', mode: 'numeric' },
		{ value: 'phone', label: 'Celular', placeholder: '(00) 00000-0000', mode: 'tel' },
		{ value: 'email', label: 'E-mail', placeholder: 'voce@email.com', mode: 'email' },
		{ value: 'random', label: 'Aleatória', placeholder: '1234abcd-…', mode: 'text' },
		{ value: 'cnpj', label: 'CNPJ', placeholder: '00.000.000/0000-00', mode: 'numeric' }
	] as const;
	const current = $derived(kinds.find((k) => k.value === kind) ?? kinds[0]);
</script>

<div class="field">
	<span class="label" id="pix-kind-label">Tipo da chave Pix</span>
	<div class="kinds" role="radiogroup" aria-labelledby="pix-kind-label">
		{#each kinds as k (k.value)}
			<button type="button" role="radio" aria-checked={kind === k.value} class="chip" class:on={kind === k.value} onclick={() => (kind = k.value)}>
				{k.label}
			</button>
		{/each}
	</div>
</div>
<div class="field">
	<label for="pix-key">Chave Pix</label>
	<input id="pix-key" class="input" inputmode={current.mode} placeholder={current.placeholder} required bind:value={key} />
	<span class="muted">É para essa chave que seus amigos vão te pagar.</span>
</div>

<style>
	.label {
		font-size: 14px;
		color: var(--muted);
	}
	.kinds {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
	.kinds .chip {
		height: 38px;
	}
</style>
