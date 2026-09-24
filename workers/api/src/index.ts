import { Hono } from 'hono';
import { cors } from 'hono/cors';
import { clientRoutes } from './routes/client';
import { authRoutes } from './routes/auth';
import { publicRoutes } from './routes/public';
import { oauthRoutes } from './routes/oauth';
import { paymentRoutes } from './routes/payment';
import { webRoutes } from './routes/web';
import { adminRoutes, publicProductRoutes } from './routes/admin';
import { nodeRoutes } from './routes/node';
import { runEntitlementMaintenance } from './lib/entitlement';
import { maybeRotateJwtKeys } from './lib/jwtkeys';

export type Env = {
  MOCK_PAY_ENABLED?: string;
  DB: D1Database;
  CACHE: KVNamespace;
  BACKUPS: R2Bucket;
  CORS_ORIGINS?: string;
  /** Bootstrap / fallback only；活簽名密鑰在 KV（jwt:current / jwt:previous），見 jwtkeys.ts */
  JWT_SECRET?: string;
  BEPUSDT_API_URL?: string;
  BEPUSDT_TOKEN?: string;
  BEPUSDT_SECRET?: string;
  PAYPAL_CLIENT_ID?: string;
  PAYPAL_CLIENT_SECRET?: string;
  PAYPAL_WEBHOOK_ID?: string;
  TURNSTILE_SECRET?: string;
  TURNSTILE_DISABLED?: string;
  COOKIE_DOMAIN?: string;
  PORTAL_URL?: string;
  /** Google OAuth Web client（wrangler secrets） */
  GOOGLE_CLIENT_ID?: string;
  GOOGLE_CLIENT_SECRET?: string;
  /** 覆寫回調 URL；預設為當前 origin + /api/v1/public/oauth/google/callback */
  GOOGLE_REDIRECT_URI?: string;
};

export function createApp() {
  const app = new Hono<{ Bindings: Env }>();

  // 對齊 Go cors middleware：白名單 + credentials；env 未配時含 localhost 與 pages.dev 生產域
  const defaultOrigins = [
    'http://localhost:5173',
    'http://localhost:5174',
    'https://rfplay.uk',
    'https://xv.rfplay.uk',
    'https://xva.rfplay.uk',
    'https://rfplay-portal.pages.dev',
    'https://rfplay-admin.pages.dev',
    // 舊域名（若 DNS 仍指向 Pages）
    'https://www.rfplay.uk',
    'https://admin.rfplay.uk',
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

  // API 根路徑：瀏覽器直開不給 404，重定向 portal
  app.get('/', (c) => c.redirect('https://xv.rfplay.uk', 302));

  // clientRoutes 內部路徑不含 /client 前綴；nodeRoutes 為 gateway 節點面（HMAC 簽名）
  app.route('/api/v1/client', clientRoutes());
  app.route('/api/v1', nodeRoutes());
  app.route('/api/v1', authRoutes());
  app.route('/api/v1', publicRoutes());
  app.route('/api/v1', oauthRoutes());
  app.route('/api/v1', paymentRoutes());
  app.route('/api/v1', webRoutes());
  app.route('/api/v1', adminRoutes());
  app.route('/api/v1', publicProductRoutes());

  // 統一 JSON 錯誤處理 + 結構化日誌
  app.onError((err, c) => {
    console.error(JSON.stringify({
      level: "error",
      path: c.req.path,
      method: c.req.method,
      message: err instanceof Error ? err.message : String(err),
      stack: err instanceof Error ? (err.stack ?? "").split("\n").slice(0, 5).join(" | ") : undefined,
    }));
    return c.json({ error: "internal server error" }, 500);
  });
  app.notFound((c) => c.json({ error: "not found" }, 404));

  return app;
}

export default {
  fetch: createApp().fetch,
  scheduled: async (_event: ScheduledController, env: Env, ctx: ExecutionContext) => {
    const now = Math.floor(Date.now() / 1000);
    ctx.waitUntil(
      (async () => {
        const rot = await maybeRotateJwtKeys(env, now);
        if (rot === 'rotated') {
          console.log(JSON.stringify({ level: 'info', msg: 'jwt signing key rotated' }));
        }
        await runEntitlementMaintenance(env.DB, now);
      })(),
    );
  },
};
