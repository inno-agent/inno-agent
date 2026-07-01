import { useEffect, useState } from 'react'
import { acceptGitFlameInvite, getLinkedGitFlameUsername, linkGitFlameUsername } from '@/api/consent'

type Status = 'checking' | 'idle' | 'loading' | 'linked' | 'taken' | 'error'
type InviteStatus = 'idle' | 'loading' | 'accepted' | 'error'

export default function OnboardingPage() {
    const [username, setUsername] = useState('')
    const [status, setStatus] = useState<Status>('checking')

    const [repoName, setRepoName] = useState('')
    const [inviteStatus, setInviteStatus] = useState<InviteStatus>('idle')

    // Restore onboarding state on mount so a page reload doesn't force
    // re-entering the username that's already linked server-side.
    useEffect(() => {
        let cancelled = false
        getLinkedGitFlameUsername()
            .then((linked) => {
                if (cancelled) return
                if (linked) {
                    setUsername(linked)
                    setStatus('linked')
                } else {
                    setStatus('idle')
                }
            })
            .catch(() => {
                if (!cancelled) setStatus('idle')
            })
        return () => {
            cancelled = true
        }
    }, [])

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

    async function acceptInvite() {
        const trimmed = repoName.trim()
        if (!trimmed) return
        setInviteStatus('loading')
        try {
            await acceptGitFlameInvite(trimmed)
            setInviteStatus('accepted')
        } catch {
            setInviteStatus('error')
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
                    disabled={status === 'checking' || status === 'loading' || status === 'linked'}
                />
            </div>

            {status !== 'linked' && (
                <button
                    onClick={submit}
                    disabled={status === 'checking' || status === 'loading' || !username.trim()}
                >
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

            <h1 style={{ marginTop: '40px' }}>Accept pending invite</h1>

            <p style={{ color: '#9a9a9a', fontSize: '14px', marginBottom: '24px' }}>
                Invited the bot as a collaborator on a repo? Confirm it here instead of
                logging into the bot's GitFlame account by hand.
                {status !== 'linked' && ' Link your GitFlame account above first.'}
            </p>

            <div className="field">
                <label htmlFor="gf-repo">Repository name</label>
                <input
                    id="gf-repo"
                    value={repoName}
                    onChange={(e) => setRepoName(e.target.value)}
                    placeholder="repo-name"
                    disabled={status !== 'linked' || inviteStatus === 'loading'}
                />
            </div>

            <button
                onClick={acceptInvite}
                disabled={status !== 'linked' || inviteStatus === 'loading' || !repoName.trim()}
            >
                {inviteStatus === 'loading' ? 'Accepting…' : 'Accept invite'}
            </button>

            {inviteStatus === 'accepted' && (
                <div className="result" style={{ marginTop: '16px' }}>
                    Invite accepted. The bot can now be assigned as a reviewer on this repo.
                </div>
            )}

            {inviteStatus === 'error' && (
                <div className="error">
                    Couldn't accept the invite. Check the repo name and that an invite is pending.
                </div>
            )}
        </div>
    )
}
