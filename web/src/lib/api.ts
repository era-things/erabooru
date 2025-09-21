import type { MediaItem, MediaDetail, TagCount } from './types/media';

const apiBase = '/api';

export interface MediaPreviewsResponse {
	media: MediaItem[];
	total: number;
}

async function handleJson<T>(res: Response): Promise<T> {
	if (!res.ok) throw new Error(`HTTP ${res.status} ${res.statusText}`);
	return res.json() as Promise<T>;
}

export async function fetchMediaPreviews(
	query: string,
	page: number,
	pageSize: number,
	vectorSearch = false
): Promise<MediaPreviewsResponse> {
	const params = new URLSearchParams({
		page: page.toString(),
		page_size: pageSize.toString()
	});
	if (query) {
		params.set('q', query);
	}
	if (vectorSearch) {
		params.set('vector', '1');
	}
	const res = await fetch(`${apiBase}/media/previews?${params.toString()}`);
	return handleJson(res);
}

export async function fetchMediaDetail(id: string): Promise<MediaDetail> {
	const res = await fetch(`${apiBase}/media/${id}`);
	return handleJson(res);
}

export async function deleteMedia(id: string): Promise<void> {
	const res = await fetch(`${apiBase}/media/${id}`, { method: 'DELETE' });
	if (!res.ok) throw new Error(`HTTP ${res.status}`);
}

export async function updateMediaTags(id: string, tags: string[]): Promise<void> {
	const res = await fetch(`${apiBase}/media/${id}/tags`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ tags })
	});
	if (!res.ok) throw new Error(`HTTP ${res.status}`);
}

export async function requestUploadUrl(filename: string): Promise<string> {
	const res = await fetch(`${apiBase}/media/upload-url`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ filename })
	});
	const data = await handleJson<{ url: string }>(res);
	return data.url;
}

export async function uploadToPresignedUrl(url: string, file: File): Promise<Response> {
	return fetch(url, {
		method: 'PUT',
		body: file,
		headers: {
			'Content-Type': file.type || 'application/octet-stream',
			'If-None-Match': '*'
		}
	});
}

export async function regenerateReverseIndex(): Promise<void> {
	const res = await fetch(`${apiBase}/admin/regenerate`, { method: 'POST' });
	if (!res.ok) throw new Error(`HTTP ${res.status}`);
}

export async function downloadMediaTags(): Promise<Blob> {
	const res = await fetch(`${apiBase}/admin/export-tags`);
	if (!res.ok) throw new Error(`HTTP ${res.status}`);
	return res.blob();
}

export async function importMediaTags(file: File): Promise<void> {
	const res = await fetch(`${apiBase}/admin/import-tags`, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-ndjson',
			'Content-Encoding': 'gzip'
		},
		body: file
	});
	if (!res.ok) throw new Error(`HTTP ${res.status}`);
}

export async function fetchTags(): Promise<TagCount[]> {
	const res = await fetch(`${apiBase}/tags`);
	const data = await handleJson<{ tags: TagCount[] }>(res);
	return data.tags;
}

export async function fetchSimilarMedia(
	vector: number[],
	limit: number,
	exclude?: string,
	name = 'vision'
): Promise<MediaItem[]> {
	const res = await fetch(`${apiBase}/media/similar`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ vector, limit, exclude, name })
	});
	const data = await handleJson<{ media: MediaItem[] }>(res);
	return data.media;
}
