export interface MediaItem {
	id: string;
	url: string;
	width: number;
	height: number;
	format: string;
	upload_date: Date;
}

export interface MediaDetail extends MediaItem {
	size: number;
	tags: string[];
}
