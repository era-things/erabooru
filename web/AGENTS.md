# AGENTS Instructions for `web`

## Scope
These instructions apply to all files in the `web/` directory.

## Svelte Runes
- This project uses **Svelte 5** with the *runes* syntax. Always write components using runes like `$state`, `$derived`, `$effect`, etc.
- Do **not** use the old reactive `$:` syntax or `$store` auto-subscriptions.

### Runes cheatsheet
- `$state(initial)` – declare reactive state.
  ```svelte
  let count = $state(0);
  ```
- `$derived(expr)` – create derived state from other values.
  ```svelte
  let doubled = $derived(count * 2);
  ```
- `$derived.by(fn)` – use when the derivation needs a callback body.
  ```svelte
  let total = $derived.by(() => items.reduce((s, i) => s + i, 0));
  ```
- `$effect(fn)` – run a function whenever its dependencies change.
  ```svelte
  $effect(() => console.log(count));
  ```
- `$props()` – access component props.
  ```svelte
  let { foo } = $props();
  ```
- `$bindable()` – mark a prop as bindable for two‑way binding.
  ```svelte
  let { value = $bindable() } = $props();
  ```
- `$inspect(...vals)` – log values reactively during development.
  ```svelte
  $inspect(count);
  ```
- `$host()` – reference the custom element host when compiling as a custom element.
  ```svelte
  $host().dispatchEvent(new CustomEvent('hello'));
  ```

## `Runed` library
This project have Runed installed. Runed is handy library with rune-based helper functions. Here is names of those function, whole descriptions can be found [here](https://runed.dev/docs)
#### ELEMENTS
- `activeElement`
- `ElementRect`
- `ElementSize`
- `IsFocusWithin`
- `IsInViewport`
- `ScrollState`
- `TextareaAutosize`
- `useIntersectionObserver`
- `useMutationObserver`
- `useResizeObserver`

#### BROWSER
- `useEventListener`

#### SENSORS
- `IsIdle`
- `onClickOutside`
- `PressedKeys`
- `useGeolocation`

#### ANIMATION
- `AnimationFrames`

#### UTILITIES
- `useDebounce`

#### REACTIVITY
- `extract`
- `resource`
- `watch`

#### STATE
- `Context`
- `Debounced`
- `FiniteStateMachine`
- `PersistedState`
- `Previous`
- `StateHistory`

#### COMPONENT
- `IsMounted`

## Formatting
Run `npm run lint` inside this folder before committing frontend changes to ensure Prettier and ESLint pass.
