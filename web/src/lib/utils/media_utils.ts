export function isFormatVideo(format: string): boolean {
	return ['mp4', 'webm', 'avi', 'mkv'].includes(format.toLowerCase());
}
