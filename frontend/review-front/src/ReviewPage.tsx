import { useState } from 'react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { requestReview } from '@/api/review'

export default function ReviewPage() {
    const [prId, setPrId] = useState('')
    const [diff, setDiff] = useState('')
    const [review, setReview] = useState<string | null>(null)
    const [error, setError] = useState('')
    const [loading, setLoading] = useState(false)

    async function submit() {
        if (!prId.trim()) {
            setError('PR id is required')
            return
        }
        setError('')
        setReview(null)
        setLoading(true)
        try {
            const md = await requestReview({ prId: prId.trim(), diff: diff.trim() || undefined })
            setReview(md)
        } catch (e) {
            const resp = (e as { response?: { data?: { error?: string }; status?: number } }).response
            setError(resp?.data?.error ?? `Request failed${resp?.status ? ` (${resp.status})` : ''}`)
        } finally {
            setLoading(false)
        }
    }

    return (
        <>
            <h1>PR Reviewer</h1>

            <label htmlFor="prid">PR ID (owner/repo/index)</label>
            <input
                id="prid"
                value={prId}
                onChange={(e) => setPrId(e.target.value)}
                placeholder="my-org/backend/42"
            />

            <label htmlFor="diff">Diff (optional — leave empty to fetch from GitFlame)</label>
            <textarea
                id="diff"
                rows={10}
                value={diff}
                onChange={(e) => setDiff(e.target.value)}
                placeholder="diff --git a/main.go b/main.go..."
            />

            <button onClick={submit} disabled={loading}>
                {loading ? 'Generating…' : 'Generate Review'}
            </button>

            {error && <div className="error">{error}</div>}

            {review !== null && (
                <div className="result">
                    <Markdown remarkPlugins={[remarkGfm]}>{review}</Markdown>
                </div>
            )}
        </>
    )
}
