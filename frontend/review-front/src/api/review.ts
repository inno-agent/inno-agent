import { apiClient } from '@/api/client'

export interface ReviewRequest {
    prId: string
    diff?: string
}

interface ReviewResponse {
    review_markdown: string
}

export async function requestReview(req: ReviewRequest): Promise<string> {
    const body: Record<string, string> = { pr_id: req.prId }
    if (req.diff) body.diff = req.diff
    const { data } = await apiClient.post<ReviewResponse>('/review', body)
    return data.review_markdown
}
