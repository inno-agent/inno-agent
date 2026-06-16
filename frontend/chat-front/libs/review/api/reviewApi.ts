import { apiClient, apiEndpoints } from '@shared/api/axios'

export const reviewPullRequest = async (prId: string, diff?: string) => {
    const { data } = await apiClient.post<{ review_markdown: string }>(apiEndpoints.review, {
        pr_id: prId,
        ...(diff ? { diff } : {}),
    })

    return data.review_markdown
}
