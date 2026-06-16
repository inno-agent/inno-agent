import axios, { type AxiosInstance } from 'axios'

const API_BASE_URL = '/api/v1'
// Orchestrator (LLM platform) API, served through the same ingress under /llm.
const LLM_BASE_URL = '/llm/v1'

export const apiEndpoints = {
    chats: '/chats',
    chatMessages: (chatId: string) => `/chats/${chatId}/messages`,
    chatStream: (chatId: string) => `/chats/${chatId}/stream`,
    review: '/review',
}

// withAuth attaches the bearer token and the shared 401 -> logout behaviour so
// every API surface handles auth identically.
const withAuth = (instance: AxiosInstance): AxiosInstance => {
    instance.interceptors.request.use((config) => {
        const token = localStorage.getItem('aicore_token')
        if (token) {
            config.headers.Authorization = `Bearer ${token}`
        }
        return config
    })

    instance.interceptors.response.use(
        (response) => response,
        (error) => {
            if (error.response?.status === 401) {
                localStorage.removeItem('aicore_token')
                localStorage.removeItem('aicore_user_id')
                window.location.href = '/'
            }
            return Promise.reject(error)
        },
    )

    return instance
}

export const apiClient = withAuth(
    axios.create({
        baseURL: API_BASE_URL,
        headers: {
            Accept: 'application/json',
            'Content-Type': 'application/json',
        },
    }),
)

export const llmClient = withAuth(
    axios.create({
        baseURL: LLM_BASE_URL,
        headers: {
            Accept: 'application/json',
        },
    }),
)

export const buildApiUrl = (
    endpoint: string,
    params?: URLSearchParams | Record<string, string | number | boolean | undefined>,
) =>
    apiClient.getUri({
        url: endpoint,
        params,
        paramsSerializer: (value) =>
            value instanceof URLSearchParams
                ? value.toString()
                : new URLSearchParams(
                      Object.entries(value).reduce<Record<string, string>>((acc, [key, item]) => {
                          if (item !== undefined) {
                              acc[key] = String(item)
                          }

                          return acc
                      }, {}),
                  ).toString(),
    })
