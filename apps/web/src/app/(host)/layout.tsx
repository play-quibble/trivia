// Layout for host-facing authenticated pages: /banks, /games, /faq.
// Adds only the Navbar — each page's view component handles its own
// main container and padding so they can set their own max-width.
import Navbar from '@/components/Navbar'
import { getSession } from '@/lib/session'

export default function HostLayout({ children }: { children: React.ReactNode }) {
  const session = getSession()
  return (
    <>
      <Navbar session={session} />
      {children}
    </>
  )
}
