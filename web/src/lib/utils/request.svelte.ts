import { onDestroy } from 'svelte';

/** Component-scoped request state. Only the latest request may publish results. */
export function createRequest<T>() {
	let current = $state<T>();
	let loading = $state(false);
	let error = $state<Error>();
	let controller: AbortController | undefined;

	function cancel() {
		controller?.abort();
		controller = undefined;
		current = undefined;
		error = undefined;
		loading = false;
	}
	onDestroy(cancel);

	async function run(fetcher: (signal: AbortSignal) => Promise<T>, apply?: (value: T) => void) {
		cancel();
		const request = new AbortController();
		controller = request;
		loading = true;
		try {
			const value = await fetcher(request.signal);
			if (request.signal.aborted) return;
			current = value;
			apply?.(value);
		} catch (cause) {
			if (!request.signal.aborted)
				error = cause instanceof Error ? cause : new Error('Request failed');
		} finally {
			if (controller === request) {
				loading = false;
				controller = undefined;
			}
		}
	}

	return {
		get current() {
			return current;
		},
		get loading() {
			return loading;
		},
		get error() {
			return error;
		},
		clearError() {
			error = undefined;
		},
		run,
		cancel
	};
}
