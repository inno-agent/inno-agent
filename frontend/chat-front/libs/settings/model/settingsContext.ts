import { createContext } from 'react'

export type Theme = 'system' | 'dark' | 'light'

export interface SettingsState {
    theme: Theme
    setTheme: (theme: Theme) => void
}

export const SettingsContext = createContext<SettingsState>({
    theme: 'system',
    setTheme: () => {},
})