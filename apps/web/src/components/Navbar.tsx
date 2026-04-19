import Link from 'next/link'
import type { Session } from '@/types'

interface NavbarProps {
  session: Session
}

export default function Navbar({ session }: NavbarProps) {
  return (
    <header className="bg-brand-blue shadow-lg">
      <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">

        {/* App name — left side, Fredoka font, "bb" italicised */}
        <div className="flex items-center gap-6">
          <Link href="/banks" className="font-display text-2xl font-semibold tracking-tight text-white">
            Qui<span className="italic">bb</span>le
          </Link>
          <Link href="/faq" className="text-sm text-white/70 hover:text-white">
            FAQ
          </Link>
        </div>

        {/* User avatar + email — right side */}
        <div className="flex items-center gap-3">
          <span className="hidden text-sm text-white/70 sm:block">{session.email}</span>
          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-white text-xs font-semibold text-brand-blue">
            {session.name
              .split(' ')
              .map((w) => w[0])
              .join('')
              .toUpperCase()
              .slice(0, 2)}
          </div>
        </div>

      </div>
    </header>
  )
}
