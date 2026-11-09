import { describe, expect, it } from 'vitest';
import { shouldPollMedia } from './mediaLifecycle';
import type { MediaDetail } from '../types/media';

const ready: MediaDetail = {
	id: 'media',
	original_name: '2026-11-09.png',
	url: '',
	width: 10,
	height: 10,
	format: 'png',
	size: 1,
	status: 'ready',
	error: null,
	embedding_availability: 'missing',
	tags: [],
	dates: [],
	embeddings: []
};

describe('media polling', () => {
	it('keeps polling ingestion, including retries', () => {
		for (const status of ['queued', 'processing', 'retrying'] as const) {
			expect(shouldPollMedia({ ...ready, status })).toBe(true);
		}
	});
	it('keeps polling an embedding retry instead of treating it as terminal failure', () => {
		expect(
			shouldPollMedia({
				...ready,
				embedding_availability: 'available',
				embeddings: [{ space: 'vision', status: 'retrying', error: 'temporary' }]
			})
		).toBe(true);
	});
	it('stops for unavailable, missing, ready, or terminally failed embeddings', () => {
		for (const embedding_availability of ['unavailable', 'missing'] as const) {
			expect(shouldPollMedia({ ...ready, embedding_availability })).toBe(false);
		}
		for (const status of ['ready', 'failed'] as const) {
			expect(
				shouldPollMedia({
					...ready,
					embedding_availability: 'available',
					embeddings: [{ space: 'vision', status, error: null }]
				})
			).toBe(false);
		}
	});
	it('stops obsolete embedding jobs when the server cannot run embeddings', () => {
		expect(
			shouldPollMedia({
				...ready,
				embedding_availability: 'unavailable',
				embeddings: [{ space: 'vision', status: 'queued', error: null }]
			})
		).toBe(false);
	});
	it('stops when ingestion fails even if an imported embedding exists', () => {
		expect(shouldPollMedia({ ...ready, status: 'failed' })).toBe(false);
	});
});
