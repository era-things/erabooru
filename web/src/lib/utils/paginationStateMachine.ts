export type PaginationState = 'single' | 'first' | 'second' | 'middle' | 'secondLast' | 'last';

export type PaginationControl =
	| {
			kind: 'first' | 'prev' | 'next' | 'last';
			page: number;
	  }
	| {
			kind: 'page';
			page: number;
			current: boolean;
	  };

export interface PaginationContext {
	current: number;
	total: number;
}

const TRANSITIONS: Record<PaginationState, PaginationState[]> = {
	single: ['single', 'first', 'second', 'middle', 'secondLast', 'last'],
	first: ['single', 'first', 'second', 'middle', 'secondLast', 'last'],
	second: ['single', 'first', 'second', 'middle', 'secondLast', 'last'],
	middle: ['single', 'first', 'second', 'middle', 'secondLast', 'last'],
	secondLast: ['single', 'first', 'second', 'middle', 'secondLast', 'last'],
	last: ['single', 'first', 'second', 'middle', 'secondLast', 'last']
};

function resolveState(context: PaginationContext): PaginationState {
	const { current, total } = context;

	if (total <= 1) {
		return 'single';
	}

	if (current <= 1) {
		return 'first';
	}

	if (current === 2 && total > 2) {
		return 'second';
	}

	if (current >= total) {
		return 'last';
	}

	if (current === total - 1) {
		return 'secondLast';
	}

	return 'middle';
}

function normaliseContext({ current, total }: PaginationContext): PaginationContext {
	const safeTotal = Number.isFinite(total) && total > 0 ? Math.trunc(total) : 1;
	const safeCurrentRaw = Number.isFinite(current) ? Math.trunc(current) : 1;
	const safeCurrent = Math.min(Math.max(safeCurrentRaw, 1), safeTotal);

	return { current: safeCurrent, total: safeTotal };
}

export class PaginationStateMachine {
	#state: PaginationState = 'single';
	#context: PaginationContext = { current: 1, total: 1 };

	get state(): PaginationState {
		return this.#state;
	}

	get context(): PaginationContext {
		return this.#context;
	}

	compute(context: PaginationContext): PaginationControl[] {
		this.#context = normaliseContext(context);
		const nextState = resolveState(this.#context);

		if (!TRANSITIONS[this.#state].includes(nextState)) {
			throw new Error(`Invalid pagination transition: ${this.#state} -> ${nextState}`);
		}

		this.#state = nextState;

		return this.#buildControls();
	}

	#buildControls(): PaginationControl[] {
		const { current, total } = this.#context;

		if (total <= 1) {
			return [
				{
					kind: 'page',
					page: 1,
					current: true
				}
			];
		}

		const controls: PaginationControl[] = [];

		if (current > 1) {
			controls.push({ kind: 'first', page: 1 }, { kind: 'prev', page: current - 1 });
		}

		const start = Math.max(1, current - 2);
		const end = Math.min(total, current + 2);

		for (let page = start; page <= end; page += 1) {
			controls.push({ kind: 'page', page, current: page === current });
		}

		if (current < total) {
			controls.push({ kind: 'next', page: current + 1 }, { kind: 'last', page: total });
		}

		return controls;
	}
}
