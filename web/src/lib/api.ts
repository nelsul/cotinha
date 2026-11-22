// Thin client for the Go API. Errors carry the API's Portuguese message.

export class ApiError extends Error {
	constructor(
		public status: number,
		message: string
	) {
		super(message);
	}
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
	const init: RequestInit = { method, credentials: 'same-origin', headers: {} };
	if (body instanceof FormData) {
		init.body = body;
	} else if (body !== undefined) {
		init.body = JSON.stringify(body);
		(init.headers as Record<string, string>)['Content-Type'] = 'application/json';
	}
	let res: Response;
	try {
		res = await fetch('/api' + path, init);
	} catch {
		throw new ApiError(0, 'Sem conexão. Verifique a internet.');
	}
	const data = await res.json().catch(() => ({}));
	if (!res.ok) {
		// A session that expired mid-use goes back to login. The session check
		// itself (/me) and the login pages handle 401 on their own.
		if (res.status === 401 && path !== '/me' && !path.startsWith('/auth') && location.pathname !== '/login') {
			location.href = '/login';
		}
		throw new ApiError(res.status, data.error ?? 'Algo deu errado.');
	}
	return data as T;
}

export const api = {
	get: <T>(path: string) => request<T>('GET', path),
	post: <T>(path: string, body?: unknown) => request<T>('POST', path, body ?? {}),
	patch: <T>(path: string, body: unknown) => request<T>('PATCH', path, body),
	put: <T>(path: string, body: unknown) => request<T>('PUT', path, body),
	del: <T>(path: string) => request<T>('DELETE', path)
};

export interface PublicUser {
	id: string;
	name: string;
	handle: string;
	avatar_url: string;
}

export interface Me extends PublicUser {
	email: string;
	pix_key_type: string;
	pix_key: string;
	onboarded: boolean;
	email_notifications: boolean;
}

export interface Balance {
	user: PublicUser;
	they_owe_cents: number;
	i_owe_cents: number;
	pending_in_cents: number;
	pending_out_cents: number;
}

export interface Payment {
	id: string;
	payer: PublicUser;
	payee: PublicUser;
	amount_cents: number;
	fee_cents: number;
	method: string;
	status: 'pending' | 'confirmed' | 'rejected';
	confirmed_by: string;
	receipt_url: string;
	created_at: string;
	decided_at: string | null;
	cotinhas: { cotinha_id: string; title: string; amount_cents: number }[];
}

export interface CotinhaSummary {
	id: string;
	title: string;
	total_cents: number;
	created_at: string;
	creditor: PublicUser;
	role: 'creditor' | 'debtor';
	my_share_cents: number;
	outstanding_cents: number;
	settled: boolean;
	people: number;
	photo_url: string;
	hidden_from_stats: boolean;
}

export type Relation = 'self' | 'friend' | 'request_sent' | 'request_received' | 'none';
