import type { MediaItem, MediaDetail, TagCount } from './types/media';
import { buildSearchParams } from './utils/searchParams';

const apiBase = '/api';

export interface MediaPreviewsResponse {
	media: MediaItem[];
	next_cursor: string;
}

export interface HiddenTagFilter {
	id: number;
	value: string;
	is_default: boolean;
}

export interface HiddenTagFiltersResponse {
	filters: HiddenTagFilter[];
	active: {
		id: number | null;
		value: string;
	};
}

export class ApiError extends Error {
	constructor(
		public status: number,
		message: string
	) {
		super(message);
	}
}

async function handleJson<T>(res: Response): Promise<T> {
	if (!res.ok) throw new ApiError(res.status, `HTTP ${res.status} ${res.statusText}`);
	return res.json() as Promise<T>;
}

export async function fetchMediaPreviews(
	query: string,
	pageSize: number,
	vectorQuery?: string | null,
	cursor?: string | null,
	signal?: AbortSignal,
	seed?: string | null
): Promise<MediaPreviewsResponse> {
	const params = buildSearchParams({ pageSize, query, vectorQuery, cursor, seed });
	const res = await fetch(`${apiBase}/media/previews?${params.toString()}`, { signal });
	return handleJson(res);
}

export async function fetchMediaDetail(id: string, signal?: AbortSignal): Promise<MediaDetail> {
	const res = await fetch(`${apiBase}/media/${id}`, { signal });
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

export interface TagRecommendationsResponse {
	recommended: { name: string; score: number; examples: number; muted?: boolean }[];
	low_evidence: { name: string; score: number; examples: number }[];
	negative: string[];
	embedding_ready: boolean;
}

export type TagFeedbackAction = 'accept' | 'reject' | 'clear';

export async function fetchTagRecommendations(
	id: string,
	signal?: AbortSignal
): Promise<TagRecommendationsResponse> {
	return handleJson(await fetch(`${apiBase}/media/${id}/tag-recommendations`, { signal }));
}

export async function sendTagFeedback(
	id: string,
	tag: string,
	action: TagFeedbackAction
): Promise<void> {
	const res = await fetch(`${apiBase}/media/${id}/tag-feedback`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ tag, action })
	});
	if (!res.ok) throw new ApiError(res.status, `Could not save tag feedback (HTTP ${res.status})`);
}

export interface TagStatisticsStatus {
	state: string;
	rebuilt_at: string | null;
	legacy_embeddings: number;
	active_profile: string;
	tags: number;
}

export async function fetchTagStatisticsStatus(signal?: AbortSignal): Promise<TagStatisticsStatus> {
	return handleJson(await fetch(`${apiBase}/settings/tag-statistics`, { signal }));
}

export async function rebuildTagStatistics(legacyProfile = ''): Promise<void> {
	const query = legacyProfile ? `?legacy_profile=${encodeURIComponent(legacyProfile)}` : '';
	const res = await fetch(`${apiBase}/settings/tag-statistics/rebuild${query}`, { method: 'POST' });
	if (!res.ok) throw new ApiError(res.status, `Could not start rebuild (HTTP ${res.status})`);
}

export async function uploadMedia(file: File): Promise<{ id: string; existed: boolean }> {
	const body = new FormData();
	body.append('file', file, file.name);
	const res = await fetch(`${apiBase}/media`, { method: 'POST', body });
	return handleJson(res);
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

export interface TagReviewResponse extends MediaPreviewsResponse {
	representatives: MediaItem[];
}

export async function fetchTagReview(
	tag: string,
	cursor = '',
	signal?: AbortSignal
): Promise<TagReviewResponse> {
	const params = new URLSearchParams({ tag, cursor });
	return handleJson(await fetch(`${apiBase}/tags/review?${params}`, { signal }));
}

export async function fetchTagSuggestions(prefix: string): Promise<TagCount[]> {
	const params = new URLSearchParams({ q: prefix });
	const res = await fetch(`${apiBase}/tags/suggest?${params.toString()}`);
	const data = await handleJson<{ tags: TagCount[] }>(res);
	return data.tags;
}

export async function fetchHiddenTagFilters(): Promise<HiddenTagFiltersResponse> {
	const res = await fetch(`${apiBase}/settings/hidden-tags`);
	return handleJson<HiddenTagFiltersResponse>(res);
}

export async function createHiddenTagFilter(value: string): Promise<void> {
	const res = await fetch(`${apiBase}/settings/hidden-tags`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ value })
	});
	if (!res.ok) {
		const message = await res.json().catch(() => ({}));
		throw new Error(message.error ?? `HTTP ${res.status}`);
	}
}

export async function selectHiddenTagFilter(id: number): Promise<void> {
	const res = await fetch(`${apiBase}/settings/hidden-tags/${id}/select`, {
		method: 'POST'
	});
	if (!res.ok) {
		throw new Error(`HTTP ${res.status}`);
	}
}

export async function deleteHiddenTagFilter(id: number): Promise<void> {
	const res = await fetch(`${apiBase}/settings/hidden-tags/${id}`, {
		method: 'DELETE'
	});
	if (!res.ok && res.status !== 404) {
		const message = await res.json().catch(() => ({}));
		throw new Error(message.error ?? `HTTP ${res.status}`);
	}
}
