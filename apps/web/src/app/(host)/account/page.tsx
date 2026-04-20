import { getMe } from '@/lib/api/user'
import AccountForm from '@/components/AccountForm'

export const metadata = { title: 'Account — Quibble' }

export default async function AccountPage() {
  const profile = await getMe()

  return (
    <main className="mx-auto max-w-lg px-4 py-10">
      <h1 className="mb-6 text-xl font-bold text-slate-800">Your account</h1>
      <AccountForm profile={profile} />
    </main>
  )
}
