import { useEffect, useState } from 'react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { requestReview } from '@/api/review'
import { listModels, type ModelOption } from '@/api/models'

export default function ReviewPage() {
    const [prId, setPrId] = useState('')
    const [models, setModels] = useState<ModelOption[]>([])
    const [model, setModel] = useState('')
    const [review, setReview] = useState<string | null>(null)
    const [error, setError] = useState('')
    const [loading, setLoading] = useState(false)

    useEffect(() => {
        let cancelled = false
        listModels()
            .then(({ models, defaultId }) => {
                if (cancelled) return
                setModels(models)
                setModel((cur) => (cur && models.some((m) => m.id === cur) ? cur : defaultId))
            })
            .catch(() => {
                /* keep empty list; orchestrator default is used when model is unset */
            })
        return () => {
            cancelled = true
        }
    }, [])

    async function submit() {
        if (!prId.trim()) {
            setError('PR id is required')
            return
        }
        setError('')
        setReview(null)
        setLoading(true)
        try {
            const md = await requestReview({ prId: prId.trim(), model: model || undefined })
            setReview(md)
        } catch (e) {
            const resp = (e as { response?: { data?: { error?: string }; status?: number } }).response
            setError(resp?.data?.error ?? `Request failed${resp?.status ? ` (${resp.status})` : ''}`)
        } finally {
            setLoading(false)
        }
    }

    return (
        <div className="page">
            <h1>PR Reviewer</h1>

            <div className="field">
                <label htmlFor="prid">PR ID (Owner/Repo/Index)</label>
                <input
                    id="prid"
                    value={prId}
                    onChange={(e) => setPrId(e.target.value)}
                    placeholder="my-org/backend/42"
                />
            </div>

            {models.length > 0 && (
                <div className="model-picker">
                    <label htmlFor="model">Model</label>
                    <select
                        id="model"
                        className="model-select"
                        value={model}
                        onChange={(e) => setModel(e.target.value)}
                    >
                        {models.map((m) => (
                            <option key={m.id} value={m.id}>
                                {m.label}
                            </option>
                        ))}
                    </select>
                </div>
            )}

            <button onClick={submit} disabled={loading}>
                {loading ? 'Generating…' : 'Generate Review'}
            </button>

            {error && <div className="error">{error}</div>}

            {review !== null && (
                <div className="result">
                    <Markdown remarkPlugins={[remarkGfm]}>{review}</Markdown>
                </div>
            )}
        </div>
    )
}
