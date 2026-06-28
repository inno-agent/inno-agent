import type { ReactNode } from 'react'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@shared/ui/select'
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

type SelectOption = string | { label: string; description: string }

interface SettingsSelectProps {
    value: string
    options: SelectOption[]
    onChange: (value: string) => void
}

export const SettingsSelect = ({ value, options, onChange }: SettingsSelectProps) => {
    const labels = options.map((o) => (typeof o === 'string' ? o : o.label))
    const effectiveValue = labels.includes(value) ? value : labels[0]
    return (
        <Select value={effectiveValue} onValueChange={onChange}>
            <SelectTrigger
                size="sm"
                className="border-none bg-transparent dark:bg-transparent dark:hover:bg-transparent shadow-none px-0 h-auto gap-1 focus-visible:ring-0 text-[var(--color-text-primary)]"
            >
                <SelectValue />
            </SelectTrigger>
            <SelectContent>
                {options.map((option) => {
                    const label = typeof option === 'string' ? option : option.label
                    const description = typeof option === 'string' ? undefined : option.description
                    return (
                        <SelectItem key={label} value={label} description={description}>
                            {label}
                        </SelectItem>
                    )
                })}
            </SelectContent>
        </Select>
    )
}
