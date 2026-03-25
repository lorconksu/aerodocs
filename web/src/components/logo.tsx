interface LogoProps {
  layout?: 'vertical' | 'horizontal'
  className?: string
}

export function Logo({ layout = 'horizontal', className = '' }: Readonly<LogoProps>) {
  if (layout === 'vertical') {
    return (
      <img
        src="/aerodoc-vertical.png"
        alt="AeroDocs"
        className={`w-20 h-auto ${className}`}
      />
    )
  }

  return (
    <img
      src="/aerodoc-horizontal.png"
      alt="AeroDocs"
      className={`w-28 h-auto ${className}`}
    />
  )
}
