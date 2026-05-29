import { fetchWithAuth } from './auth'

export type OAuthApprovalDetails = {
  clientLabel: string
  clientId: string
  redirectUri: string
  scope: string
  resource: string
}

export async function fetchOAuthApprovalDetails(
  approvalToken: string,
): Promise<OAuthApprovalDetails> {
  return (await fetchWithAuth(
    `/oauth/approval?approval_token=${encodeURIComponent(approvalToken)}`,
  )) as OAuthApprovalDetails
}
