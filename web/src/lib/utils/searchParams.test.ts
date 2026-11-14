import { describe, expect, it } from 'vitest';
import { buildSearchParams, buildSearchUrl } from './searchParams';

describe('search sorting parameters', () => {
	it('sends random sorting and its saved seed without enabling vector inference', () => {
		const params = buildSearchParams({
			pageSize: 60,
			query: 'cat',
			vectorQuery: ' :random ',
			seed: 'saved',
			cursor: 'next'
		});
		expect(Object.fromEntries(params)).toEqual({
			page_size: '60',
			q: 'cat',
			sort: 'random',
			seed: 'saved',
			cursor: 'next'
		});
	});
	it('preserves a supplied seed in shareable URLs', () => {
		const url = new URL(
			buildSearchUrl({ pageSize: 60, vectorQuery: ':random', seed: 'abcde' }),
			'http://localhost'
		);
		expect(url.searchParams.get('seed')).toBe('abcde');
		expect(url.searchParams.get('sort')).toBe('random');
	});
	it('keeps ordinary similarity sorting compatible and drops random-only parameters', () => {
		const params = buildSearchParams({
			pageSize: 60,
			query: 'cat',
			vectorQuery: 'forest',
			seed: 'old-seed'
		});
		expect(Object.fromEntries(params)).toEqual({
			page_size: '60',
			q: 'cat',
			vector: '1',
			vector_q: 'forest'
		});
	});
});
