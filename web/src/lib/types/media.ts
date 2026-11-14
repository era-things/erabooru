export interface MediaDate {
	name: string;
	value: string;
}

export interface MediaItem {
	id: string;
	original_name: string;
	url: string;
	width: number;
	height: number;
	format: string;
}

export type ProcessingStatus = 'queued' | 'processing' | 'retrying' | 'ready' | 'failed';

export interface MediaDetail extends MediaItem {
	status: ProcessingStatus;
	error: string | null;
	embedding_availability: 'available' | 'pending' | 'unavailable' | 'missing';
	size: number;
	tags: TagCount[];
	dates: MediaDate[];
	embeddings: MediaEmbedding[];
}

export interface MediaEmbedding {
	space: string;
	status: ProcessingStatus;
	error: string | null;
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

export interface MediaGridState {
	key: string;
	media: MediaPreviewItem[];
	nextCursor: string | null;
}
