import { useEffect, useState } from 'react'
import { getToken, handleCallback, login } from '@/auth/auth'
import ReviewPage from '@/ReviewPage'

export default function App() {
    const isCallback = window.location.pathname === '/callback'
    const [token] = useState(() => getToken())

    useEffect(() => {
        if (isCallback) {
            void handleCallback()
        } else if (!token) {
            void login()
        }
    }, [isCallback, token])

    if (isCallback) {
        return <div className="center">Logging in…</div>
    }
    if (!token) {
        return <div className="center">Redirecting to login…</div>
    }
    return <ReviewPage />
}
