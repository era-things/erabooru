import type { MediaDetail, ProcessingStatus } from '../types/media';

export function isProcessing(status: ProcessingStatus): boolean {
	return status === 'queued' || status === 'processing' || status === 'retrying';
}

export function shouldPollMedia(detail: MediaDetail): boolean {
	if (detail.status === 'failed') return false;
	if (isProcessing(detail.status)) return true;
	if (detail.embedding_availability === 'unavailable') return false;
	return (
		detail.embedding_availability === 'pending' ||
		detail.embeddings.some((entry) => isProcessing(entry.status))
	);
}
