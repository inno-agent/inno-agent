import type { ReactNode } from 'react'
import styles from './SettingsRow.module.css'

interface SettingsRowProps {
    label: string
    description?: string
    children: ReactNode
}

export const SettingsRow = ({ label, description, children }: SettingsRowProps) => {
    return (
        <div className={styles.row}>
            <div className={styles.labelGroup}>
                <span className={styles.label}>{label}</span>
                {description && <span className={styles.description}>{description}</span>}
            </div>
            {children}
        </div>
    )
}

export const SettingsSectionTitle = ({ children }: { children: ReactNode }) => (
    <div className={styles.section}>
        <div className={styles.sectionTitle}>{children}</div>
        <div className={styles.sectionDivider} />
    </div>
)

interface SettingsSelectProps {
    value: string
    options: string[]
    onChange: (value: string) => void
}

export const SettingsSelect = ({ value, options, onChange }: SettingsSelectProps) => (
    <select className={styles.select} value={value} onChange={(e) => onChange(e.target.value)}>
        {options.map((option) => (
            <option key={option} value={option}>
                {option}
            </option>
        ))}
    </select>
)
