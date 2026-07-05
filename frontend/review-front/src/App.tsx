import { useEffect, useState } from 'react'
import { getToken, handleCallback, login } from '@/auth/auth'
import OnboardingPage from '@/OnboardingPage'
import ReviewPage from '@/ReviewPage'
import styles from '@/styles/App.module.scss'

type Tab = 'review' | 'onboarding'

export default function App() {
    const isCallback = window.location.pathname === '/callback'
    const [token] = useState(() => getToken())
    const [tab, setTab] = useState<Tab>('review')

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

    return (
        <>
            <nav className={styles.nav}>
                <button
                    onClick={() => setTab('review')}
                    className={`${styles['nav-button']} ${tab === 'review' ? styles['nav-button--active'] : styles['nav-button--inactive']}`}
                >
                    PR Review
                </button>
                <button
                    onClick={() => setTab('onboarding')}
                    className={`${styles['nav-button']} ${tab === 'onboarding' ? styles['nav-button--active'] : styles['nav-button--inactive']}`}
                >
                    Link account
                </button>
            </nav>

            {tab === 'review' ? <ReviewPage /> : <OnboardingPage />}
        </>
    )
}
