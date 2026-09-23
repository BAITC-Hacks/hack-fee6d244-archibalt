/** Знак: карточка задачи со шкалой готовности, последний сегмент заполнен акцентом. */
export function LogoMark({ size = 36 }: { size?: number }) {
  return <svg className="ui-logo-mark" width={size} height={size} viewBox="0 0 32 32" aria-hidden="true" focusable="false">
    <rect x="2.25" y="2.25" width="27.5" height="27.5" rx="7" fill="none" stroke="var(--ink)" strokeWidth="2.5" />
    <rect x="6.5" y="18" width="5" height="7.5" rx="1.25" fill="var(--ink)" />
    <rect x="13.5" y="13.5" width="5" height="12" rx="1.25" fill="var(--ink)" />
    <rect x="20.5" y="7" width="5" height="18.5" rx="1.25" fill="var(--accent-dark)" />
  </svg>
}

export function Logo({ size = 36 }: { size?: number }) {
  return <span className="ui-logo"><LogoMark size={size} /><span className="ui-logo-word"><b>Archibalt</b><small>AI Sana</small></span></span>
}
