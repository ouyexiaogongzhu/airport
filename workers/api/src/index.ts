import { Hono } from 'hono';

export type Env = {
  DB: D1Database;
  CACHE: KVNamespace;
  BACKUPS: R2Bucket;
  JWT_SECRET?: string;
  BEPUSDT_API_URL?: string;
  BEPUSDT_TOKEN?: string;
  BEPUSDT_SECRET?: string;
  PAYPAL_CLIENT_ID?: string;
  PAYPAL_CLIENT_SECRET?: string;
  PAYPAL_WEBHOOK_ID?: string;
  TURNSTILE_SECRET?: string;
};

export function createApp() {
  const app = new Hono<{ Bindings: Env }>();

  app.get('/health', (c) =>
    c.json({ status: 'ok', service: 'rfplay-api' }),
  );

  return app;
}

export default { fetch: createApp().fetch };
