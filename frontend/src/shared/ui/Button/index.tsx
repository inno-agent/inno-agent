import styles from './Button.module.css';
import React from 'react';

interface ButtonProps {
    variant: 'button' | 'list' | 'icon'
    isActive?: boolean
    onClick?: () => void
    children?: React.ReactNode
}

const Button = ({ variant, isActive, onClick, children }: ButtonProps) => {
    const className = [
        styles[variant],
        isActive ? styles.listActive : ''
    ].join(' ')

    return (
        <button className={className} onClick={onClick}>
            {children}
        </button>
    )
}

export default Button