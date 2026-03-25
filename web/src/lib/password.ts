/**
 * Validates password strength and returns a list of unmet requirements.
 * Returns an empty array if the password meets all requirements.
 */
export function validatePassword(pw: string): string[] {
  const errors: string[] = []
  if (pw.length < 12) errors.push('At least 12 characters')
  if (!/[A-Z]/.test(pw)) errors.push('One uppercase letter')
  if (!/[a-z]/.test(pw)) errors.push('One lowercase letter')
  if (!/\d/.test(pw)) errors.push('One digit')
  if (!/[^a-zA-Z0-9]/.test(pw)) errors.push('One special character')
  return errors
}
