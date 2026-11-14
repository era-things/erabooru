export interface SearchParamsOptions {
	pageSize: number;
	query?: string;
	vectorQuery?: string | null | undefined;
	cursor?: string | null | undefined;
	seed?: string | null;
}

export function createRandomSeed(): string {
	return Array.from({ length: 5 }, () =>
		String.fromCharCode(97 + Math.floor(Math.random() * 26))
	).join('');
}

export function buildSearchParams({
	pageSize,
	query,
	vectorQuery,
	cursor,
	seed
}: SearchParamsOptions): URLSearchParams {
	const params = new URLSearchParams({
		page_size: String(pageSize)
	});

	if (query) {
		params.set('q', query);
	}

	const vectorText = typeof vectorQuery === 'string' ? vectorQuery.trim() : '';
	if (vectorText === ':random') {
		params.set('sort', 'random');
		if (seed) params.set('seed', seed);
	} else if (vectorText) {
		params.set('vector', '1');
		params.set('vector_q', vectorText);
	}
	if (cursor) params.set('cursor', cursor);

	return params;
}

export function buildSearchUrl(options: SearchParamsOptions): string {
	return `/?${buildSearchParams(options).toString()}`;
}
