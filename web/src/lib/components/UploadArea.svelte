<script lang="ts">
	import { xxhash128 } from 'hash-wasm';

	let fileInput: HTMLInputElement | null = $state(null);
	import { requestUploadUrl, uploadToPresignedUrl } from '$lib/api';

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

	async function upload() {
		const file = fileInput?.files?.[0];
		if (!file) return;

		if (!supportedTypes.includes(file.type)) {
			alert('Unsupported file type');
			fileInput!.value = '';
			return;
		}

		const uploadName = await getUploadName(file);

		try {
			const url = await requestUploadUrl(uploadName);
			console.log('Uploading to:', url);
			const up = await uploadToPresignedUrl(url, file);
			if (up.ok) {
				alert('Upload successful');
			} else if (up.status === 412) {
				alert('File already exists (duplicate detected)');
			} else {
				alert(`Upload failed: ${up.status} ${up.statusText}`);
			}
		} catch (error) {
			console.error('Upload error:', error);
			alert('Upload failed due to network error');
		}

		fileInput!.value = '';
	}

	function trigger() {
		fileInput?.click();
	}

	async function getUploadName(file: File): Promise<string> {
		const arrayBuffer = await file.arrayBuffer();
		const uint8 = new Uint8Array(arrayBuffer);
		const hash = await xxhash128(uint8);
		return hash;
	}
</script>

<div
	class="cursor-pointer rounded border-2 border-dashed border-gray-300 p-8 text-center"
	onclick={trigger}
	onkeydown={(e) => e.key === 'Enter' && trigger()}
	role="button"
	aria-label="Upload file"
	tabindex="0"
>
	<p class="text-gray-500">Click to upload a file</p>
	<input
		type="file"
		accept={supportedTypes.join(', ')}
		class="hidden"
		bind:this={fileInput}
		onchange={upload}
	/>
</div>
