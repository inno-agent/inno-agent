import { useState } from 'react'
import { linkGitFlameUsername } from '@/api/consent'

type Status = 'idle' | 'loading' | 'linked' | 'taken' | 'error'

export default function OnboardingPage() {
    const [username, setUsername] = useState('')
    const [status, setStatus] = useState<Status>('idle')

    async function submit() {
        const trimmed = username.trim()
        if (!trimmed) return
        setStatus('loading')
        try {
            await linkGitFlameUsername(trimmed)
            setStatus('linked')
        } catch (e) {
            const code = (e as { response?: { status?: number } }).response?.status
            if (code === 409) {
                setStatus('taken')
            } else {
                setStatus('error')
            }
        }
    }

    return (
        <div className="page">
            <h1>Link GitFlame account</h1>

            <p style={{ color: '#9a9a9a', fontSize: '14px', marginBottom: '24px' }}>
                Enter your GitFlame username to allow the review bot to act on your behalf
                when you assign it as a reviewer.
            </p>

            <div className="field">
                <label htmlFor="gf-username">GitFlame username</label>
                <input
                    id="gf-username"
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    placeholder="your-gitflame-username"
                    disabled={status === 'loading' || status === 'linked'}
                />
            </div>

            {status !== 'linked' && (
                <button onClick={submit} disabled={status === 'loading' || !username.trim()}>
                    {status === 'loading' ? 'Linking…' : 'Link account'}
                </button>
            )}

            {status === 'linked' && (
                <div className="result" style={{ marginTop: '16px' }}>
                    Account linked. The bot can now review PRs on your behalf.
                </div>
            )}

            {status === 'taken' && (
                <div className="error">
                    This GitFlame username is already linked to another account.
                </div>
            )}

            {status === 'error' && (
                <div className="error">Something went wrong. Please try again.</div>
            )}
        </div>
    )
}
