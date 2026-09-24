/**
 * Same-origin /api/* proxy → Worker `rfplay-api` via Service Binding `API`.
 *
 * Local: wrangler pages dev dist --service API=rfplay-api
 * (run `wrangler dev` in workers/api in parallel).
 */

interface Env {
  API: Fetcher
}

export const onRequest: PagesFunction<Env> = async (context) => {
  const incoming = context.request
  const url = new URL(incoming.url)
  url.protocol = 'https:'
  url.hostname = 'api.rfplay.uk'

  const headers = new Headers(incoming.headers)
  headers.set('Host', 'api.rfplay.uk')

  const init: RequestInit & { duplex?: 'half' } = {
    method: incoming.method,
    headers,
    redirect: 'manual',
  }
  if (incoming.method !== 'GET' && incoming.method !== 'HEAD') {
    init.body = incoming.body
    init.duplex = 'half'
  }

  return context.env.API.fetch(new Request(url.toString(), init))
}
