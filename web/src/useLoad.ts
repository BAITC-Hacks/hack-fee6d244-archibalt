import { useEffect, useState } from 'react'
import { api } from './api'

export function useLoad<T>(path: string, initial: T) {
  const [data, setData] = useState<T>(initial)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  useEffect(() => { let alive = true; setLoading(true); setError(''); api<T>(path).then(value => { if (alive) setData(value) }).catch((err: Error) => { if (alive) setError(err.message) }).finally(() => { if (alive) setLoading(false) }); return () => { alive = false } }, [path])
  return { data, setData, loading, error, setError }
}
