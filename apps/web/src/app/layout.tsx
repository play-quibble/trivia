import type { Metadata } from 'next'
import { Fredoka } from 'next/font/google'
import './globals.css'

// next/font/google downloads and self-hosts the font at build time.
// variable: '--font-display' injects a CSS custom property so Tailwind's
// font-display utility class can reference it throughout the app.
const fredoka = Fredoka({
  subsets: ['latin'],
  weight: ['400', '600'],
  variable: '--font-display',
})

export const metadata: Metadata = {
  title: 'Quibble',
  description: 'Host live trivia games',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={fredoka.variable}>
      <body className="min-h-screen bg-slate-100 text-gray-900 antialiased">
        {children}
      </body>
    </html>
  )
}
