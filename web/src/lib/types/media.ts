export interface MediaItem {
	id: number;
	url: string;
	width: number;
	height: number;
	format: string;
}

export interface MediaDetail extends MediaItem {
	size: number;
	tags: string[];
}
