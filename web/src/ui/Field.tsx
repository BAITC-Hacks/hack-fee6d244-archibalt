import { cloneElement, isValidElement, useId, type ReactElement, type ReactNode } from 'react'
import { cx } from './cx'

interface FieldProps {
  label: ReactNode
  hint?: ReactNode
  error?: string
  required?: boolean
  optional?: boolean
  id?: string
  className?: string
  children: ReactElement<Record<string, unknown>>
}

/** Подпись, подсказка и ошибка для одного контрола; связывает их через id и aria-describedby. */
export function Field({ label, hint, error, required, optional, id, className, children }: FieldProps) {
  const auto = useId()
  const controlId = id || (children.props.id as string | undefined) || auto
  const hintId = hint ? `${controlId}-hint` : undefined
  const errorId = error ? `${controlId}-error` : undefined
  const describedBy = [errorId, hintId].filter(Boolean).join(' ') || undefined
  const control = isValidElement(children) ? cloneElement(children, { id: controlId, 'aria-describedby': describedBy, invalid: Boolean(error), required: required || undefined }) : children
  return <div className={cx('ui-field', className)}>
    <label className="ui-field-label" htmlFor={controlId}>{label}{required && <span className="ui-field-required">обязательно</span>}{optional && <span className="ui-field-optional">необязательно</span>}</label>
    {control}
    {error && <p className="ui-field-error" id={errorId}>{error}</p>}
    {hint && <p className="ui-field-hint" id={hintId}>{hint}</p>}
  </div>
}
