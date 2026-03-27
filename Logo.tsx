import React from 'react';

interface LogoProps extends React.SVGProps<SVGSVGElement> {
  /** Choose which layout to render */
  layout?: 'vertical' | 'horizontal';
}

export function Logo({ layout = 'horizontal', className = '', ...props }: LogoProps) {
  if (layout === 'vertical') {
    return (
      <svg 
        viewBox="0 0 160 210" 
        fill="none" 
        xmlns="http://www.w3.org/2000/svg"
        className={`w-28 h-auto ${className}`}
        {...props}
      >
        {/* Document Shape Outline (Slanted) */}
        <path d="M70 20 L110 20 L140 50 L120 130 L40 130 Z" stroke="currentColor" strokeWidth="8" strokeLinejoin="round" className="text-slate-400" />
        
        {/* Top Right Fold */}
        <path d="M110 20 L110 50 L140 50 Z" fill="currentColor" className="text-slate-400" />
        
        {/* Cyan Aero Slashes (Angled cutting upwards) */}
        <path d="M20 90 L80 50 L90 65 L30 105 Z" className="fill-cyan-500" />
        <path d="M10 115 L90 65 L100 80 L20 130 Z" className="fill-cyan-500" />
        <path d="M30 140 L100 95 L110 110 L40 155 Z" className="fill-cyan-500" />
        
        {/* Text */}
        <text x="80" y="175" textAnchor="middle" className="font-bold text-4xl tracking-wider" fill="currentColor" fontFamily="sans-serif">AERO</text>
        <text x="80" y="205" textAnchor="middle" className="font-bold text-4xl tracking-wider" fill="currentColor" fontFamily="sans-serif">DOCS</text>
      </svg>
    );
  }

  // Horizontal Layout
  return (
    <svg 
      viewBox="0 0 280 80" 
      fill="none" 
      xmlns="http://www.w3.org/2000/svg"
      className={`w-48 h-auto ${className}`}
      {...props}
    >
      {/* Document Shape Outline (Slanted) */}
      <path d="M35 15 L55 15 L70 30 L60 70 L20 70 Z" stroke="currentColor" strokeWidth="4" strokeLinejoin="round" className="text-slate-400" />
      
      {/* Top Right Fold */}
      <path d="M55 15 L55 30 L70 30 Z" fill="currentColor" className="text-slate-400" />
      
      {/* Cyan Aero Slashes (Angled cutting upwards) */}
      <path d="M10 50 L40 30 L45 37 L15 57 Z" className="fill-cyan-500" />
      <path d="M5 62 L45 37 L50 44 L10 69 Z" className="fill-cyan-500" />
      <path d="M15 74 L50 52 L55 59 L20 81 Z" className="fill-cyan-500" />
      
      {/* Text */}
      <text x="90" y="55" className="font-bold text-4xl tracking-wide" fill="currentColor" fontFamily="sans-serif">AeroDocs</text>
    </svg>
  );
}
