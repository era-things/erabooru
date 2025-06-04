<script lang="ts">
</script>

<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
	{#each Array.from({ length: 6 }, (_, i) => i) as i (i)}
		<div class="flex aspect-square items-center justify-center bg-gray-200 text-gray-500">
			Placeholder {i + 1}
		</div>
	{/each}
</div>
