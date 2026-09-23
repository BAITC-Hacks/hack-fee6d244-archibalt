import { forwardRef, useCallback, useLayoutEffect, useRef, type InputHTMLAttributes, type TextareaHTMLAttributes } from 'react'
import { cx } from './cx'

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> { invalid?: boolean }

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input({ invalid, className, ...rest }, ref) {
  return <input ref={ref} className={cx('ui-input', className)} aria-invalid={invalid || undefined} {...rest} />
})

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> { invalid?: boolean; minRows?: number }

/** Многострочное поле, высота подстраивается под текст. */
export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea({ invalid, className, minRows = 3, value, ...rest }, ref) {
  const inner = useRef<HTMLTextAreaElement | null>(null)
  const setRefs = useCallback((node: HTMLTextAreaElement | null) => {
    inner.current = node
    if (typeof ref === 'function') ref(node); else if (ref) ref.current = node
  }, [ref])
  useLayoutEffect(() => {
    const node = inner.current
    if (!node) return
    node.style.height = 'auto'
    node.style.height = `${node.scrollHeight + 2}px`
  }, [value])
  return <textarea ref={setRefs} rows={minRows} value={value} className={cx('ui-textarea', className)} aria-invalid={invalid || undefined} {...rest} />
})
