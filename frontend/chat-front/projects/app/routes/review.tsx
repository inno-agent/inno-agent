import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { reviewPullRequest } from '@libs/review/api/reviewApi'
import { StaticMarkdown } from '@libs/review/ui/StaticMarkdown'
import { Button } from '@shared/ui/button'
import styles from './review.module.css'

export const Route = createFileRoute('/review')({
    validateSearch: (search: Record<string, unknown>) => ({
        prId:
            typeof search.pr_id === 'string' && search.pr_id.length > 0
                ? search.pr_id
                : undefined,
        diff:
            typeof search.diff === 'string' && search.diff.length > 0
                ? search.diff
                : undefined,
    }),
    component: ReviewPage,
})

function ReviewPage() {
    const { prId, diff: diffFromQuery } = Route.useSearch()
    const [diffInput, setDiffInput] = useState(diffFromQuery ?? '')
    const [reviewMarkdown, setReviewMarkdown] = useState<string | null>(null)
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState<string | null>(null)

    const handleReview = async () => {
        if (!prId) {
            setError('Укажите pr_id в query-параметре, например: /review?pr_id=123')
            return
        }

        setLoading(true)
        setError(null)

        try {
            const markdown = await reviewPullRequest(prId, diffInput.trim() || undefined)
            setReviewMarkdown(markdown)
        } catch {
            setError('Не удалось получить ревью. Укажите diff или настройте GitFlame.')
        } finally {
            setLoading(false)
        }
    }

    return (
        <section className={styles.page}>
            <header className={styles.header}>
                <h1 className={styles.title}>AI PR Review</h1>
                <p className={styles.subtitle}>
                    {prId ? `Pull request: ${prId}` : 'Передайте pr_id через query-параметр'}
                </p>
            </header>

            <label className={styles.diffField}>
                <span className={styles.diffLabel}>Diff (опционально)</span>
                <textarea
                    className={styles.diffInput}
                    value={diffInput}
                    onChange={(event) => setDiffInput(event.target.value)}
                    placeholder="diff --git a/main.go b/main.go"
                    rows={8}
                />
            </label>

            <div className={styles.actions}>
                <Button onClick={handleReview} disabled={loading || !prId}>
                    {loading ? 'Генерация...' : 'Review'}
                </Button>
            </div>

            {error && <p className={styles.error}>{error}</p>}

            <div className={styles.review}>
                {reviewMarkdown ? (
                    <StaticMarkdown content={reviewMarkdown} />
                ) : (
                    <p className={styles.empty}>
                        Нажмите Review, чтобы сгенерировать markdown-отчёт по PR.
                    </p>
                )}
            </div>
        </section>
    )
}
