import { Hono } from 'hono';
import { cors } from 'hono/cors';
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
  CORS_ORIGINS?: string;
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

  // 對齊 Go cors middleware：白名單 + credentials；env 未配時含 localhost 與 pages.dev 生產域
  const defaultOrigins = [
    'http://localhost:5173',
    'http://localhost:5174',
    'https://rfplay.uk',
    'https://www.rfplay.uk',
    'https://admin.rfplay.uk',
    'https://rfplay-portal.pages.dev',
    'https://rfplay-admin.pages.dev',
  ];
  app.use(
    '*',
    cors({
      origin: (o, c) => {
        const configured = (c.env.CORS_ORIGINS ?? '')
          .split(',')
          .map((s: string) => s.trim())
          .filter(Boolean);
        const allowed = new Set(configured.length ? configured : defaultOrigins);
        return allowed.has(o) ? o : null;
      },
      credentials: true,
    }),
  );

  app.get('/health', (c) =>
    c.json({ status: 'ok', service: 'rfplay-api' }),
  );

  // API 根路徑：瀏覽器直開不給 404，重定向官網
  app.get('/', (c) => c.redirect('https://www.rfplay.uk', 302));

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
