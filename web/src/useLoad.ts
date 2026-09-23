import { useEffect, useRef, useState } from 'react'
import { api } from './api'

/**
 * Загрузка по пути. `loading` — только до первого ответа; при смене пути прежние данные
 * остаются на экране (keepPrevious), а `refreshing` показывает фоновое обновление (без размонтирования).
 */
export function useLoad<T>(path: string, initial: T, { keepPrevious = false }: { keepPrevious?: boolean } = {}) {
  const [data, setData] = useState<T>(initial)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState('')
  const loaded = useRef(false)
  useEffect(() => {
    let alive = true
    if (keepPrevious && loaded.current) setRefreshing(true); else setLoading(true)
    setError('')
    api<T>(path)
      .then(value => { if (alive) { setData(value); loaded.current = true } })
      .catch((err: Error) => { if (alive) setError(err.message) })
      .finally(() => { if (alive) { setLoading(false); setRefreshing(false) } })
    return () => { alive = false }
  }, [path, keepPrevious])
  return { data, setData, loading, refreshing, error, setError }
}
