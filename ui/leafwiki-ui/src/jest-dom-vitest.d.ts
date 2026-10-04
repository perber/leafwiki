// @testing-library/jest-dom's own vitest augmentation declares `Assertion<T>`,
// but vitest 5 changed it to `Assertion<R, T>`, so the upstream merge no longer
// applies. Re-declare it with vitest's signature until jest-dom catches up.
import type { TestingLibraryMatchers } from '@testing-library/jest-dom/matchers'

/* eslint-disable @typescript-eslint/no-empty-object-type, @typescript-eslint/no-unused-vars */
declare module 'vitest' {
  interface Assertion<
    R extends void | Promise<void> = void,
    T = unknown,
  > extends TestingLibraryMatchers<unknown, T> {}
  interface AsymmetricMatchersContaining extends TestingLibraryMatchers<
    unknown,
    unknown
  > {}
}
