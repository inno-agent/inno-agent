import { createUserManager } from '@/auth/authClient'

const TOKEN_KEY = 'aicore_token'
const USER_ID_KEY = 'aicore_user_id'

export function getToken(): string | null {
    return localStorage.getItem(TOKEN_KEY)
}

export function logout(): void {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(USER_ID_KEY)
}

// login redirects the browser to the IdP. Never returns.
export async function login(): Promise<void> {
    const um = await createUserManager()
    await um.signinRedirect()
}

// handleCallback completes the OIDC flow on /callback: exchange the id_token for
// an aicore_token, persist it, then send the user back to the app root.
export async function handleCallback(): Promise<void> {
    try {
        const um = await createUserManager()
        const user = await um.signinRedirectCallback()

        const resp = await fetch('/identity/v1/exchange', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ token: user.id_token }),
        })
        if (!resp.ok) throw new Error(`exchange failed: ${resp.status}`)
        const { access_token } = (await resp.json()) as { access_token: string }

        const [, payloadB64] = access_token.split('.')
        const payload = JSON.parse(
            atob(payloadB64.replace(/-/g, '+').replace(/_/g, '/')),
        ) as { sub: string }

        localStorage.setItem(TOKEN_KEY, access_token)
        localStorage.setItem(USER_ID_KEY, payload.sub)
    } catch (err) {
        console.error('Auth callback failed:', err)
    } finally {
        window.location.replace('/')
    }
}
