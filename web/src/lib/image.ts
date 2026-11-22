// Pictures are shrunk in the browser before upload: smaller uploads on mobile data
// and less R2 storage. The API re-checks and re-compresses anything that slips through.

export const PHOTO_MAX = 1600; // cotinha photos and receipts (receipts need readable text)
export const AVATAR_SIZE = 512;

function canvasBlob(canvas: HTMLCanvasElement, type: string, quality: number): Promise<Blob | null> {
	return new Promise((resolve) => canvas.toBlob(resolve, type, quality));
}

/** Encodes a canvas as WebP when the browser supports it, otherwise JPEG. */
export async function encodeCanvas(canvas: HTMLCanvasElement, quality = 0.8): Promise<Blob> {
	const webp = await canvasBlob(canvas, 'image/webp', quality);
	if (webp && webp.type === 'image/webp') return webp;
	const jpeg = await canvasBlob(canvas, 'image/jpeg', quality);
	if (!jpeg) throw new Error('Não foi possível processar a imagem.');
	return jpeg;
}

/** Loads a picked file as an <img>, honouring the photo's EXIF rotation. */
export function loadImage(file: Blob): Promise<HTMLImageElement> {
	return new Promise((resolve, reject) => {
		const url = URL.createObjectURL(file);
		const img = new Image();
		img.onload = () => resolve(img);
		img.onerror = () => {
			URL.revokeObjectURL(url);
			reject(new Error('Este navegador não consegue abrir essa imagem. Use JPG ou PNG.'));
		};
		img.src = url;
	});
}

/** HEIC/HEIF (iPhone and many Android cameras), by type, name or file header. */
async function isHeif(file: Blob): Promise<boolean> {
	if (/image\/hei[cf]/.test(file.type)) return true;
	if (file instanceof File && /\.hei[cf]$/i.test(file.name)) return true;
	const head = new Uint8Array(await file.slice(0, 12).arrayBuffer());
	const text = String.fromCharCode(...head);
	return text.slice(4, 8) === 'ftyp' && ['heic', 'heix', 'hevc', 'hevx', 'heim', 'heis', 'mif1', 'msf1'].includes(text.slice(8, 12));
}

/**
 * Loads a picture the browser can draw. Browsers other than Safari can't open
 * HEIC/HEIF, so those are converted to JPEG first with a decoder that is only
 * downloaded when needed.
 */
export async function loadAnyImage(file: Blob): Promise<HTMLImageElement> {
	try {
		return await loadImage(file);
	} catch (err) {
		if (!(await isHeif(file))) throw err;
		const { default: heic2any } = await import('heic2any');
		const out = await heic2any({ blob: file, toType: 'image/jpeg', quality: 0.92 });
		return loadImage(Array.isArray(out) ? out[0] : out);
	}
}

/**
 * Resizes a picture so its longest side is at most maxSide and re-encodes it.
 * Falls back to the original file only when it can't be decoded at all; the API
 * then handles it.
 */
export async function compressImage(file: File, maxSide = PHOTO_MAX, quality = 0.8): Promise<Blob> {
	let img: HTMLImageElement;
	try {
		img = await loadAnyImage(file);
	} catch {
		return file;
	}
	const scale = Math.min(1, maxSide / Math.max(img.naturalWidth, img.naturalHeight));
	const canvas = document.createElement('canvas');
	canvas.width = Math.max(1, Math.round(img.naturalWidth * scale));
	canvas.height = Math.max(1, Math.round(img.naturalHeight * scale));
	const ctx = canvas.getContext('2d')!;
	ctx.fillStyle = '#ffffff'; // transparent PNGs become white, not black, if JPEG is used
	ctx.fillRect(0, 0, canvas.width, canvas.height);
	ctx.imageSmoothingQuality = 'high';
	ctx.drawImage(img, 0, 0, canvas.width, canvas.height);
	URL.revokeObjectURL(img.src);
	const out = await encodeCanvas(canvas, quality);
	const keepOriginal = scale === 1 && out.size >= file.size && !(await isHeif(file));
	return keepOriginal ? file : out;
}

/** Adds a picture to a form with a filename matching its type. */
export function appendImage(form: FormData, blob: Blob, name = 'foto') {
	const ext = blob.type === 'image/webp' ? 'webp' : blob.type === 'image/png' ? 'png' : 'jpg';
	form.append('file', blob, `${name}.${ext}`);
}
