export default function PlainHttpNotice({ protocol = window.location.protocol }: { protocol?: string }) {
  if (protocol !== 'http:') return null

  return (
    <aside className="http-notice" role="note" aria-label="Connection security">
      <strong>Plain HTTP connection.</strong>
      <span>
        This page was loaded over HTTP. Session tokens are sent without encryption. On an untrusted
        LAN, use HTTPS through a reverse proxy.
      </span>
    </aside>
  )
}
