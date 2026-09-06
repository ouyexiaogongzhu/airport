// 支付接口 — 签名由主线定义（实现体；三函数签名不变；routes/web.ts 依赖）
// 对齐 Go manager/internal/handler/payment.go + payment_provider.go（Payoneer/Stripe 已删除决策，不迁移）

import { md5Hex } from './md5';

export type PaymentOrderInput = {
  id: number; // orders.id
  userId: number;
  amount: number;
};

export type PaymentEnv = {
  BEPUSDT_API_URL?: string;
  BEPUSDT_TOKEN?: string;
  BEPUSDT_SECRET?: string;
  PAYPAL_CLIENT_ID?: string;
  PAYPAL_CLIENT_SECRET?: string;
  PAYPAL_WEBHOOK_ID?: string;
  // 沙箱对拍旋钮（§5.2）：默认生产 https://api-m.paypal.com，沙箱设 https://api-m.sandbox.paypal.com
  PAYPAL_API_BASE?: string;
};

type CallbackResult = {
  orderId: string;
  status: 'paid' | 'failed' | 'pending';
  transactionId: string;
};

const GB_BYTES = 1073741824;
const SUBSCRIPTION_DURATION_SECONDS = 30 * 86400; // Go subscriptionDurationSeconds：时长与产品无关，固定 30 天

// ── BEpusdt ─────────────────────────────────────────────────────────────────

// Go bepusdtSign：非空参数（排除 signature）按 ASCII 排序 k=v& 拼接 + token → 小写 MD5
export function bepusdtSign(params: Record<string, string>, token: string): string {
  const keys = Object.keys(params)
    .filter((k) => k !== 'signature' && params[k] !== '')
    .sort(); // JS 默认排序 = UTF-16 码元序，对 ASCII 键即 Go sort.Strings
  return md5Hex(keys.map((k) => `${k}=${params[k]}`).join('&') + token);
}

// Go strconv.FormatFloat(f, 'f', -1, 64)：最短十进制表示。JS String() 同为最短往返表示，
// 唯一差异是 ≥1e21 走科学计数法——金额域不可能出现（ponytail: 需要绝对等价时手写定点格式化）
function formatAmount(amount: number): string {
  return String(amount);
}

async function bepusdtCreatePayment(
  order: PaymentOrderInput,
  env: PaymentEnv,
  notifyURL: string,
  redirectURL: string,
): Promise<string> {
  const host = (env.BEPUSDT_API_URL ?? '').replace(/\/+$/, '');
  const token = env.BEPUSDT_TOKEN ?? '';
  if (!host || !token) {
    throw new Error('bepusdt: BEPUSDT_API_URL and BEPUSDT_TOKEN are required');
  }

  const params: Record<string, string> = {
    order_id: String(order.id),
    amount: formatAmount(order.amount),
    notify_url: notifyURL,
    redirect_url: redirectURL,
  };
  params.signature = bepusdtSign(params, token);

  const resp = await fetch(`${host}/api/v1/order/create-transaction`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(params),
    signal: AbortSignal.timeout(15000), // Go http.Client{Timeout: 15s}
  });
  const api = (await resp.json().catch(() => null)) as {
    status?: boolean;
    message?: string;
    data?: { trade_id?: string; url?: string };
  } | null;
  if (!api) throw new Error('bepusdt: decode response failed');
  if (!api.status || !api.data?.url) {
    throw new Error(`bepusdt: order rejected: ${api.message ?? ''}`);
  }
  return api.data.url;
}

async function bepusdtVerifyCallback(
  body: Record<string, unknown>,
  env: PaymentEnv,
): Promise<CallbackResult> {
  // Fail closed：无 secret（回落 token，与 Go 一致）直接拒收
  const secret = env.BEPUSDT_SECRET || env.BEPUSDT_TOKEN || '';
  if (!secret) throw new Error('BEPUSDT_SECRET not configured');

  const s = (v: unknown): string => (v == null ? '' : String(v)); // Go 零值 "" 语义
  const params: Record<string, string> = {
    order_id: s(body.order_id),
    amount: s(body.amount),
    actual_amount: s(body.actual_amount),
    token: s(body.token),
    status: String(Number(body.status ?? 0)), // Go strconv.Itoa(int 字段)
    block_transaction_id: s(body.block_transaction_id),
    created_at: s(body.created_at),
    expired_at: s(body.expired_at),
  };
  const expected = hexBytes(bepusdtSign(params, secret));
  const provided = hexBytes(s(body.signature).toLowerCase()); // Go strings.ToLower 后 hmac.Equal
  if (provided === null || expected === null || !constantTimeEqual(provided, expected)) {
    throw new Error('invalid signature');
  }

  const status = Number(body.status ?? 0);
  return {
    orderId: params.order_id,
    transactionId: params.block_transaction_id,
    status: status === 2 ? 'paid' : status === 1 ? 'pending' : 'failed',
  };
}

// ── Mock ────────────────────────────────────────────────────────────────────

async function mockVerifyCallback(body: Record<string, unknown>): Promise<CallbackResult> {
  const orderId = Number(body.order_id);
  const status = typeof body.status === 'string' ? body.status : '';
  if (!Number.isInteger(orderId) || orderId <= 0 || status === '') {
    throw new Error('invalid callback body');
  }
  return {
    orderId: String(orderId),
    transactionId: `mock_tx_${orderId}`,
    status: status === 'paid' ? 'paid' : status === 'pending' ? 'pending' : 'failed',
  };
}

// ── PayPal（§5.2 新增通道，官方 REST，无 SDK）────────────────────────────────

// ponytail: isolate 内存缓存（token 有效期 ~3h，同 isolate 命中足够）；PaymentEnv 无 KV 绑定，
// 需要跨 isolate 共享时由主线把 CACHE 传进来改 KV
let paypalTokenCache: { token: string; expiresAt: number } | null = null;

async function paypalAccessToken(env: PaymentEnv): Promise<string> {
  if (paypalTokenCache && Date.now() < paypalTokenCache.expiresAt) {
    return paypalTokenCache.token;
  }
  const id = env.PAYPAL_CLIENT_ID ?? '';
  const secret = env.PAYPAL_CLIENT_SECRET ?? '';
  if (!id || !secret) {
    throw new Error('paypal: PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET are required');
  }
  const base = env.PAYPAL_API_BASE ?? 'https://api-m.paypal.com';
  const resp = await fetch(`${base}/v1/oauth2/token`, {
    method: 'POST',
    headers: {
      Authorization: `Basic ${btoa(`${id}:${secret}`)}`,
      'Content-Type': 'application/x-www-form-urlencoded',
    },
    body: 'grant_type=client_credentials',
    signal: AbortSignal.timeout(15000),
  });
  const data = (await resp.json().catch(() => null)) as
    | { access_token?: string; expires_in?: number }
    | null;
  if (!data?.access_token) throw new Error('paypal: failed to obtain access token');
  paypalTokenCache = {
    token: data.access_token,
    expiresAt: Date.now() + (Math.max((data.expires_in ?? 3600) - 60, 0)) * 1000,
  };
  return paypalTokenCache.token;
}

async function paypalBase(env: PaymentEnv): Promise<string> {
  return env.PAYPAL_API_BASE ?? 'https://api-m.paypal.com';
}

// PayPal 產品以 USD 計價（§5.2：付款人卡自動換匯；products.currency 留給 web.ts 下單時校驗）
async function paypalCreatePayment(order: PaymentOrderInput, env: PaymentEnv): Promise<string> {
  const token = await paypalAccessToken(env);
  const resp = await fetch(`${await paypalBase(env)}/v2/checkout/orders`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({
      intent: 'CAPTURE',
      purchase_units: [
        {
          custom_id: String(order.id), // 回调据此定位 D1 订单
          // PayPal USD 金额必须是两位小数字符串（与 BEpusdt 的最短表示不同，平台强约束）
          amount: { currency_code: 'USD', value: order.amount.toFixed(2) },
        },
      ],
    }),
    signal: AbortSignal.timeout(15000),
  });
  const data = (await resp.json().catch(() => null)) as {
    links?: { rel: string; href: string }[];
  } | null;
  const approve = data?.links?.find((l) => l.rel === 'approve')?.href;
  if (!approve) throw new Error('paypal: order rejected: no approve url');
  return approve;
}

// 验签走官方 /v1/notifications/verify-webhook-signature（免自行处理证书链，§5.2）。
// transmission_* 五元组在 HTTP headers 上——路由层合入 body._headers 传入（合约签名只收 body）。
async function paypalVerifyCallback(
  body: Record<string, unknown>,
  env: PaymentEnv,
): Promise<CallbackResult> {
  const webhookId = env.PAYPAL_WEBHOOK_ID ?? '';
  if (!webhookId) throw new Error('PAYPAL_WEBHOOK_ID not configured'); // fail closed

  const headers = (body._headers ?? {}) as Record<string, unknown>;
  const h = (name: string): string => (headers[name] == null ? '' : String(headers[name]));
  if (!h('paypal-transmission-sig') || !h('paypal-transmission-id')) {
    throw new Error('invalid signature'); // 缺验签头，等同签名无效
  }

  const event: Record<string, unknown> = { ...body };
  delete event._headers; // webhook_event 必须是 PayPal 原始事件体
  const token = await paypalAccessToken(env);
  const resp = await fetch(`${await paypalBase(env)}/v1/notifications/verify-webhook-signature`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({
      auth_algo: h('paypal-auth-algo'),
      cert_url: h('paypal-cert-url'),
      transmission_id: h('paypal-transmission-id'),
      transmission_sig: h('paypal-transmission-sig'),
      transmission_time: h('paypal-transmission-time'),
      webhook_id: webhookId,
      webhook_event: JSON.stringify(event),
    }),
    signal: AbortSignal.timeout(15000),
  });
  const data = (await resp.json().catch(() => null)) as
    | { verification_status?: string }
    | null;
  if (data?.verification_status !== 'SUCCESS') throw new Error('invalid signature');

  const eventType = typeof body.event_type === 'string' ? body.event_type : '';
  const resource = (body.resource ?? {}) as Record<string, unknown>;
  const supp = (resource.supplementary_data ?? {}) as {
    related_ids?: { order_id?: unknown };
  };
  const orderId =
    (resource.custom_id == null ? '' : String(resource.custom_id)) ||
    (supp.related_ids?.order_id == null ? '' : String(supp.related_ids.order_id));
  const transactionId = resource.id == null ? '' : String(resource.id);

  if (eventType === 'PAYMENT.CAPTURE.COMPLETED') {
    return { orderId, transactionId, status: 'paid' };
  }
  if (eventType === 'PAYMENT.CAPTURE.DENIED' || eventType === 'PAYMENT.CAPTURE.DECLINED') {
    return { orderId, transactionId, status: 'failed' };
  }
  return { orderId, transactionId, status: 'pending' }; // 其他事件不落状态（与 Go 语义一致：无分支命中→ok）
}

// ── 合约三函数 ───────────────────────────────────────────────────────────────

// 建单：调 provider REST，返回托管收银台 URL（存 orders.payment_url）
// provider: 'bepusdt' | 'paypal' | 'mock'
export async function createPaymentURL(
  order: PaymentOrderInput,
  provider: string,
  env: PaymentEnv,
  notifyURL: string,
  redirectURL: string,
): Promise<string> {
  switch (provider) {
    case 'mock':
      return '/api/v1/public/payment/callback'; // Go MockProvider.CreatePayment 原样返回
    case 'bepusdt':
      return bepusdtCreatePayment(order, env, notifyURL, redirectURL);
    case 'paypal':
      return paypalCreatePayment(order, env);
    default:
      throw new Error('invalid payment provider');
  }
}

// 回调验签 + 解析（BEpusdt MD5 fail-closed / PayPal verify-webhook-signature / mock）
export async function verifyAndParseCallback(
  provider: string,
  body: Record<string, unknown>,
  env: PaymentEnv,
): Promise<{ orderId: string; status: 'paid' | 'failed' | 'pending'; transactionId: string }> {
  switch (provider) {
    case 'mock':
      return mockVerifyCallback(body);
    case 'bepusdt':
      return bepusdtVerifyCallback(body, env);
    case 'paypal':
      return paypalVerifyCallback(body, env);
    default:
      throw new Error('invalid payment provider');
  }
}

// 订单激活事务（幂等）：orders.status pending→paid + 用户激活/顺延 expire_time
// 语义对齐 activateSubscriptionTx：expire_time = max(now, 旧值) + 产品时长
export async function activateSubscription(
  db: D1Database,
  orderId: number,
  userId: number,
  productId: number,
): Promise<void> {
  const row = await db
    .prepare(
      'SELECT o.amount AS amount, p.name AS name FROM orders o JOIN products p ON p.id = o.product_id' +
        ' WHERE o.id = ? AND o.user_id = ? AND p.id = ?',
    )
    .bind(orderId, userId, productId)
    .first<{ amount: number; name: string }>();
  if (!row) throw new Error('order or product not found');

  const now = Math.floor(Date.now() / 1000);
  // 冪等閘門：用戶更新以「訂單仍為 pending」為前提（EXISTS），且放在訂單翻轉【之前】。
  // D1 batch 是單一事務，併發重複回調（BEpusdt/PayPal 重試同時到達）時，後到的事務
  // 看到已翻轉的 status，兩條語句全部空轉；若訂單先翻、用戶無條件更新（舊順序），
  // 併發回調會對同一筆訂單雙重順延 30 天。
  // traffic_limit = int64(amount * 1GiB)，rate/used 清零，period_start = now；
  // SQL MAX() 落「顺延」語義（活躍續費 max(now, 舊值)+30d）
  await db.batch([
    db
      .prepare(
        "UPDATE users SET subscription_status='active', subscription_tier=?," +
          ' traffic_limit_bytes=?, rate_limit_bps=0, traffic_used_bytes=0, traffic_period_start=?,' +
          ' expire_time = MAX(?, COALESCE(expire_time, 0)) + ? WHERE id = ? AND EXISTS' +
          " (SELECT 1 FROM orders WHERE id = ? AND status = 'pending')",
      )
      .bind(
        row.name,
        Math.trunc(row.amount * GB_BYTES),
        now,
        now,
        SUBSCRIPTION_DURATION_SECONDS,
        userId,
        orderId,
      ),
    db
      .prepare("UPDATE orders SET status='paid', updated_at=? WHERE id=? AND status='pending'")
      .bind(new Date().toISOString(), orderId),
  ]);
}

// ── 小工具 ──────────────────────────────────────────────────────────────────

function hexBytes(hex: string): Uint8Array | null {
  if (hex.length === 0 || hex.length % 2 !== 0) return null;
  const out = new Uint8Array(hex.length / 2);
  for (let i = 0; i < out.length; i++) {
    const v = parseInt(hex.slice(i * 2, i * 2 + 2), 16);
    if (Number.isNaN(v)) return null;
    out[i] = v;
  }
  return out;
}

// 恒定时间比较（等价 Go hmac.Equal；长度不等直接 false，同长度逐字节异或）
function constantTimeEqual(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false;
  let diff = 0;
  for (let i = 0; i < a.length; i++) diff |= a[i]! ^ b[i]!;
  return diff === 0;
}
