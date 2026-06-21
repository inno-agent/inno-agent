import { useState } from 'react'
import { Pencil } from 'lucide-react'
import { Button } from '@shared/ui/button'
import { SettingsRow, SettingsSectionTitle } from '@libs/settings/ui/rows/SettingsRow'
import { deleteAccount } from '@libs/settings/api/settingsApi'
import styles from './AccountTab.module.css'

interface AccountTabProps {
    email: string
    onLogout: () => void
}

export const AccountTab = ({ email, onLogout }: AccountTabProps) => {
    const [confirmingDelete, setConfirmingDelete] = useState(false)
    const [isDeleting, setIsDeleting] = useState(false)

    const handleDelete = async () => {
        setIsDeleting(true)
        await deleteAccount()
        setIsDeleting(false)
        setConfirmingDelete(false)
    }

    return (
        <>
            <SettingsSectionTitle>Аккаунт</SettingsSectionTitle>

            <SettingsRow label="Электронная почта">
                <span className={styles.value}>{email}</span>
            </SettingsRow>

            <SettingsRow label="Аватар">
                <button className={styles.avatarEdit}>
                    <Pencil className={styles.avatarEditIcon} />
                </button>
            </SettingsRow>

            <SettingsRow label="Выйти из аккаунта">
                <Button variant="outline" size="sm" onClick={onLogout}>
                    Выйти
                </Button>
            </SettingsRow>

            <SettingsRow label="Удалить аккаунт">
                {confirmingDelete ? (
                    <div className={styles.confirmGroup}>
                        <Button variant="outline" size="sm" onClick={() => setConfirmingDelete(false)}>
                            Отмена
                        </Button>
                        <Button
                            variant="destructive"
                            size="sm"
                            className="hover:bg-red-700"
                            onClick={handleDelete}
                            disabled={isDeleting}
                        >
                            {isDeleting ? 'Удаление...' : 'Точно удалить'}
                        </Button>
                    </div>
                ) : (
                    <Button
                        variant="destructive"
                        size="sm"
                        className="hover:bg-red-700"
                        onClick={() => setConfirmingDelete(true)}
                    >
                        Удалить аккаунт
                    </Button>
                )}
            </SettingsRow>
        </>
    )
}
