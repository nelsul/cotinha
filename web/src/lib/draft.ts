// Keeps the "Nova cotinha" form between visits. IndexedDB is used (not
// localStorage) so the picked photos survive too. Every call fails softly:
// private mode or blocked storage just means no draft.

const DB = 'cotinha';
const STORE = 'drafts';

function open(): Promise<IDBDatabase> {
	return new Promise((resolve, reject) => {
		const req = indexedDB.open(DB, 1);
		req.onupgradeneeded = () => req.result.createObjectStore(STORE);
		req.onsuccess = () => resolve(req.result);
		req.onerror = () => reject(req.error);
	});
}

async function run<T>(mode: IDBTransactionMode, fn: (s: IDBObjectStore) => IDBRequest): Promise<T | undefined> {
	try {
		const db = await open();
		return await new Promise<T>((resolve, reject) => {
			const req = fn(db.transaction(STORE, mode).objectStore(STORE));
			req.onsuccess = () => resolve(req.result as T);
			req.onerror = () => reject(req.error);
		});
	} catch {
		return undefined;
	}
}

export const loadDraft = <T>(key: string) => run<T>('readonly', (s) => s.get(key));
export const saveDraft = (key: string, value: unknown) => run('readwrite', (s) => s.put(value, key));
export const clearDraft = (key: string) => run('readwrite', (s) => s.delete(key));
