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
}

export interface MediaDetail extends MediaItem {
	size: number;
	tags: TagCount[];
	dates: MediaDate[];
}

export interface TagCount {
	name: string;
	count: number;
}

export interface MediaPreviewItem extends MediaItem {
	/**
	 * Height of the preview element. This may be smaller than the
	 * original height when the image is cropped for display.
	 */
	displayHeight: number;
	/** Height of the original image before cropping. */
	originalHeight: number;
	/** Whether the preview is cropped because it's too tall. */
	cropped: boolean;
}
