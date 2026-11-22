<script lang="ts">
	import { api, type PublicUser } from '$lib/api';
	import { firstName, money, shortDate } from '$lib/format';
	import Avatar from '$lib/components/Avatar.svelte';
	import Header from '$lib/components/Header.svelte';
	import Icon from '$lib/components/Icon.svelte';

	interface Note {
		id: string;
		kind: string;
		actor: PublicUser | null;
		cotinha_id: string;
		cotinha_title: string;
		payment_id: string;
		amount_cents: number;
		read: boolean;
		created_at: string;
	}

	let notes = $state<Note[] | null>(null);
	let error = $state('');

	async function load() {
		try {
			notes = await api.get<Note[]>('/notifications');
			// Opening the page reads everything; the list keeps showing what was new.
			if (notes.some((n) => !n.read)) await api.post('/notifications/read');
		} catch (e) {
			error = (e as Error).message;
		}
	}
	load();

	function text(n: Note): string {
		const who = n.actor ? firstName(n.actor.name) : 'Alguém';
		const amount = money(n.amount_cents);
		switch (n.kind) {
			case 'friend_request':
				return `${who} quer ser seu amigo no Cotinha.`;
			case 'friend_accepted':
				return `${who} aceitou seu pedido de amizade.`;
			case 'cotinha_added':
				return `${who} criou “${n.cotinha_title}”: sua parte é ${amount}.`;
			case 'cotinha_deleted':
				return `${who} apagou a cotinha “${n.cotinha_title}”. Você não deve mais os ${amount} dela.`;
			case 'payment_sent':
				return `${who} diz que te pagou ${amount}${n.cotinha_title ? ` (${n.cotinha_title})` : ''}. Confira e confirme.`;
			case 'payment_confirmed':
				return `${who} confirmou que recebeu seus ${amount}.`;
			case 'payment_rejected':
				return `${who} disse que seu Pix de ${amount} não caiu.`;
			case 'compensated':
				return `${who} compensou as dívidas entre vocês: ${amount} de cada lado.`;
		}
		return '';
	}

	function href(n: Note): string {
		switch (n.kind) {
			case 'friend_request':
			case 'friend_accepted':
				return '/amigos';
			case 'cotinha_added':
				return `/cotinha/${n.cotinha_id}`;
			case 'cotinha_deleted':
				return '/cotinhas';
			case 'compensated':
				return n.actor ? `/pagar/${n.actor.id}` : '/';
			default:
				return n.payment_id ? `/pagamento/${n.payment_id}` : '/';
		}
	}

	const icon: Record<string, string> = {
		payment_sent: 'receipt',
		payment_confirmed: 'check',
		payment_rejected: 'close',
		compensated: 'swap',
		cotinha_added: 'plus',
		cotinha_deleted: 'trash',
		friend_request: 'friends',
		friend_accepted: 'friends'
	};
</script>

<main class="page no-nav">
	<Header title="Notificações" />
	{#if error}<div class="error" role="alert">{error}</div>{/if}

	{#if notes && notes.length === 0}
		<section class="empty card">
			<Icon name="bell" size={32} stroke={1.8} />
			<p>Nada por aqui ainda. Pedidos de amizade, cotinhas novas e pagamentos aparecem aqui.</p>
		</section>
	{/if}

	<ul class="list">
		{#each notes ?? [] as n (n.id)}
			<li>
				<a class="note" class:new={!n.read} href={href(n)}>
					<div class="who">
						{#if n.actor}<Avatar user={n.actor} size={44} />{/if}
						<span class="kind"><Icon name={icon[n.kind] ?? 'bell'} size={12} stroke={3} /></span>
					</div>
					<div class="grow">
						<span>{text(n)}</span>
						<span class="muted small">{shortDate(n.created_at)}</span>
					</div>
					{#if !n.read}<span class="dot" aria-label="Nova"></span>{/if}
				</a>
			</li>
		{/each}
	</ul>
</main>

<style>
	.list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.note {
		background: var(--surface);
		border: 1px solid transparent;
		border-radius: 18px;
		padding: 14px 16px;
		display: flex;
		align-items: center;
		gap: 14px;
		text-decoration: none;
		color: var(--text);
		line-height: 1.35;
	}
	.note.new {
		background: var(--note-bg);
		border-color: var(--note-line);
	}
	.who {
		position: relative;
		flex-shrink: 0;
	}
	.kind {
		position: absolute;
		right: -4px;
		bottom: -4px;
		width: 22px;
		height: 22px;
		border-radius: 11px;
		background: var(--accent);
		color: var(--accent-ink);
		border: 2px solid var(--bg);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.grow {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 15px;
	}
	.dot {
		width: 10px;
		height: 10px;
		border-radius: 5px;
		background: var(--accent);
		flex-shrink: 0;
	}
	.empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 10px;
		text-align: center;
		color: var(--muted);
	}
	.empty p {
		margin: 0;
	}
</style>
