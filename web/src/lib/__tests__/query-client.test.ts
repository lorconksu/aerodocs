import { queryClient } from '../query-client'
import { QueryClient } from '@tanstack/react-query'

describe('queryClient', () => {
  it('is an instance of QueryClient', () => {
    expect(queryClient).toBeInstanceOf(QueryClient)
  })

  it('has retry set to 1', () => {
    const options = queryClient.getDefaultOptions()
    expect(options.queries?.retry).toBe(1)
  })

  it('has staleTime set to 30000', () => {
    const options = queryClient.getDefaultOptions()
    expect(options.queries?.staleTime).toBe(30_000)
  })

  it('has refetchOnWindowFocus disabled', () => {
    const options = queryClient.getDefaultOptions()
    expect(options.queries?.refetchOnWindowFocus).toBe(false)
  })
})
