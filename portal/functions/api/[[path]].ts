/**
 * Same-origin /api/* proxy → Worker `rfplay-api` via Service Binding `API`.
 *
 * Keeps Cookie / CSRF / Authorization headers intact. Rewrites the request
 * URL host to api.rfplay.uk so the Worker sees the public API hostname.
 *
 * Local: wrangler pages dev dist --service API=rfplay-api
 * (run `wrangler dev` in workers/api in parallel).
 */

interface Env {
  API?: Fetcher
}

export const onRequest: PagesFunction<Env> = async (context) => {
  if (!context.env.API) {
    return new Response(JSON.stringify({ error: 'api binding unavailable' }), {
      status: 503,
      headers: { 'content-type': 'application/json' },
    })
  }

  const incoming = context.request
  const url = new URL(incoming.url)
  url.protocol = 'https:'
  url.hostname = 'api.rfplay.uk'
  // Preserve pathname + search (/api/v1/...).

  const headers = new Headers(incoming.headers)
  headers.set('Host', 'api.rfplay.uk')

  const init: RequestInit & { duplex?: 'half' } = {
    method: incoming.method,
    headers,
    redirect: 'manual',
  }
  if (incoming.method !== 'GET' && incoming.method !== 'HEAD') {
    init.body = incoming.body
    // Required when forwarding a streaming body to another fetch.
    init.duplex = 'half'
  }

  return context.env.API.fetch(new Request(url.toString(), init))
}
