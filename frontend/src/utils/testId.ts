/**
 * Conditional data-testid helper.
 *
 * Returns `{ 'data-testid': id }` in dev/test mode, empty object in production.
 * Spread the result onto any JSX element:
 *
 *   <div {...testId('player-username')}>
 *
 * Enabled when:
 *  - `import.meta.env.DEV` is true (Vite dev server, Vitest)
 *  - `import.meta.env.VITE_TEST_MODE === 'true'` (Docker test mode for Playwright)
 *
 * In production builds both are false — testids are stripped from the DOM.
 */
const enabled = import.meta.env.DEV || import.meta.env.VITE_TEST_MODE === 'true'

export function testId(id: string): { 'data-testid'?: string } {
  return enabled ? { 'data-testid': id } : {}
}
