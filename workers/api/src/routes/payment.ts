// 支付回调路由 — 对齐 Go cmd/server/main.go:69-70 挂载（/api/v1 前缀由主线 route 时决定）
//   POST /public/payment/callback          → Go MockPayCallback（dev only，fails closed）
//   POST /public/payment/callback/:provider → Go PaymentCallback（bepusdt / paypal / mock）
import type { Env } from '../index';
import { Hono } from 'hono';
import { activateSubscription, verifyAndParseCallback } from '../lib/payments';

type OrderRow = {
  id: number;
  user_id: number;
  product_id: number;
  amount: number;
  status: string | null;
  provider: string | null;
};

// Go os.Getenv("MOCK_PAY_ENABLED") == "1"；Env 暂无此绑定，主线加 vars 后可去掉 cast
function mockPayEnabled(env: Env): boolean {
  return (env as { MOCK_PAY_ENABLED?: unknown }).MOCK_PAY_ENABLED === '1';
}

// PayPal 官方验签五元组在 HTTP headers 上，合入 body 传给合约函数（payments.ts 约定 _headers）
function paypalHeaders(h: Headers): Record<string, string> {
  const get = (name: string): string => h.get(name) ?? '';
  return {
    'paypal-auth-algo': get('paypal-auth-algo'),
    'paypal-cert-url': get('paypal-cert-url'),
    'paypal-transmission-id': get('paypal-transmission-id'),
    'paypal-transmission-sig': get('paypal-transmission-sig'),
    'paypal-transmission-time': get('paypal-transmission-time'),
  };
}

export function paymentRoutes() {
  const app = new Hono<{ Bindings: Env }>();

  app.post('/public/payment/callback', async (c) => {
    if (!mockPayEnabled(c.env)) {
      return c.json({ error: 'mock payment callback is disabled' }, 403);
    }
    let body: Record<string, unknown>;
    try {
      body = await c.req.json<Record<string, unknown>>();
    } catch {
      return c.json({ error: 'invalid request body' }, 400);
    }
    const orderId = Number(body.order_id);
    if (!Number.isInteger(orderId) || orderId === 0) {
      return c.json({ error: 'order_id is required' }, 400);
    }
    const order = await c.env.DB.prepare('SELECT * FROM orders WHERE id = ?')
      .bind(orderId)
      .first<OrderRow>();
    if (!order) return c.json({ error: 'order not found' }, 404);
    if (order.status !== 'pending') {
      return c.json({ error: 'order is not in pending status' }, 400);
    }

    const status = typeof body.status === 'string' ? body.status : '';
    if (status === 'paid') {
      try {
        // 镜像真实 PaymentCallback 路径：激活订阅而非只入余额
        await activateSubscription(c.env.DB, order.id, order.user_id, order.product_id);
      } catch {
        return c.json({ error: 'failed to process payment' }, 500);
      }
      return c.json({ message: 'payment successful', order: { ...order, status: 'paid' } });
    }
    if (status === 'cancelled') {
      await c.env.DB.prepare("UPDATE orders SET status='cancelled', updated_at=? WHERE id=?")
        .bind(new Date().toISOString(), order.id)
        .run();
      return c.json({ message: 'payment cancelled', order: { ...order, status: 'cancelled' } });
    }
    return c.json({ error: "invalid status, must be 'paid' or 'cancelled'" }, 400);
  });

  app.post('/public/payment/callback/:provider', async (c) => {
    const provider = c.req.param('provider');
    if (provider !== 'bepusdt' && provider !== 'paypal' && provider !== 'mock') {
      return c.json({ error: 'invalid payment provider' }, 400);
    }
    // mock 永不默认可达（Go PaymentCallback 同款闸门）
    if (provider === 'mock' && !mockPayEnabled(c.env)) {
      return c.json({ error: 'mock payment provider is disabled' }, 403);
    }

    let json: Record<string, unknown>;
    try {
      json = await c.req.json<Record<string, unknown>>();
    } catch {
      return c.json({ error: 'invalid callback body' }, 400);
    }
    const body =
      provider === 'paypal' ? { ...json, _headers: paypalHeaders(c.req.raw.headers) } : json;

    let result: Awaited<ReturnType<typeof verifyAndParseCallback>>;
    try {
      result = await verifyAndParseCallback(provider, body, c.env);
    } catch (e) {
      return c.json({ error: e instanceof Error ? e.message : 'invalid callback' }, 400);
    }

    const orderId = Number.parseInt(result.orderId, 10);
    if (!Number.isInteger(orderId) || orderId <= 0) {
      return c.json({ error: 'invalid order id in callback' }, 400);
    }
    const order = await c.env.DB.prepare('SELECT * FROM orders WHERE id = ?')
      .bind(orderId)
      .first<OrderRow>();
    if (!order) return c.json({ error: 'order not found' }, 404);
    if (order.provider !== provider) return c.json({ error: 'provider mismatch' }, 400);

    // 幂等：已 paid 的重复回调零副作用
    if (order.status === 'paid') return c.text('ok');

    if (result.status === 'paid' && order.status === 'pending') {
      try {
        await activateSubscription(c.env.DB, order.id, order.user_id, order.product_id);
      } catch {
        return c.json({ error: 'failed to activate subscription' }, 500);
      }
      return c.text('ok');
    }

    if (result.status === 'failed') {
      await c.env.DB.prepare("UPDATE orders SET status='failed', updated_at=? WHERE id=?")
        .bind(new Date().toISOString(), order.id)
        .run();
    }
    return c.text('ok');
  });

  return app;
}
