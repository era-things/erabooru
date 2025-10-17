import { describe, expect, it } from 'vitest';
import { PaginationStateMachine } from './paginationStateMachine';

describe('PaginationStateMachine', () => {
	it('handles a single page list', () => {
		const machine = new PaginationStateMachine();
		const controls = machine.compute({ current: 1, total: 1 });

		expect(controls).toEqual([{ kind: 'page', page: 1, current: true }]);
		expect(machine.state).toBe('single');
	});

	it('navigates through a two page list', () => {
		const machine = new PaginationStateMachine();

		expect(machine.compute({ current: 1, total: 2 })).toEqual([
			{ kind: 'page', page: 1, current: true },
			{ kind: 'page', page: 2, current: false },
			{ kind: 'next', page: 2 },
			{ kind: 'last', page: 2 }
		]);
		expect(machine.state).toBe('first');

		expect(machine.compute({ current: 2, total: 2 })).toEqual([
			{ kind: 'first', page: 1 },
			{ kind: 'prev', page: 1 },
			{ kind: 'page', page: 1, current: false },
			{ kind: 'page', page: 2, current: true }
		]);
		expect(machine.state).toBe('last');
	});

	it('provides neighbours around the current page in the middle of a longer list', () => {
		const machine = new PaginationStateMachine();

		expect(machine.compute({ current: 3, total: 5 })).toEqual([
			{ kind: 'first', page: 1 },
			{ kind: 'prev', page: 2 },
			{ kind: 'page', page: 1, current: false },
			{ kind: 'page', page: 2, current: false },
			{ kind: 'page', page: 3, current: true },
			{ kind: 'page', page: 4, current: false },
			{ kind: 'page', page: 5, current: false },
			{ kind: 'next', page: 4 },
			{ kind: 'last', page: 5 }
		]);
		expect(machine.state).toBe('middle');
	});

	it('moves through the edges of a five page list', () => {
		const machine = new PaginationStateMachine();

		expect(machine.compute({ current: 1, total: 5 })).toEqual([
			{ kind: 'page', page: 1, current: true },
			{ kind: 'page', page: 2, current: false },
			{ kind: 'page', page: 3, current: false },
			{ kind: 'next', page: 2 },
			{ kind: 'last', page: 5 }
		]);
		expect(machine.state).toBe('first');

		expect(machine.compute({ current: 2, total: 5 })).toEqual([
			{ kind: 'first', page: 1 },
			{ kind: 'prev', page: 1 },
			{ kind: 'page', page: 1, current: false },
			{ kind: 'page', page: 2, current: true },
			{ kind: 'page', page: 3, current: false },
			{ kind: 'page', page: 4, current: false },
			{ kind: 'next', page: 3 },
			{ kind: 'last', page: 5 }
		]);
		expect(machine.state).toBe('second');

		expect(machine.compute({ current: 4, total: 5 })).toEqual([
			{ kind: 'first', page: 1 },
			{ kind: 'prev', page: 3 },
			{ kind: 'page', page: 2, current: false },
			{ kind: 'page', page: 3, current: false },
			{ kind: 'page', page: 4, current: true },
			{ kind: 'page', page: 5, current: false },
			{ kind: 'next', page: 5 },
			{ kind: 'last', page: 5 }
		]);
		expect(machine.state).toBe('secondLast');

		expect(machine.compute({ current: 5, total: 5 })).toEqual([
			{ kind: 'first', page: 1 },
			{ kind: 'prev', page: 4 },
			{ kind: 'page', page: 3, current: false },
			{ kind: 'page', page: 4, current: false },
			{ kind: 'page', page: 5, current: true }
		]);
		expect(machine.state).toBe('last');
	});
});
