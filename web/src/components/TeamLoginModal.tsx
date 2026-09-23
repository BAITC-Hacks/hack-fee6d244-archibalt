import { useEffect, useRef, useState, type FormEvent } from 'react'
import { api, json, type Team } from '../api'
import { errorText, type TeamSession } from '../fields'
import { Alert, Button, Chips, Field, Input, Modal, useToast } from '../ui'

const DEMO_HINT = 'Демо-режим: код никуда не отправляется, введите 000000'

/** Вход команды в два шага: контакт → одноразовый код. */
export function TeamLoginModal({ open, onClose, onLogin }: { open: boolean; onClose: () => void; onLogin: (session: TeamSession) => void }) {
  const toast = useToast()
  const [step, setStep] = useState<'contact' | 'code'>('contact')
  const [contact, setContact] = useState(''); const [teamName, setTeamName] = useState(''); const [code, setCode] = useState('')
  const [hint, setHint] = useState(''); const [busy, setBusy] = useState(false); const [error, setError] = useState('')
  const [teams, setTeams] = useState<Team[]>([])
  const contactRef = useRef<HTMLInputElement>(null); const codeRef = useRef<HTMLInputElement>(null)

  useEffect(() => { if (!open) return; setError(''); setCode(''); let alive = true; api<Team[]>('/teams').then(list => { if (alive) setTeams(list) }).catch(() => { if (alive) setTeams([]) }); return () => { alive = false } }, [open])
  useEffect(() => { if (open && step === 'code') codeRef.current?.focus() }, [open, step])

  async function requestCode(event: FormEvent) {
    event.preventDefault()
    if (!contact.trim()) { setError('Укажите email или телефон команды.'); contactRef.current?.focus(); return }
    setBusy(true); setError('')
    try {
      const response = await api<{ hint?: string; demo?: boolean }>('/auth/request-code', json('POST', { contact }))
      setHint(response.demo ? DEMO_HINT : response.hint || ''); setCode(''); setStep('code')
    } catch (err) { setError(errorText(err)); contactRef.current?.focus() } finally { setBusy(false) }
  }

  async function verify(value: string) {
    if (busy) return
    if (value.length !== 6) { setError('Код состоит из 6 цифр.'); codeRef.current?.focus(); return }
    setBusy(true); setError('')
    try {
      const session = await api<TeamSession>('/auth/verify', json('POST', { contact, code: value, team_name: teamName }))
      localStorage.setItem('team_token', session.token)
      toast.success(`Вы вошли как ${session.team.name}`)
      setStep('contact'); setCode(''); setHint('')
      onLogin(session)
    } catch (err) { setError(errorText(err)); setCode(''); window.setTimeout(() => codeRef.current?.focus(), 0) } finally { setBusy(false) }
  }

  function onCodeChange(raw: string) {
    const digits = raw.replace(/\D/g, '').slice(0, 6)
    setCode(digits); setError('')
    if (digits.length === 6) void verify(digits)
  }

  const seedTeams = teams.map(team => team.name).filter(Boolean).slice(0, 5)

  if (step === 'contact') return <Modal open={open} onClose={onClose} title="Войти как команда" description="Регистрации нет: введите контакт, и мы пришлём одноразовый код." initialFocus={contactRef}>
    <form className="login-form" onSubmit={requestCode} noValidate>
      <Field label="Email или телефон команды" required error={error || undefined}>
        <Input ref={contactRef} value={contact} onChange={event => { setContact(event.target.value); setError('') }} placeholder="team@example.com" autoComplete="email" inputMode="email" />
      </Field>
      <Field label="Название команды" optional hint="Нужно только при первом входе. Можно выбрать демо-команду.">
        <Input value={teamName} onChange={event => setTeamName(event.target.value)} placeholder="Как вас представить бизнесу" autoComplete="organization" />
      </Field>
      <Chips label="Демо-команды" options={seedTeams} value={teamName} onPick={setTeamName} />
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
      <Button variant="ghost" size="sm" className="login-back" disabled={busy} onClick={() => { setStep('contact'); setError(''); setCode('') }}>Изменить контакт или команду</Button>
    </form>
  </Modal>
}
