import type { Config } from 'tailwindcss'

const config: Config = {
  // Tell Tailwind which files to scan for class names.
  // It removes any classes not found here from the production build (tree-shaking).
  content: [
    './src/pages/**/*.{js,ts,jsx,tsx,mdx}',
    './src/components/**/*.{js,ts,jsx,tsx,mdx}',
    './src/app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {
      colors: {
        brand: {
          blue: '#00338D',
          red: '#C60C30',
        },
      },
      fontFamily: {
        // Consumed via CSS variable set by next/font in layout.tsx.
        // Use with className="font-display" anywhere in the app.
        display: ['var(--font-display)'],
      },
    },
  },
  plugins: [],
}

export default config
