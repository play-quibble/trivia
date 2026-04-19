// Layout for public (player-facing) pages: /join and /play/[code].
// No Navbar — players don't have host accounts.
// The root layout handles <html>/<body>; this just passes children through.
export default function PublicLayout({ children }: { children: React.ReactNode }) {
  return <>{children}</>
}
