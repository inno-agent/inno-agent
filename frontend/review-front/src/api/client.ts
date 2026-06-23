import axios, { type AxiosInstance } from 'axios'
import { getToken, login, logout } from '@/auth/auth'

// withAuth attaches the bearer token and the shared 401 -> login behaviour so
// every API surface handles auth identically.
function withAuth(instance: AxiosInstance): AxiosInstance {
    instance.interceptors.request.use((config) => {
        const token = getToken()
        if (token) {
            config.headers.Authorization = `Bearer ${token}`
        }
        return config
    })

    instance.interceptors.response.use(
        (response) => response,
        (error) => {
            if (error.response?.status === 401) {
                logout()
                void login()
            }
            return Promise.reject(error)
        },
    )

    return instance
}

// review-api (PR reviews), same-origin via ingress.
export const apiClient = withAuth(
    axios.create({
        baseURL: '/api/v1',
        headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    }),
)

// Orchestrator (LLM platform) API, served under /llm by the ingress.
export const llmClient = withAuth(
    axios.create({
        baseURL: '/llm/v1',
        headers: { Accept: 'application/json' },
    }),
)
