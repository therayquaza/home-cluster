import * as SecureStore from 'expo-secure-store'
import * as AuthSession from 'expo-auth-session'
import * as WebBrowser from 'expo-web-browser'
WebBrowser.maybeCompleteAuthSession()
const key = 'dinks_access_token'
const issuer = process.env.EXPO_PUBLIC_OIDC_ISSUER ?? 'https://keycloak.internal.rayq.app/realms/home'
const clientId = process.env.EXPO_PUBLIC_OIDC_CLIENT_ID ?? 'dinks-mobile'
export const token = () => SecureStore.getItemAsync(key)
export async function signIn() {
  const discovery = await AuthSession.fetchDiscoveryAsync(issuer)
  const redirectUri = AuthSession.makeRedirectUri({ scheme: 'dinks' })
  const request = new AuthSession.AuthRequest({ clientId, redirectUri, responseType: AuthSession.ResponseType.Code, scopes: ['openid', 'profile', 'email'], usePKCE: true })
  await request.makeAuthUrlAsync(discovery)
  const response = await request.promptAsync(discovery)
  if (response.type !== 'success') return false
  const tokens = await AuthSession.exchangeCodeAsync({ clientId, code: response.params.code, redirectUri, extraParams: { code_verifier: request.codeVerifier ?? '' } }, discovery)
  if (!tokens.accessToken) throw new Error('No access token returned')
  await SecureStore.setItemAsync(key, tokens.accessToken)
  return true
}
export const signOut = () => SecureStore.deleteItemAsync(key)
