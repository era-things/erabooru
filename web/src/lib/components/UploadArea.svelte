<script lang="ts">
	import { xxhash128 } from 'hash-wasm';

	let fileInput: HTMLInputElement | null = $state(null);
	import { apiBase } from '$lib/config';

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

		const res = await fetch(`/api/media/upload-url`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ filename: uploadName })
		});
		if (!res.ok) {
			console.warn(`Failed to get upload URL: ${res.status} ${res.statusText}`);
			alert(`Failed to get upload URL from /api/media/upload-url`);
			return;
		}

		const data: { url: string } = await res.json();
		try {
			console.log('Uploading to:', data.url);
			const up = await fetch(data.url, {
				method: 'PUT',
				body: file,
				headers: {
					'Content-Type': file.type || 'application/octet-stream',
					'If-None-Match': '*' 
				}
			});
            if (up.ok) {
                alert('Upload successful');
            } else if (up.status === 412) {
                // 412 Precondition Failed = duplicate file
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
