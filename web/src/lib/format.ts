const brl = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' });

export function money(cents: number): string {
	return brl.format(cents / 100);
}

const day = new Intl.DateTimeFormat('pt-BR', { day: '2-digit', month: 'short' });
const time = new Intl.DateTimeFormat('pt-BR', { hour: '2-digit', minute: '2-digit' });

export function shortDate(iso: string): string {
	const d = new Date(iso);
	const today = new Date();
	if (d.toDateString() === today.toDateString()) return 'hoje, ' + time.format(d);
	return day.format(d).replace('.', '');
}

export function untilText(iso: string): string {
	const d = new Date(iso);
	const tomorrow = new Date();
	tomorrow.setDate(tomorrow.getDate() + 1);
	const prefix = d.toDateString() === tomorrow.toDateString() ? 'amanhã' : day.format(d);
	return `${prefix}, ${time.format(d)}`;
}

export function firstName(name: string): string {
	return name.split(' ')[0] || name;
}
