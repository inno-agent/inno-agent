export type SettingsTab = 'general' | 'account' | 'personalization'

export interface CurrentUser {
    email: string
    avatarUrl?: string
}
