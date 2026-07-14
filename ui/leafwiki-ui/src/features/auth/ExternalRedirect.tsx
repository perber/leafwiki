import { useEffect } from 'react'
import { redirectToExternal } from '@/lib/redirectToExternal'

export default function ExternalRedirect({
  to,
  returnTo,
}: {
  to: string
  returnTo?: string
}) {
  useEffect(() => {
    redirectToExternal(to, returnTo)
  }, [to, returnTo])
  return null
}
