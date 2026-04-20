import Link from 'next/link'
import type { UserProfile } from '@/types'

interface NavbarProps {
  profile: UserProfile
}

// Derive initials from display_name if set, otherwise from email.
function initials(profile: UserProfile): string {
  if (profile.display_name) {
    return profile.display_name
      .split(' ')
      .map((w) => w[0])
      .join('')
      .toUpperCase()
      .slice(0, 2)
  }
  return (profile.email ?? '?')[0].toUpperCase()
}

export default function Navbar({ profile }: NavbarProps) {
  const label = profile.display_name ?? profile.email ?? 'Account'

  return (
    <header className="bg-brand-blue shadow-lg">
      <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">

        {/* Left nav */}
        <div className="flex items-center gap-6">
          <Link href="/banks" className="font-display text-2xl font-semibold tracking-tight text-white">
            Qui<span className="italic">bb</span>le
          </Link>
          <Link href="/banks" className="text-sm text-white/70 hover:text-white">
            Banks
          </Link>
          <Link href="/quizzes" className="text-sm text-white/70 hover:text-white">
            Quizzes
          </Link>
          <Link href="/games" className="text-sm text-white/70 hover:text-white">
            Games
          </Link>
          <Link href="/faq" className="text-sm text-white/70 hover:text-white">
            FAQ
          </Link>
        </div>

        {/* Right — avatar links to /account */}
        <Link href="/account" className="flex items-center gap-3 group">
          <span className="hidden text-sm text-white/70 group-hover:text-white sm:block transition-colors">
            {label}
          </span>
          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-white text-xs font-semibold text-brand-blue group-hover:bg-white/90 transition-colors">
            {initials(profile)}
          </div>
        </Link>

      </div>
    </header>
  )
}
