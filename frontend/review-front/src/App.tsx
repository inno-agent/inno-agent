import { useEffect, useState } from 'react'
import { getToken, handleCallback, login } from '@/auth/auth'
import OnboardingPage from '@/OnboardingPage'
import ReviewPage from '@/ReviewPage'

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
            <nav
                style={{
                    display: 'flex',
                    gap: '16px',
                    padding: '16px 20px 0',
                    maxWidth: '750px',
                    margin: '0 auto',
                }}
            >
                <button
                    onClick={() => setTab('review')}
                    style={{
                        background: 'none',
                        border: 'none',
                        padding: '4px 0',
                        marginTop: 0,
                        color: tab === 'review' ? '#f2f2f2' : '#7a7a7a',
                        fontSize: '13px',
                        fontWeight: tab === 'review' ? 600 : 400,
                        borderBottom: tab === 'review' ? '2px solid #ececec' : '2px solid transparent',
                        borderRadius: 0,
                        cursor: 'pointer',
                    }}
                >
                    PR Review
                </button>
                <button
                    onClick={() => setTab('onboarding')}
                    style={{
                        background: 'none',
                        border: 'none',
                        padding: '4px 0',
                        marginTop: 0,
                        color: tab === 'onboarding' ? '#f2f2f2' : '#7a7a7a',
                        fontSize: '13px',
                        fontWeight: tab === 'onboarding' ? 600 : 400,
                        borderBottom: tab === 'onboarding' ? '2px solid #ececec' : '2px solid transparent',
                        borderRadius: 0,
                        cursor: 'pointer',
                    }}
                >
                    Link account
                </button>
            </nav>

            {tab === 'review' ? <ReviewPage /> : <OnboardingPage />}
        </>
    )
}
