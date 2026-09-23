import type { ReactNode } from 'react'
import { Button, type ButtonVariant } from './Button'
import { Modal } from './Modal'

interface ConfirmDialogProps {
  open: boolean
  title: ReactNode
  children?: ReactNode
  confirmLabel: string
  cancelLabel?: string
  tone?: ButtonVariant
  busy?: boolean
  onConfirm: () => void
  onCancel: () => void
}

/** Подтверждение действия вместо window.confirm. */
export function ConfirmDialog({ open, title, children, confirmLabel, cancelLabel = 'Отмена', tone = 'primary', busy, onConfirm, onCancel }: ConfirmDialogProps) {
  return <Modal open={open} onClose={onCancel} title={title} footer={<><Button variant="ghost" onClick={onCancel} disabled={busy}>{cancelLabel}</Button><Button variant={tone} loading={busy} onClick={onConfirm}>{confirmLabel}</Button></>}>
    {children}
  </Modal>
}
