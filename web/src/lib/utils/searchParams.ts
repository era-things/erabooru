export interface SearchParamsOptions {
	pageSize: number;
	query?: string;
	vectorQuery?: string | null | undefined;
	cursor?: string | null | undefined;
}

export function buildSearchParams({
	pageSize,
	query,
	vectorQuery,
	cursor
}: SearchParamsOptions): URLSearchParams {
	const params = new URLSearchParams({
		page_size: String(pageSize)
	});

	if (query) {
		params.set('q', query);
	}

	const vectorText = typeof vectorQuery === 'string' ? vectorQuery.trim() : '';
	if (vectorText) {
		params.set('vector', '1');
		params.set('vector_q', vectorText);
	}
	if (cursor) params.set('cursor', cursor);

	return params;
}

export function buildSearchUrl(options: SearchParamsOptions): string {
	return `/?${buildSearchParams(options).toString()}`;
}
