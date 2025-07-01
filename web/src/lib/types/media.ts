export interface MediaDate {
	name: string;
	value: string;
}

export interface MediaItem {
	id: string;
	url: string;
	width: number;
	height: number;
	format: string;
	dates: MediaDate[];
}

export interface MediaDetail extends MediaItem {
	size: number;
	tags: string[];
}
