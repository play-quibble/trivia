import type { Metadata } from 'next'
import { Fredoka } from 'next/font/google'
import './globals.css'
import Navbar from '@/components/Navbar'
import { getSession } from '@/lib/session'

// next/font/google downloads and self-hosts the font at build time.
// No external request is made by the browser — better performance and privacy
// than a <link> tag pointing at fonts.googleapis.com.
//
// variable: '--font-display' injects a CSS custom property onto the <html>
// element so Tailwind's font-display utility class can reference it.
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
  const session = getSession()

  return (
    // fredoka.variable attaches --font-display to the <html> element so every
    // descendant can use font-display via Tailwind.
    <html lang="en" className={fredoka.variable}>
      <body className="min-h-screen bg-gray-50 text-gray-900 antialiased">
        <Navbar session={session} />
        <main className="mx-auto max-w-5xl px-6 py-8">{children}</main>
      </body>
    </html>
  )
}
