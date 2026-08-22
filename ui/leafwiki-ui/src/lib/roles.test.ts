import { describe, expect, it } from 'vitest'
import { hasRole } from './roles'

describe('hasRole', () => {
  it('returns true when the role is in the allowed list', () => {
    expect(hasRole('admin', ['admin', 'editor'])).toBe(true)
  })

  it('returns false when the role is not in the allowed list', () => {
    expect(hasRole('viewer', ['admin', 'editor'])).toBe(false)
  })

  it('treats an empty allowed list as "any authenticated user"', () => {
    expect(hasRole('admin', [])).toBe(true)
    expect(hasRole('viewer', [])).toBe(true)
  })

  it('returns false when role is undefined, even with an empty allowed list', () => {
    expect(hasRole(undefined, ['admin'])).toBe(false)
    expect(hasRole(undefined, [])).toBe(false)
  })
})
