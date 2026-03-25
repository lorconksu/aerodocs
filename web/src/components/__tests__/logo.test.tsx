import { render, screen } from '@testing-library/react'
import { Logo } from '../logo'

describe('Logo', () => {
  it('renders horizontal layout by default', () => {
    render(<Logo />)
    const img = screen.getByRole('img', { name: 'AeroDocs' })
    expect(img).toBeInTheDocument()
    expect(img).toHaveAttribute('src', '/aerodoc-horizontal.png')
    expect(img.className).toContain('w-28')
  })

  it('renders horizontal layout when layout="horizontal"', () => {
    render(<Logo layout="horizontal" />)
    const img = screen.getByRole('img', { name: 'AeroDocs' })
    expect(img).toHaveAttribute('src', '/aerodoc-horizontal.png')
    expect(img.className).toContain('w-28')
  })

  it('renders vertical layout when layout="vertical"', () => {
    render(<Logo layout="vertical" />)
    const img = screen.getByRole('img', { name: 'AeroDocs' })
    expect(img).toHaveAttribute('src', '/aerodoc-vertical.png')
    expect(img.className).toContain('w-20')
  })

  it('passes additional className for horizontal layout', () => {
    render(<Logo layout="horizontal" className="extra-class" />)
    const img = screen.getByRole('img', { name: 'AeroDocs' })
    expect(img.className).toContain('extra-class')
  })

  it('passes additional className for vertical layout', () => {
    render(<Logo layout="vertical" className="w-40" />)
    const img = screen.getByRole('img', { name: 'AeroDocs' })
    expect(img.className).toContain('w-40')
  })

  it('has alt text "AeroDocs"', () => {
    render(<Logo />)
    expect(screen.getByAltText('AeroDocs')).toBeInTheDocument()
  })
})
