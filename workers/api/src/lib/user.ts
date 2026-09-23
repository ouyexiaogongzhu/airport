// SanitizedUser — portal / auth 回傳的安全子集；永不包含 password_hash / vless_uuid / ss_password / trojan_password。

export type UserRow = {
  id: number;
  username: string;
  role: string;
  status: string;
  balance: number;
  subscription_status: string;
  subscription_tier: string | null;
  traffic_limit_bytes: number;
  traffic_used_bytes: number;
  expire_time: number;
  rate_limit_bps: number;
  traffic_period_start: number;
  client_token: string | null;
  created_at: string | null;
  email?: string | null;
  phone?: string | null;
  display_name?: string | null;
  billing_address?: string | null;
};

/** SELECT 欄位清單 — auth validate / profile GET·PUT 共用 */
export const USER_PROFILE_COLS =
  'id, username, role, status, balance, subscription_status, subscription_tier, ' +
  'traffic_limit_bytes, traffic_used_bytes, expire_time, rate_limit_bps, traffic_period_start, ' +
  'client_token, created_at, email, phone, display_name, billing_address';

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export function isValidEmail(email: string): boolean {
  return email.length <= 254 && EMAIL_RE.test(email);
}

export function sanitizedUser(u: UserRow): Record<string, unknown> {
  return {
    id: u.id,
    username: u.username,
    role: u.role,
    status: u.status,
    balance: u.balance,
    subscription_status: u.subscription_status,
    subscription_tier: u.subscription_tier,
    traffic_limit_bytes: u.traffic_limit_bytes,
    traffic_used_bytes: u.traffic_used_bytes,
    expire_time: u.expire_time,
    rate_limit_bps: u.rate_limit_bps,
    traffic_period_start: u.traffic_period_start,
    client_token: u.client_token,
    created_at: u.created_at,
    email: u.email ?? null,
    phone: u.phone ?? null,
    display_name: u.display_name ?? null,
    billing_address: u.billing_address ?? null,
  };
}
