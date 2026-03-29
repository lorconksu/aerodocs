import { validatePassword } from '../password'

describe('validatePassword', () => {
  it('returns empty array for a strong password', () => {
    expect(validatePassword('StrongPass1@#$')).toEqual([])
  })

  it('requires at least 12 characters', () => {
    expect(validatePassword('Short1@')).toContain('At least 12 characters')
  })

  it('requires one uppercase letter', () => {
    expect(validatePassword('alllowercase1@#')).toContain('One uppercase letter')
  })

  it('requires one lowercase letter', () => {
    expect(validatePassword('ALLUPPERCASE1@#')).toContain('One lowercase letter')
  })

  it('requires one digit', () => {
    expect(validatePassword('NoDigitsHere@#$')).toContain('One digit')
  })

  it('requires one special character', () => {
    expect(validatePassword('NoSpecialChar123')).toContain('One special character')
  })

  it('returns multiple errors for a very weak password', () => {
    const errors = validatePassword('short')
    expect(errors.length).toBeGreaterThan(1)
  })
})
