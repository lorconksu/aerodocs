import React from 'react';

interface LogoProps extends React.SVGProps<SVGSVGElement> {
  /** Choose which layout to render */
  layout?: 'vertical' | 'horizontal';
}

export function Logo({ layout = 'horizontal', className = '', ...props }: LogoProps) {
  if (layout === 'vertical') {
    return (
      <svg 
        viewBox="0 0 120 140" 
        fill="none" 
        xmlns="http://www.w3.org/2000/svg"
        className={`w-24 h-auto ${className}`}
        {...props}
      >
        {/* The Main Document Shape (Inherits text color) */}
        <path d="M40 10L80 10L100 30L100 80L30 80L40 10Z" fill="currentColor" />
        <path d="M80 10L80 30L100 30" stroke="#1e293b" strokeWidth="4" strokeLinejoin="round"/>
        
        {/* The Aerodynamic Slants (Cyan Accent) */}
        <path d="M20 40L60 40L50 50L10 50Z" className="fill-cyan-500" />
        <path d="M15 55L65 55L55 65L5 65Z" className="fill-cyan-500" />
        <path d="M25 70L75 70L65 80L15 80Z" className="fill-cyan-500" />

        {/* The Text */}
        <text x="60" y="110" textAnchor="middle" className="font-bold text-2xl" fill="currentColor" fontFamily="sans-serif">AERO</text>
        <text x="60" y="135" textAnchor="middle" className="font-bold text-2xl" fill="currentColor" fontFamily="sans-serif">DOCS</text>
      </svg>
    );
  }

  // Horizontal Layout
  return (
    <svg 
      viewBox="0 0 200 60" 
      fill="none" 
      xmlns="http://www.w3.org/2000/svg"
      className={`w-40 h-auto ${className}`}
      {...props}
    >
      {/* The Main Document Shape */}
      <path d="M25 10L45 10L55 20L55 50L20 50L25 10Z" fill="currentColor" />
      <path d="M45 10L45 20L55 20" stroke="#1e293b" strokeWidth="2" strokeLinejoin="round"/>
      
      {/* The Aerodynamic Slants (Cyan Accent) */}
      <path d="M10 20L35 20L30 25L5 25Z" className="fill-cyan-500" />
      <path d="M5 30L35 30L30 35L0 35Z" className="fill-cyan-500" />
      <path d="M15 40L40 40L35 45L10 45Z" className="fill-cyan-500" />

      {/* The Text */}
      <text x="70" y="40" className="font-bold text-3xl" fill="currentColor" fontFamily="sans-serif">AeroDocs</text>
    </svg>
  );
}
