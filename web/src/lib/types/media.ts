export interface MediaItem {
	id: string;
	url: string;
	width: number;
	height: number;
	format: string;
}

export interface MediaDetail extends MediaItem {
	size: number;
	tags: string[];
}
