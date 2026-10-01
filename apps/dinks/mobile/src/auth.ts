import * as SecureStore from 'expo-secure-store'
import * as AuthSession from 'expo-auth-session'
import * as WebBrowser from 'expo-web-browser'

WebBrowser.maybeCompleteAuthSession()

const TOKEN_KEY = 'dinks_access_token'

const issuer = process.env.EXPO_PUBLIC_OIDC_ISSUER ?? 'https://keycloak.internal.rayq.app/realms/home'
const clientId = process.env.EXPO_PUBLIC_OIDC_CLIENT_ID ?? 'dinks-mobile'

/**
 * Keycloak matches the incoming redirect_uri against the client's registered
 * "Valid Redirect URIs". `makeRedirectUri({ scheme })` on its own produces the
 * hostless `dinks://`, which can never equal a registered entry like
 * `dinks://oauthredirect`, so the authorization request is rejected with
 * "Invalid parameter: redirect_uri". Naming the path yields the concrete URI
 * that the `dinks-mobile` client must have registered.
 */
export const redirectUri = AuthSession.makeRedirectUri({ scheme: 'dinks', path: 'oauthredirect' })

/** The OIDC access token, when the member signed in with Keycloak. */
export const token = () => SecureStore.getItemAsync(TOKEN_KEY)

/**
 * Keycloak authorization-code + PKCE, opened in the system browser. Returns
 * false when the member backs out, so the caller can stay on the sign-in screen
 * without treating a cancellation as an error.
 */
export async function signInWithKeycloak(): Promise<boolean> {
  const discovery = await AuthSession.fetchDiscoveryAsync(issuer)
  const request = new AuthSession.AuthRequest({
    clientId,
    redirectUri,
    responseType: AuthSession.ResponseType.Code,
    scopes: ['openid', 'profile', 'email'],
    usePKCE: true,
  })
  await request.makeAuthUrlAsync(discovery)
  const response = await request.promptAsync(discovery)
  if (response.type === 'cancel' || response.type === 'dismiss') return false
  if (response.type !== 'success') throw new Error('Sign in was not completed')
  const codeVerifier = request.codeVerifier
  if (!codeVerifier) throw new Error('Missing PKCE code verifier')
  const tokens = await AuthSession.exchangeCodeAsync(
    { clientId, code: response.params.code, redirectUri, extraParams: { code_verifier: codeVerifier } },
    discovery,
  )
  if (!tokens.accessToken) throw new Error('No access token returned')
  await SecureStore.setItemAsync(TOKEN_KEY, tokens.accessToken)
  return true
}

/** Clears the Keycloak token. The session cookie is cleared by POST /auth/logout. */
export const signOut = () => SecureStore.deleteItemAsync(TOKEN_KEY)
