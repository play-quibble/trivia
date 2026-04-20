import Navbar from '@/components/Navbar'
import { getMe } from '@/lib/api/user'

export default async function HostLayout({ children }: { children: React.ReactNode }) {
  const profile = await getMe()
  return (
    <>
      <Navbar profile={profile} />
      {children}
    </>
  )
}
