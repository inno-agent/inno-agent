import axios from 'axios'
import { getToken, login, logout } from '@/auth/auth'

export const apiClient = axios.create({
    baseURL: '/api/v1',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
})

apiClient.interceptors.request.use((config) => {
    const token = getToken()
    if (token) {
        config.headers.Authorization = `Bearer ${token}`
    }
    return config
})

apiClient.interceptors.response.use(
    (response) => response,
    (error) => {
        if (error.response?.status === 401) {
            logout()
            void login()
        }
        return Promise.reject(error)
    },
)
