import React, { useEffect, useState } from 'react'
import { SettingsContext, type Theme } from './model/settingsContext'

export function SettingsProvider({ children }: Readonly<{ children: React.ReactNode }>) {
    const [theme, setTheme] = useState<Theme>(
        (localStorage.getItem('theme') as Theme) ?? 'system'
    )

    useEffect(() => {
        const root = document.documentElement
        if (theme === 'system') {
            const isDark = window.matchMedia('(prefers-color-scheme: dark)').matches
            root.className = isDark ? 'dark' : 'light'
        } else {
            root.className = theme
        }
        localStorage.setItem('theme', theme)
    }, [theme])

    return (
        <SettingsContext.Provider value={{ theme, setTheme }}>
            {children}
        </SettingsContext.Provider>
    )
}