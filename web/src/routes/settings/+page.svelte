<script lang="ts">
	import TabNav from '$lib/components/TabNav.svelte';
	import { apiBase } from '$lib/config';

	async function regenerate() {
		if (!confirm('Are you sure you want to regenerate the reverse index?')) {
			return;
		}
		const res = await fetch(`${apiBase}/admin/regenerate`, { method: 'POST' });
		if (res.ok) {
			alert('Regeneration complete');
		} else {
			alert(`Failed to regenerate: ${res.status} ${res.statusText}`);
		}
	}
</script>

<TabNav active="settings" />
<div class="p-4">
	<button class="rounded border px-3 py-1" on:click={regenerate}> Regenerate reverse index </button>
</div>
