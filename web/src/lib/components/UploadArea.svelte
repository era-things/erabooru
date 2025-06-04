<script lang="ts">
       let fileInput: HTMLInputElement | null = null;
       const apiBase = import.meta.env.DEV ? 'http://localhost:8080' : '';

       async function upload() {
               const file = fileInput?.files?.[0];
               if (!file) return;

               if (file.type !== 'image/png') {
                       alert('Only PNG files are supported');
                       fileInput!.value = '';
                       return;
               }

               const res = await fetch(`${apiBase}/api/upload-url`, {
                       method: 'POST',
                       headers: { 'Content-Type': 'application/json' },
                       body: JSON.stringify({ filename: file.name })
               });
               if (!res.ok) {
                        console.warn(`Failed to get upload URL: ${res.status} ${res.statusText}`);
                       alert(`Failed to get upload URL from ${apiBase}/api/upload-url`);
                       return;
               }

               const data: { url: string } = await res.json();
               const up = await fetch(data.url, {
                       method: 'PUT',
                       headers: { 'Content-Type': 'image/png' },
                       body: file
               });
               if (!up.ok) {
                       alert('Upload failed');
               } else {
                       alert('Upload successful');
               }
               fileInput!.value = '';
       }

       function trigger() {
               fileInput?.click();
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
               accept="image/png"
               class="hidden"
               bind:this={fileInput}
               on:change={upload}
       />
</div>
