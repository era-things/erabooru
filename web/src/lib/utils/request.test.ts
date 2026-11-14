import { afterEach, describe, expect, it, vi } from 'vitest';

const lifecycle = vi.hoisted(() => ({ cleanup: [] as (() => void)[] }));
vi.mock('svelte', () => ({ onDestroy: (fn: () => void) => lifecycle.cleanup.push(fn) }));
import { createRequest } from './request.svelte';

function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (error: Error) => void;
	const promise = new Promise<T>((done, fail) => {
		resolve = done;
		reject = fail;
	});
	return { promise, resolve, reject };
}

afterEach(() => lifecycle.cleanup.splice(0).forEach((cleanup) => cleanup()));

describe('component requests', () => {
	it('ignores stale success and keeps a newer request loading', async () => {
		const request = createRequest<string>();
		const old = deferred<string>();
		const latest = deferred<string>();
		const publish = vi.fn();
		let oldSignal!: AbortSignal;
		const first = request.run((signal) => {
			oldSignal = signal;
			return old.promise;
		}, publish);
		const second = request.run(() => latest.promise, publish);
		expect(oldSignal.aborted).toBe(true);
		old.resolve('stale');
		await first;
		expect(request.current).toBeUndefined();
		expect(request.loading).toBe(true);
		expect(publish).not.toHaveBeenCalled();
		latest.resolve('latest');
		await second;
		expect(request.current).toBe('latest');
		expect(request.loading).toBe(false);
		expect(publish).toHaveBeenCalledExactlyOnceWith('latest');
	});

	it('ignores stale errors and clears a current error on retry', async () => {
		const request = createRequest<string>();
		const old = deferred<string>();
		const latest = deferred<string>();
		const first = request.run(() => old.promise);
		const second = request.run(() => latest.promise);
		old.reject(new Error('stale failure'));
		await first;
		expect(request.error).toBeUndefined();
		expect(request.loading).toBe(true);
		latest.reject(new Error('current failure'));
		await second;
		expect(request.error?.message).toBe('current failure');
		await request.run(async () => 'retried');
		expect(request.error).toBeUndefined();
		expect(request.current).toBe('retried');
	});

	it('aborts on unmount and prevents a late result from being applied', async () => {
		const request = createRequest<string>();
		const pending = deferred<string>();
		const publish = vi.fn();
		let signal!: AbortSignal;
		const loading = request.run((value) => {
			signal = value;
			return pending.promise;
		}, publish);
		lifecycle.cleanup.at(-1)!();
		expect(signal.aborted).toBe(true);
		expect(request.loading).toBe(false);
		pending.resolve('late');
		await loading;
		expect(request.current).toBeUndefined();
		expect(publish).not.toHaveBeenCalled();
	});
});
