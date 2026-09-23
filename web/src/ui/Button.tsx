import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react'
import { Link, type LinkProps } from 'react-router-dom'
import { cx } from './cx'
import { Spinner } from './Spinner'

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger'
export type ButtonSize = 'sm' | 'md' | 'lg'
interface Look { variant?: ButtonVariant; size?: ButtonSize; block?: boolean }

export const buttonClass = ({ variant = 'secondary', size = 'md', block }: Look, extra?: string) =>
  cx('ui-btn', `ui-btn-${variant}`, size !== 'md' && `ui-btn-${size}`, block && 'ui-btn-block', extra)

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement>, Look { loading?: boolean; children: ReactNode }

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button({ variant, size, block, loading = false, disabled, className, type = 'button', children, ...rest }, ref) {
  return <button ref={ref} type={type} className={buttonClass({ variant, size, block }, className)} disabled={disabled || loading} aria-busy={loading || undefined} {...rest}>
    {loading && <Spinner size="sm" />}{children}
  </button>
})

export function ButtonLink({ variant, size, block, className, ...rest }: LinkProps & Look) {
  return <Link className={buttonClass({ variant, size, block }, className)} {...rest} />
}
