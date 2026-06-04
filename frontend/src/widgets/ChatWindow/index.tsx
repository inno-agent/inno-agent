import { useState, useRef, useEffect } from 'react'
import styles from './ChatWindow.module.css'
import Plus from '../../shared/ui/icons/plus.svg?react'

const models = ['Claude Sonnet 4.6', 'Claude Opus 4.7', 'Claude Haiku 4.5']

export const ChatWindow = () => {
    const [isModelOpen, setIsModelOpen] = useState(false)
    const [selectedModel, setSelectedModel] = useState(models[0])
    const [text, setText] = useState('')
    const wrapperRef = useRef<HTMLDivElement>(null)

    useEffect(() => {
        const handleOutside = (e: MouseEvent) => {
            if (wrapperRef.current && !wrapperRef.current.contains(e.target as Node)) {
                setIsModelOpen(false)
            }
        }
        document.addEventListener('mousedown', handleOutside)
        return () => document.removeEventListener('mousedown', handleOutside)
    }, [])

    const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault()
            if (text.trim()) {
                setText('')
            }
        }
    }

    return (
        <div className={styles.window}>
            <h1 className={styles.title}>Над чем будем работать?</h1>
            <div className={styles.inputWrapper}>
                <textarea
                    className={styles.input}
                    placeholder="Как я могу помочь вам сегодня?"
                    value={text}
                    onChange={e => setText(e.target.value)}
                    onKeyDown={handleKeyDown}
                />
                <div className={styles.toolbar}>
                    <button className={styles.toolbarBtn} aria-label="Прикрепить файл">
                        <Plus />
                    </button>
                    <div className={styles.toolbarRight}>
                        <div className={styles.modelSelectorWrapper} ref={wrapperRef}>
                            <button
                                className={styles.modelSelector}
                                onClick={() => setIsModelOpen(v => !v)}
                            >
                                {selectedModel}
                                <svg
                                    className={[styles.arrow, isModelOpen ? styles.arrowOpen : ''].join(' ')}
                                    width="10" height="6" viewBox="0 0 10 6" fill="none"
                                >
                                    <path d="M1 1L5 5L9 1" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                                </svg>
                            </button>
                            {isModelOpen && (
                                <div className={styles.modelDropdown}>
                                    {models.map(model => (
                                        <button
                                            key={model}
                                            className={[
                                                styles.modelOption,
                                                selectedModel === model ? styles.modelOptionActive : '',
                                            ].join(' ')}
                                            onClick={() => { setSelectedModel(model); setIsModelOpen(false) }}
                                        >
                                            {model}
                                        </button>
                                    ))}
                                </div>
                            )}
                        </div>
                        <button
                            className={styles.sendBtn}
                            aria-label="Отправить"
                            disabled={!text.trim()}
                        >
                            <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                                <path d="M8 12V4M8 4L4 8M8 4L12 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                            </svg>
                        </button>
                    </div>
                </div>
            </div>
        </div>
    )
}
