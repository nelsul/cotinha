<script lang="ts">
	import QRCode from 'qrcode';
	import { session } from '$lib/session.svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import Header from '$lib/components/Header.svelte';
	import Icon from '$lib/components/Icon.svelte';

	const me = session.me!;
	const link = `${location.origin}/add/${me.handle}`;
	let qr = $state('');
	let tab = $state<'mine' | 'scan'>('mine');
	let shared = $state('');

	QRCode.toDataURL(link, { margin: 1, width: 480, color: { dark: '#0B0F0D', light: '#F1F5F2' } }).then((u) => (qr = u));

	async function share() {
		const text = `Me adiciona no Cotinha: @${me.handle}`;
		if (navigator.share) {
			await navigator.share({ title: 'Cotinha', text, url: link }).catch(() => {});
		} else {
			await navigator.clipboard.writeText(link);
			shared = 'Link copiado!';
			setTimeout(() => (shared = ''), 2000);
		}
	}
</script>

<main class="page no-nav">
	<Header title="Adicionar amigos" back="/amigos" />

	<div class="segmented" style="grid-template-columns: repeat(2, minmax(0, 1fr))" role="tablist">
		<button role="tab" aria-selected={tab === 'mine'} class:on={tab === 'mine'} onclick={() => (tab = 'mine')}>Meu código</button>
		<button role="tab" aria-selected={tab === 'scan'} class:on={tab === 'scan'} onclick={() => (tab = 'scan')}>Escanear</button>
	</div>

	{#if tab === 'mine'}
		<section class="badge">
			<div class="me">
				<Avatar user={me} size={84} />
				<strong class="name">{me.name}</strong>
				<span>@{me.handle}</span>
			</div>
			{#if qr}<img class="qr" src={qr} alt="QR code para me adicionar como amigo" />{/if}
			<p>Quem ler este código com a câmera do celular abre o Cotinha e te manda um pedido de amizade.</p>
		</section>
		<button class="btn" onclick={share}><Icon name="share" size={18} />{shared || 'Compartilhar link'}</button>
	{:else}
		<section class="card scan">
			<Icon name="camera" size={40} stroke={1.8} />
			<p>Abra a <strong>câmera do celular</strong> e aponte para o QR do seu amigo. O link abre aqui no Cotinha e é só tocar em <strong>Adicionar</strong>.</p>
			<a class="btn ghost" href="/amigos">Ou busque pelo @</a>
		</section>
	{/if}
</main>

<style>
	.badge {
		background: var(--accent);
		color: var(--accent-ink);
		border-radius: 32px;
		padding: 24px;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
		margin-top: 44px;
	}
	.me {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 4px;
		margin-top: -66px;
	}
	.me :global(img),
	.me :global(.initial) {
		border: 5px solid var(--bg);
	}
	.name {
		font-size: 22px;
		font-weight: 800;
	}
	.qr {
		width: min(240px, 70vw);
		border-radius: 20px;
	}
	.badge p {
		margin: 0;
		font-weight: 600;
		text-align: center;
		max-width: 280px;
		font-size: 14px;
	}
	.scan {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 14px;
		text-align: center;
		color: var(--soft);
	}
	.scan p {
		margin: 0;
		line-height: 1.5;
	}
</style>
