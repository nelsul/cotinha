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

/**
 * Resizes a picture so its longest side is at most maxSide and re-encodes it.
 * Falls back to the original file when the browser can't decode it (e.g. HEIC
 * outside Safari); the API then handles it.
 */
export async function compressImage(file: File, maxSide = PHOTO_MAX, quality = 0.8): Promise<Blob> {
	let img: HTMLImageElement;
	try {
		img = await loadImage(file);
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
	return scale === 1 && out.size >= file.size ? file : out;
}

/** Adds a picture to a form with a filename matching its type. */
export function appendImage(form: FormData, blob: Blob, name = 'foto') {
	const ext = blob.type === 'image/webp' ? 'webp' : blob.type === 'image/png' ? 'png' : 'jpg';
	form.append('file', blob, `${name}.${ext}`);
}
