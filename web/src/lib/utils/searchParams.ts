export interface SearchParamsOptions {
	page: number;
	pageSize: number;
	query?: string;
	vectorQuery?: string | null | undefined;
}

export function buildSearchParams({
	page,
	pageSize,
	query,
	vectorQuery
}: SearchParamsOptions): URLSearchParams {
	const params = new URLSearchParams({
		page: String(page),
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

	return params;
}

export function buildSearchUrl(options: SearchParamsOptions): string {
	return `/?${buildSearchParams(options).toString()}`;
}
