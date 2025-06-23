<script lang="ts">
	import xxhash from "xxhash-wasm";

	let fileInput: HTMLInputElement | null = null;
	//const apiBase = import.meta.env.DEV ? 'http://localhost:8080' : '';
	const apiBase = 'http://localhost/api';
       export let supportedTypes: string[] = [
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

		const res = await fetch(`${apiBase}/media/upload-url`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ filename: uploadName })
		});
		if (!res.ok) {
			console.warn(`Failed to get upload URL: ${res.status} ${res.statusText}`);
			alert(`Failed to get upload URL from ${apiBase}/media/upload-url`);
			return;
		}

		const data: { url: string } = await res.json();
		try {
			console.log('Uploading to:', data.url);
			const up = await fetch(data.url, {
				method: 'PUT',
				body: file
			});
			if (!up.ok) {
				alert('Upload failed');
			} else {
				alert('Upload successful');
			}
		} catch (error) {
			console.error('Upload error:', error);
		}

		fileInput!.value = '';
	}

	function trigger() {
		fileInput?.click();
	}

	async function getUploadName(file: File): Promise<string> {
		const hasher = await xxhash();
		const arrayBuffer = await file.arrayBuffer();
		const uint8 = new Uint8Array(arrayBuffer);
		const hash = hasher.h64Raw(uint8).toString(16);

		//add file size to reduce chances of hash collision
		return hash + '_' + file.size + '.' + file.name.split('.').pop();
	}
</script>

<div
	class="cursor-pointer rounded border-2 border-dashed border-gray-300 p-8 text-center"
	on:click={trigger}
	on:keydown={(e) => e.key === 'Enter' && trigger()}
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
		on:change={upload}
	/>
</div>
