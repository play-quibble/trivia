// Banks page — server component.
// Fetches the initial bank list from the real Go API on the server,
// then hands it to the client component that handles interactivity.
import { listBanks } from '@/lib/api/banks'
import BanksView from '@/components/BanksView'

export const metadata = { title: 'Question Banks — Quibble' }

export default async function BanksPage() {
  // listBanks calls the Go API with the DEV_AUTH_TOKEN from the environment.
  // The API derives the owner from the token, so no user ID is passed explicitly.
  const banks = await listBanks()
  return <BanksView banks={banks} />
}
