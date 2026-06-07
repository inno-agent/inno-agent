import { createContext, useEffect, useRef, useState } from 'react'
import { type UserManager } from 'oidc-client-ts'
import { createUserManager } from './authClient'

export interface AuthState {
    token: string | null
    userId: string | null
    userManager: UserManager | null
    loading: boolean
}

export const AuthContext = createContext<AuthState>({
    token: null,
    userId: null,
    userManager: null,
    loading: true,
})

export function AuthProvider({ children }: Readonly<{ children: React.ReactNode }>) {
    const [state, setState] = useState<AuthState>({
        token: null,
        userId: null,
        userManager: null,
        loading: true,
    })
    const initDone = useRef(false)

    useEffect(() => {
        if (initDone.current) return
        initDone.current = true

        const isCallback = window.location.pathname === '/callback'

        const storedToken = localStorage.getItem('aicore_token')
        const storedUserId = localStorage.getItem('aicore_user_id')

        if (storedToken && storedUserId && !isCallback) {
            setState((prev) => ({ ...prev, token: storedToken, userId: storedUserId, loading: false }))
            createUserManager().then((um) => setState((prev) => ({ ...prev, userManager: um })))
            return
        }

        createUserManager().then((um) => {
            setState((prev) => ({ ...prev, userManager: um }))
            if (!isCallback) {
                um.signinRedirect()
            } else {
                setState((prev) => ({ ...prev, loading: false }))
            }
        })
    }, [])

    return <AuthContext.Provider value={state}>{children}</AuthContext.Provider>
}
