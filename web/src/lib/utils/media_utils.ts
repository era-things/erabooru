export function isFormatVideo(format: string): boolean {
	return ['mp4', 'webm', 'avi', 'mkv'].includes(format.toLowerCase());
}

export function mediaBorderClass(format: string): string {
	return isFormatVideo(format) ? 'border-2 border-dashed border-neutral-900' : '';
}
