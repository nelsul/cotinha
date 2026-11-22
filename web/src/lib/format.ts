const brl = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' });

export function money(cents: number): string {
	return brl.format(cents / 100);
}

/** Parses "80", "80,5", "1.234,56" or "R$ 12,00" into cents, or null. */
export function parseMoney(input: string): number | null {
	const clean = input.replace(/[^\d,.]/g, '');
	if (!clean) return null;
	let normalized = clean;
	if (clean.includes(',')) {
		normalized = clean.replace(/\./g, '').replace(',', '.');
	}
	const value = Number(normalized);
	if (!Number.isFinite(value) || value <= 0) return null;
	return Math.round(value * 100);
}

export function centsToInput(cents: number): string {
	return (cents / 100).toFixed(2).replace('.', ',');
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
