<script lang="ts">
	import { onDestroy } from 'svelte';
	import MediaGrid from '$lib/components/MediaGrid.svelte';
	import type { MediaPreviewItem } from '$lib/types/media';
	import { uploadMedia } from '$lib/api';

	interface UploadedItem {
		id: string;
		name: string;
		size: number;
		type: string;
		preview: MediaPreviewItem;
		objectUrl?: string;
	}

	const supportedTypes: string[] = [
		'image/png',
		'image/jpeg',
		'image/jpg',
		'image/gif',
		'video/mp4',
		'video/webm',
		'video/x-msvideo',
		'video/x-matroska'
	];

	let fileInput: HTMLInputElement | null = $state(null);
	let uploaded = $state<UploadedItem[]>([]);
	let uploading = $state(false);

	const uploadedCount = $derived(uploaded.length);
	const totalBytes = $derived(uploaded.reduce((sum, item) => sum + item.size, 0));
	const totalMegabytes = $derived(totalBytes / (1024 * 1024));
	const previews = $derived(uploaded.map((item) => item.preview));

	let destroyed = false;

	const previewQueue: { file: File; id: string }[] = [];
	let previewWorker: Promise<void> | undefined;
	function startPreviews() {
		if (previewWorker) return;
		previewWorker = (async () => {
			while (previewQueue.length && !destroyed) {
				const item = previewQueue.shift()!;
				try {
					const preview = await createPreview(item.file, item.id);
					if (destroyed) {
						if (preview.url.startsWith('blob:')) URL.revokeObjectURL(preview.url);
						break;
					}
					uploaded = uploaded.map((entry) =>
						entry.id === item.id
							? {
									...entry,
									preview,
									objectUrl: preview.url.startsWith('blob:') ? preview.url : undefined
								}
							: entry
					);
				} catch (error) {
					console.error('Preview failed', error);
				}
			}
		})().finally(() => {
			previewWorker = undefined;
			if (previewQueue.length && !destroyed) startPreviews();
		});
	}

	async function upload() {
		if (uploading) return;
		const files = Array.from(fileInput?.files ?? []);
		if (!files.length) return;
		uploading = true;
		if (fileInput) fileInput.value = '';
		const failures: string[] = [];
		const skipped: string[] = [];
		let next = 0;
		async function uploadNext() {
			while (next < files.length && !destroyed) {
				const file = files[next++];
				if (!supportedTypes.includes(file.type)) {
					failures.push(`${file.name}: unsupported file type`);
					continue;
				}
				try {
					const { id, existed } = await uploadMedia(file);
					if (destroyed) return;
					if (existed) {
						skipped.push(file.name);
						continue;
					}
					uploaded = [
						...uploaded,
						{
							id,
							name: file.name,
							size: file.size,
							type: file.type,
							preview: createFallbackPreview(file, id)
						}
					];
					previewQueue.push({ file, id });
					startPreviews();
				} catch (error) {
					console.error('Upload failed', error);
					failures.push(file.name);
				}
			}
		}
		try {
			await Promise.all(Array.from({ length: Math.min(3, files.length) }, uploadNext));
			// Uploads finish independently of local preview decoding.
			if (!destroyed && (failures.length || skipped.length)) {
				alert(
					[
						failures.length
							? `Failed (${failures.length}): ${failures.slice(0, 10).join(', ')}`
							: '',
						skipped.length ? `Skipped ${skipped.length} existing files.` : ''
					]
						.filter(Boolean)
						.join('\n')
				);
			}
		} finally {
			uploading = false;
		}
	}

	function trigger() {
		fileInput?.click();
	}

	async function createPreview(file: File, id: string): Promise<MediaPreviewItem> {
		if (file.type.startsWith('image/')) {
			return createImagePreview(file, id);
		}

		if (file.type.startsWith('video/')) {
			return createVideoPreview(file, id);
		}

		return createFallbackPreview(file, id);
	}

	async function createImagePreview(file: File, id: string): Promise<MediaPreviewItem> {
		const objectUrl = URL.createObjectURL(file);
		const { width, height } = await readImageDimensions(objectUrl);
		const displayHeight = Math.min(height, width * 3);

		return {
			id,
			url: objectUrl,
			width,
			height,
			format: detectFormat(file),
			displayHeight,
			originalHeight: height,
			cropped: displayHeight < height
		} satisfies MediaPreviewItem;
	}

	async function readImageDimensions(url: string): Promise<{ width: number; height: number }> {
		return new Promise((resolve) => {
			const image = new Image();
			const timeout = window.setTimeout(() => {
				image.src = '';
				resolve({ width: 512, height: 512 });
			}, 10000);
			image.onload = () => {
				clearTimeout(timeout);
				resolve({ width: image.naturalWidth || 1, height: image.naturalHeight || 1 });
			};
			image.onerror = (error) => {
				clearTimeout(timeout);
				console.error('Failed to read image dimensions', error);
				resolve({ width: 512, height: 512 });
			};
			image.src = url;
		});
	}

	async function createVideoPreview(file: File, id: string): Promise<MediaPreviewItem> {
		const format = detectFormat(file);
		const sourceUrl = URL.createObjectURL(file);
		let width = 1280;
		let height = 720;
		let url = buildVideoPlaceholder(format);

		try {
			const frame = await captureVideoFrame(sourceUrl);
			width = frame.width;
			height = frame.height;
			url = frame.url;
		} catch (error) {
			console.error('Failed to create video preview', error);
		} finally {
			URL.revokeObjectURL(sourceUrl);
		}

		const displayHeight = Math.min(height, width * 3);

		return {
			id,
			url,
			width,
			height,
			format,
			displayHeight,
			originalHeight: height,
			cropped: displayHeight < height
		} satisfies MediaPreviewItem;
	}

	async function captureVideoFrame(
		sourceUrl: string
	): Promise<{ url: string; width: number; height: number }> {
		const video = await new Promise<HTMLVideoElement>((resolve, reject) => {
			const timeout = window.setTimeout(() => {
				video.removeAttribute('src');
				video.load();
				reject(new Error('Video preview timed out'));
			}, 10000);
			const finish = (value: HTMLVideoElement) => {
				clearTimeout(timeout);
				resolve(value);
			};
			const video = document.createElement('video');
			video.preload = 'auto';
			video.muted = true;
			video.playsInline = true;
			video.src = sourceUrl;
			video.onloadeddata = () => {
				const seekTo = Math.min(Number.isFinite(video.duration) ? video.duration / 2 : 0, 2);
				if (seekTo > 0.05) {
					video.onseeked = () => finish(video);
					video.currentTime = seekTo;
				} else {
					finish(video);
				}
			};
			video.onerror = () => {
				clearTimeout(timeout);
				reject(new Error('Browser could not decode the video'));
			};
		});

		const width = video.videoWidth || 1280;
		const height = video.videoHeight || 720;
		const scale = Math.min(1, 640 / width);
		const canvas = document.createElement('canvas');
		canvas.width = Math.max(1, Math.round(width * scale));
		canvas.height = Math.max(1, Math.round(height * scale));

		const context = canvas.getContext('2d');
		if (!context) throw new Error('Canvas is unavailable');
		context.drawImage(video, 0, 0, canvas.width, canvas.height);

		const blob = await new Promise<Blob>((resolve, reject) => {
			canvas.toBlob(
				(value) => (value ? resolve(value) : reject(new Error('Video frame encoding failed'))),
				'image/jpeg',
				0.85
			);
		});

		video.removeAttribute('src');
		video.load();
		return { url: URL.createObjectURL(blob), width, height };
	}

	function createFallbackPreview(file: File, id: string): MediaPreviewItem {
		const size = 512;
		return {
			id,
			url: buildGenericPlaceholder(file.name),
			width: size,
			height: size,
			format: detectFormat(file),
			displayHeight: size,
			originalHeight: size,
			cropped: false
		} satisfies MediaPreviewItem;
	}

	function detectFormat(file: File): string {
		const extension = file.name.split('.').pop();
		if (extension) return extension.toLowerCase();
		const [type, subtype] = file.type.split('/');
		return (subtype ?? type ?? 'file').toLowerCase();
	}

	function buildVideoPlaceholder(format: string): string {
		const label = format.toUpperCase();
		const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360" viewBox="0 0 640 360">
        <defs>
                <linearGradient id="grad" x1="0%" y1="0%" x2="100%" y2="100%">
                        <stop offset="0%" stop-color="#111827" />
                        <stop offset="100%" stop-color="#1f2937" />
                </linearGradient>
        </defs>
        <rect width="640" height="360" rx="24" fill="url(#grad)" />
        <polygon points="270,180 370,240 370,120" fill="#f97316" />
        <text x="50%" y="75%" dominant-baseline="middle" text-anchor="middle" font-family="Arial" font-size="48" fill="#f9fafb">Video (${label})</text>
        </svg>`;
		return `data:image/svg+xml,${encodeURIComponent(svg)}`;
	}

	function buildGenericPlaceholder(name: string): string {
		const truncated = name.length > 24 ? `${name.slice(0, 21)}...` : name;
		const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 512 512">
        <rect width="512" height="512" rx="32" fill="#111827" />
        <text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle" font-family="Arial" font-size="48" fill="#f9fafb">${truncated}</text>
        </svg>`;
		return `data:image/svg+xml,${encodeURIComponent(svg)}`;
	}

	onDestroy(() => {
		destroyed = true;
		for (const item of uploaded) {
			if (item.objectUrl) {
				URL.revokeObjectURL(item.objectUrl);
			}
		}
	});
</script>

<div
	class="cursor-pointer rounded border-2 border-dashed border-gray-300 p-8 text-center"
	onclick={trigger}
	onkeydown={(e) => e.key === 'Enter' && trigger()}
	role="button"
	aria-label="Upload files"
	tabindex="0"
>
	<p class="text-gray-500">{uploading ? 'Uploading…' : 'Click to upload files'}</p>
	<input
		type="file"
		accept={supportedTypes.join(', ')}
		class="hidden"
		bind:this={fileInput}
		multiple
		onchange={upload}
	/>
</div>

{#if uploadedCount > 0}
	<div class="mt-6 space-y-4">
		<div class="rounded border border-gray-200 bg-white p-4 shadow-sm">
			<p class="font-medium text-gray-700">
				Uploaded {uploadedCount}
				{uploadedCount === 1 ? 'file' : 'files'} · {totalMegabytes.toFixed(2)} MB total
			</p>
		</div>
		<MediaGrid items={previews} />
		<div class="rounded border border-gray-200 bg-white p-4 shadow-sm">
			<h3 class="mb-2 font-semibold text-gray-800">Upload history</h3>
			<ul class="space-y-1 text-sm text-gray-600">
				{#each uploaded as item (item.id)}
					<li class="flex flex-wrap justify-between gap-2">
						<span class="font-medium text-gray-800">{item.name}</span>
						<span>{(item.size / (1024 * 1024)).toFixed(2)} MB</span>
					</li>
				{/each}
			</ul>
		</div>
	</div>
{/if}
