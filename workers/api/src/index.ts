import { Hono } from 'hono';
import { clientRoutes } from './routes/client';
import { authRoutes } from './routes/auth';
import { publicRoutes } from './routes/public';
import { paymentRoutes } from './routes/payment';
import { webRoutes } from './routes/web';
import { adminRoutes, publicProductRoutes } from './routes/admin';

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
  COOKIE_DOMAIN?: string;
};

export function createApp() {
  const app = new Hono<{ Bindings: Env }>();

  app.get('/health', (c) =>
    c.json({ status: 'ok', service: 'rfplay-api' }),
  );

  // M1/M3 路由（M2 節點面待接入）：clientRoutes 內部路徑不含 /client 前綴
  app.route('/api/v1/client', clientRoutes());
  app.route('/api/v1', authRoutes());
  app.route('/api/v1', publicRoutes());
  app.route('/api/v1', paymentRoutes());
  app.route('/api/v1', webRoutes());
  app.route('/api/v1', adminRoutes());
  app.route('/api/v1', publicProductRoutes());

  return app;
}

export default { fetch: createApp().fetch };
