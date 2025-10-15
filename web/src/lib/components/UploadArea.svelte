<script lang="ts">
	import { onDestroy } from 'svelte';
	import { xxhash128 } from 'hash-wasm';
	import MediaGrid from '$lib/components/MediaGrid.svelte';
	import type { MediaPreviewItem } from '$lib/types/media';
	import { requestUploadUrl, uploadToPresignedUrl } from '$lib/api';

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

	async function upload() {
		const files = Array.from(fileInput?.files ?? []);
		if (!files.length) return;

		uploading = true;

		for (const file of files) {
			if (!supportedTypes.includes(file.type)) {
				alert(`Unsupported file type: ${file.type || file.name}`);
				continue;
			}

			const uploadName = await getUploadName(file);

			try {
				const url = await requestUploadUrl(uploadName);
				const up = await uploadToPresignedUrl(url, file);

				if (up.ok) {
					const preview = await createPreview(file, uploadName);
					uploaded = [
						...uploaded,
						{
							id: uploadName,
							name: file.name,
							size: file.size,
							type: file.type,
							preview,
							objectUrl: file.type.startsWith('image/') ? preview.url : undefined
						}
					];
				} else if (up.status === 412) {
					alert(`File already exists: ${file.name}`);
				} else {
					alert(`Upload failed for ${file.name}: ${up.status} ${up.statusText}`);
				}
			} catch (error) {
				console.error('Upload error:', error);
				alert(`Upload failed for ${file.name} due to network error`);
			}
		}

		if (fileInput) {
			fileInput.value = '';
		}
		uploading = false;
	}

	function trigger() {
		fileInput?.click();
	}

	async function getUploadName(file: File): Promise<string> {
		const arrayBuffer = await file.arrayBuffer();
		const uint8 = new Uint8Array(arrayBuffer);
		return xxhash128(uint8);
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
			image.onload = () => {
				resolve({ width: image.naturalWidth || 1, height: image.naturalHeight || 1 });
			};
			image.onerror = (error) => {
				console.error('Failed to read image dimensions', error);
				resolve({ width: 512, height: 512 });
			};
			image.src = url;
		});
	}

	async function createVideoPreview(file: File, id: string): Promise<MediaPreviewItem> {
		const objectUrl = URL.createObjectURL(file);
		const { width, height } = await readVideoDimensions(objectUrl);
		URL.revokeObjectURL(objectUrl);

		const format = detectFormat(file);
		const placeholder = buildVideoPlaceholder(format);
		const displayHeight = Math.min(height, width * 3);

		return {
			id,
			url: placeholder,
			width,
			height,
			format,
			displayHeight,
			originalHeight: height,
			cropped: displayHeight < height
		} satisfies MediaPreviewItem;
	}

	async function readVideoDimensions(url: string): Promise<{ width: number; height: number }> {
		return new Promise((resolve) => {
			const video = document.createElement('video');
			video.preload = 'metadata';
			video.muted = true;
			video.src = url;
			video.onloadedmetadata = () => {
				const width = video.videoWidth || 1280;
				const height = video.videoHeight || 720;
				resolve({ width, height });
			};
			video.onerror = (error) => {
				console.error('Failed to read video metadata', error);
				resolve({ width: 1280, height: 720 });
			};
		});
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
