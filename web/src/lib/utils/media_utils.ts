export function isFormatVideo(format: string): boolean {
	return ['mp4', 'webm', 'avi'].includes(format.toLowerCase());
}
