import { Button, ButtonLink } from '../ui'

export function PageError({ message, retry }: { message: string; retry?: () => void }) {
  return <div className="container error-page">
    <span className="error-mark" aria-hidden="true">?</span>
    <h1>Не получилось открыть страницу</h1>
    <p>{message}</p>
    {retry ? <Button onClick={retry}>Повторить</Button> : <ButtonLink variant="primary" to="/">В каталог задач</ButtonLink>}
  </div>
}
