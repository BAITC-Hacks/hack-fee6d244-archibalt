import { useEffect, useRef, useState, type FormEvent } from 'react'
import { api, json } from '../api'
import { errorText, type TeamSession } from '../fields'
import { Alert, Button, Field, Input, Modal, Segmented, useToast } from '../ui'

export type Role = 'team' | 'business'
const DEMO_HINT = 'Демо-режим: код никуда не отправляется, введите 000000'
const roles: { value: Role; label: string }[] = [{ value: 'business', label: 'Я бизнес' }, { value: 'team', label: 'Я команда' }]
const copy: Record<Role, { title: string; description: string; contact: string }> = {
  business: { title: 'Войти как заявитель', description: 'Так вы сможете вернуться к задаче, дополнить её и увидеть отклики.', contact: 'Ваш email или телефон' },
  team: { title: 'Войти как команда', description: 'Укажите email или телефон команды, мы пришлём одноразовый код.', contact: 'Email или телефон команды' },
}

interface LoginModalProps {
  open: boolean
  role: Role
  onRoleChange: (role: Role) => void
  onClose: () => void
  onTeam: (session: TeamSession) => void
  onBusiness: (session: { token: string; contact: string }) => void | Promise<void>
}

/** Единый вход для бизнеса и команды: контакт → одноразовый код. */
export function LoginModal({ open, role, onRoleChange, onClose, onTeam, onBusiness }: LoginModalProps) {
  const toast = useToast()
  const [step, setStep] = useState<'contact' | 'code'>('contact')
  const [contact, setContact] = useState(''); const [teamName, setTeamName] = useState(''); const [code, setCode] = useState('')
  const [hint, setHint] = useState(''); const [busy, setBusy] = useState(false); const [error, setError] = useState('')
  const contactRef = useRef<HTMLInputElement>(null); const codeRef = useRef<HTMLInputElement>(null)

  useEffect(() => { if (open) { setError(''); setCode(''); setStep('contact') } }, [open])
  useEffect(() => { if (open && step === 'code') codeRef.current?.focus() }, [open, step])

  function switchRole(next: Role) { onRoleChange(next); setError(''); setStep('contact') }

  async function requestCode(event: FormEvent) {
    event.preventDefault()
    if (!contact.trim()) { setError('Укажите email или телефон.'); contactRef.current?.focus(); return }
    setBusy(true); setError('')
    try {
      const response = await api<{ hint?: string; demo?: boolean }>(role === 'team' ? '/auth/request-code' : '/auth/business/request-code', json('POST', { contact }))
      setHint(response.demo ? DEMO_HINT : response.hint || ''); setCode(''); setStep('code')
    } catch (err) { setError(errorText(err)); contactRef.current?.focus() } finally { setBusy(false) }
  }

  async function verify(value: string) {
    if (busy) return
    if (value.length !== 6) { setError('Код состоит из 6 цифр.'); codeRef.current?.focus(); return }
    setBusy(true); setError('')
    try {
      if (role === 'team') {
        const session = await api<TeamSession>('/auth/verify', json('POST', { contact, code: value, team_name: teamName }))
        toast.success(`Вы вошли как ${session.team.name}`)
        onTeam(session)
      } else {
        const session = await api<{ token: string; contact: string }>('/auth/business/verify', json('POST', { contact, code: value }))
        toast.success('Вы вошли как заявитель')
        await onBusiness(session)
      }
      setStep('contact'); setCode(''); setHint('')
    } catch (err) { setError(errorText(err)); setCode(''); window.setTimeout(() => codeRef.current?.focus(), 0) } finally { setBusy(false) }
  }

  function onCodeChange(raw: string) {
    const digits = raw.replace(/\D/g, '').slice(0, 6)
    setCode(digits); setError('')
    if (digits.length === 6) void verify(digits)
  }

  if (step === 'contact') return <Modal open={open} onClose={onClose} title={copy[role].title} description={copy[role].description} initialFocus={contactRef}>
    <form className="login-form" onSubmit={requestCode} noValidate>
      <Segmented label="Кто входит" value={role} options={roles} onChange={switchRole} />
      <Field label={copy[role].contact} required error={error || undefined}>
        <Input ref={contactRef} value={contact} onChange={event => { setContact(event.target.value); setError('') }} placeholder="name@example.com" autoComplete="email" inputMode="email" />
      </Field>
      {role === 'team' && <Field label="Название команды" optional hint="Нужно только при первом входе: так команду увидит бизнес.">
        <Input value={teamName} onChange={event => setTeamName(event.target.value)} placeholder="Как вас представить бизнесу" autoComplete="organization" />
      </Field>}
      <Button type="submit" variant="primary" size="lg" block loading={busy}>{busy ? 'Отправляем код…' : 'Получить код'}</Button>
    </form>
  </Modal>

  return <Modal open={open} onClose={onClose} title="Введите код" description={`Код для ${contact}. Вход произойдёт сам после шестой цифры.`} initialFocus={codeRef}>
    <form className="login-form" onSubmit={event => { event.preventDefault(); void verify(code) }} noValidate>
      {hint && <Alert tone="warning">
        <div className="login-hint"><span>{hint}</span><Button size="sm" variant="secondary" disabled={busy} onClick={() => onCodeChange('000000')}>Подставить 000000</Button></div>
      </Alert>}
      <Field label="Код из 6 цифр" error={error || undefined}>
        <Input ref={codeRef} className="ui-input-code" value={code} onChange={event => onCodeChange(event.target.value)} inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]*" maxLength={6} placeholder="••••••" disabled={busy} />
      </Field>
      <Button type="submit" variant="primary" size="lg" block loading={busy} disabled={code.length !== 6}>{busy ? 'Входим…' : 'Войти'}</Button>
      <Button variant="ghost" size="sm" className="login-back" disabled={busy} onClick={() => { setStep('contact'); setError(''); setCode('') }}>Изменить контакт</Button>
    </form>
  </Modal>
}
