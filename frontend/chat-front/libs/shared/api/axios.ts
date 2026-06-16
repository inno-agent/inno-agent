import axios from 'axios'

const API_BASE_URL = '/api/v1'

export const apiEndpoints = {
    chats: '/chats',
    chatMessages: (chatId: string) => `/chats/${chatId}/messages`,
    chatStream: (chatId: string) => `/chats/${chatId}/stream`,
    review: '/review',
}

export const apiClient = axios.create({
    baseURL: API_BASE_URL,
    headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
    },
})

apiClient.interceptors.request.use((config) => {
    const token = localStorage.getItem('aicore_token')
    if (token) {
        config.headers.Authorization = `Bearer ${token}`
    }
    return config
})

apiClient.interceptors.response.use(
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
