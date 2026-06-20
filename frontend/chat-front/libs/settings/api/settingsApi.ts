import type { CurrentUser } from '@libs/settings/model/types'

// TODO: заменить на реальный запрос к identity/profile API
export const getCurrentUser = async (): Promise<CurrentUser> => {
    return {
        email: 'k.krutova@innopolis.university',
        avatarUrl: undefined,
    }
}

// TODO: заменить на реальный запрос удаления аккаунта
export const deleteAccount = async (): Promise<void> => {
    return Promise.resolve()
}
