// product.price 是主幣單位（元 / 美元），與後台錄入和支付金額一致
const SYMBOLS: Record<string, string> = { USD: '$', CNY: '¥' }

export function formatPrice(amount: number, currency?: string | null): string {
  const symbol = SYMBOLS[(currency || 'USD').toUpperCase()] ?? `${currency} `
  return `${symbol}${Number(amount || 0).toFixed(2)}`
}
