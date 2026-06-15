import { UserManager, type UserManagerSettings } from 'oidc-client-ts'

export async function createUserManager(): Promise<UserManager> {
    const resp = await fetch('/identity/v1/config')
    const { authority, client_id } = await resp.json()

    // oidc-client-ts discovers authorize/token/jwks from
    // `${authority}/.well-known/openid-configuration` (served by authentik).
    // The browser talks to authentik directly; no identity-side proxy.
    const settings: UserManagerSettings = {
        authority,
        client_id,
        redirect_uri: window.location.origin + '/callback',
        scope: 'openid email profile',
        response_type: 'code',
    }

    return new UserManager(settings)
}
