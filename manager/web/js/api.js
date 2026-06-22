/**
 * RFPlay Airport Manager API client
 * Fetch wrapper with JWT auth, base URL, and mock fallback
 */

const API = (() => {
  const BASE_URL = 'http://localhost:8080/api/v1';

  // ─── Mock data ───────────────────────────────────────────
  const MOCK = {
    login: { token: 'mock-jwt-token-abc123', user: { id: 1, username: 'admin', role: 'admin' } },
    dashboard: {
      users: 1284,
      nodes: 56,
      orders: 892,
      revenue: 45820,
      activeNodes: 42,
      pendingOrders: 23,
      usersTrend: 12,
      nodesTrend: 5,
      ordersTrend: -3,
      revenueTrend: 8,
    },
    nodes: [
      { id: 1, name: 'LHR-Gate-A12', type: 'gateway', status: 'online', ip: '10.0.1.12', location: 'London Heathrow', uptime: '99.97%', lastSeen: '2026-06-22T10:30:00Z' },
      { id: 2, name: 'JFK-Terminal-4', type: 'gateway', status: 'online', ip: '10.0.2.4', location: 'New York JFK', uptime: '99.82%', lastSeen: '2026-06-22T10:29:00Z' },
      { id: 3, name: 'DXB-Checkin-B', type: 'kiosk', status: 'online', ip: '10.0.3.7', location: 'Dubai International', uptime: '98.45%', lastSeen: '2026-06-22T10:28:00Z' },
      { id: 4, name: 'HKG-Arrival-S5', type: 'gateway', status: 'offline', ip: '10.0.4.19', location: 'Hong Kong International', uptime: '87.33%', lastSeen: '2026-06-21T22:15:00Z' },
      { id: 5, name: 'CDG-2E-Gate', type: 'kiosk', status: 'maintenance', ip: '10.0.5.3', location: 'Paris Charles de Gaulle', uptime: '92.10%', lastSeen: '2026-06-22T08:00:00Z' },
      { id: 6, name: 'SIN-Terminal-3', type: 'gateway', status: 'online', ip: '10.0.6.21', location: 'Singapore Changi', uptime: '99.91%', lastSeen: '2026-06-22T10:30:00Z' },
      { id: 7, name: 'LAX-TBIT-5', type: 'scanner', status: 'online', ip: '10.0.7.14', location: 'Los Angeles International', uptime: '99.54%', lastSeen: '2026-06-22T10:27:00Z' },
      { id: 8, name: 'FRA-Z-62', type: 'gateway', status: 'offline', ip: '10.0.8.9', location: 'Frankfurt Airport', uptime: '76.88%', lastSeen: '2026-06-20T14:45:00Z' },
      { id: 9, name: 'IST-Checkin-D', type: 'kiosk', status: 'online', ip: '10.0.9.11', location: 'Istanbul Airport', uptime: '98.76%', lastSeen: '2026-06-22T10:30:00Z' },
      { id: 10, name: 'AMS-Pier-G', type: 'gateway', status: 'maintenance', ip: '10.0.10.5', location: 'Amsterdam Schiphol', uptime: '95.20%', lastSeen: '2026-06-22T07:30:00Z' },
      { id: 11, name: 'NRT-Satellite-2', type: 'scanner', status: 'online', ip: '10.0.11.8', location: 'Tokyo Narita', uptime: '99.88%', lastSeen: '2026-06-22T10:25:00Z' },
    ],
    users: [
      { id: 1, username: 'admin', email: 'admin@rfplay.uk', role: 'admin', status: 'active', created: '2025-01-15', orders: 0 },
      { id: 2, username: 'j.smith', email: 'j.smith@rfplay.uk', role: 'operator', status: 'active', created: '2025-03-20', orders: 47 },
      { id: 3, username: 'm.jones', email: 'm.jones@rfplay.uk', role: 'operator', status: 'active', created: '2025-04-02', orders: 33 },
      { id: 4, username: 'l.chen', email: 'l.chen@rfplay.uk', role: 'viewer', status: 'active', created: '2025-05-10', orders: 0 },
      { id: 5, username: 'r.patel', email: 'r.patel@rfplay.uk', role: 'operator', status: 'suspended', created: '2025-06-18', orders: 12 },
      { id: 6, username: 'a.williams', email: 'a.williams@rfplay.uk', role: 'viewer', status: 'active', created: '2025-07-01', orders: 0 },
      { id: 7, username: 'k.tanaka', email: 'k.tanaka@rfplay.uk', role: 'operator', status: 'inactive', created: '2025-08-14', orders: 5 },
      { id: 8, username: 'd.muller', email: 'd.muller@rfplay.uk', role: 'operator', status: 'active', created: '2025-09-22', orders: 28 },
      { id: 9, username: 's.nguyen', email: 's.nguyen@rfplay.uk', role: 'viewer', status: 'active', created: '2025-10-05', orders: 0 },
      { id: 10, username: 'p.garcia', email: 'p.garcia@rfplay.uk', role: 'operator', status: 'suspended', created: '2025-11-11', orders: 7 },
    ],
    orders: [
      { id: 'ORD-2026-001', userId: 2, username: 'j.smith', service: 'airport-wifi', status: 'active', amount: 299.00, created: '2026-06-01', expires: '2026-12-01' },
      { id: 'ORD-2026-002', userId: 3, username: 'm.jones', service: 'gateway-pro', status: 'active', amount: 599.00, created: '2026-06-03', expires: '2026-12-03' },
      { id: 'ORD-2026-003', userId: 2, username: 'j.smith', service: 'analytics-basic', status: 'active', amount: 149.00, created: '2026-06-05', expires: '2026-09-05' },
      { id: 'ORD-2026-004', userId: 8, username: 'd.muller', service: 'airport-wifi', status: 'pending', amount: 299.00, created: '2026-06-10', expires: '2026-12-10' },
      { id: 'ORD-2026-005', userId: 5, username: 'r.patel', service: 'gateway-pro', status: 'cancelled', amount: 599.00, created: '2026-06-12', expires: null },
      { id: 'ORD-2026-006', userId: 3, username: 'm.jones', service: 'premium-support', status: 'active', amount: 899.00, created: '2026-06-14', expires: '2026-12-14' },
      { id: 'ORD-2026-007', userId: 10, username: 'p.garcia', service: 'airport-wifi', status: 'expired', amount: 299.00, created: '2025-12-01', expires: '2026-06-01' },
      { id: 'ORD-2026-008', userId: 7, username: 'k.tanaka', service: 'analytics-advanced', status: 'active', amount: 449.00, created: '2026-06-16', expires: '2026-12-16' },
      { id: 'ORD-2026-009', userId: 2, username: 'j.smith', service: 'gateway-pro', status: 'pending', amount: 599.00, created: '2026-06-18', expires: '2026-12-18' },
      { id: 'ORD-2026-010', userId: 8, username: 'd.muller', service: 'premium-support', status: 'active', amount: 899.00, created: '2026-06-20', expires: '2026-12-20' },
      { id: 'ORD-2026-011', userId: 4, username: 'l.chen', service: 'airport-wifi', status: 'pending', amount: 299.00, created: '2026-06-21', expires: '2026-12-21' },
      { id: 'ORD-2026-012', userId: 6, username: 'a.williams', service: 'analytics-basic', status: 'active', amount: 149.00, created: '2026-06-15', expires: '2026-09-15' },
    ],
  };

  // ─── Helpers ──────────────────────────────────────────────

  function getToken() {
    return localStorage.getItem('rfplay_token');
  }

  function simulateDelay(ms = 400) {
    return new Promise(r => setTimeout(r, ms));
  }

  function mockResponse(data) {
    return { ok: true, status: 200, json: () => Promise.resolve({ code: 0, message: 'success', data }) };
  }

  // ─── Public API ───────────────────────────────────────────

  async function request(endpoint, options = {}) {
    const token = getToken();
    const headers = { 'Content-Type': 'application/json', ...options.headers };

    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    try {
      const url = `${BASE_URL}${endpoint}`;
      const resp = await fetch(url, { ...options, headers, signal: AbortSignal.timeout(5000) });
      return await resp.json();
    } catch (err) {
      console.warn(`[API] fetch failed for ${endpoint}, falling back to mock:`, err.message);
      await simulateDelay();
      return mockHandler(endpoint, options);
    }
  }

  function mockHandler(endpoint, options) {
    const method = (options.method || 'GET').toUpperCase();

    // Login
    if (endpoint === '/auth/login' && method === 'POST') {
      const body = JSON.parse(options.body || '{}');
      if (body.username && body.password) {
        return { code: 0, message: 'success', data: { ...MOCK.login, user: { ...MOCK.login.user, username: body.username } } };
      }
      return { code: 400, message: 'username and password required' };
    }

    // Dashboard
    if (endpoint === '/dashboard' && method === 'GET') {
      return { code: 0, message: 'success', data: MOCK.dashboard };
    }

    // Nodes list
    if (endpoint.startsWith('/nodes') && method === 'GET') {
      return { code: 0, message: 'success', data: MOCK.nodes };
    }

    // Update node
    if (endpoint.match(/^\/nodes\/\d+$/) && method === 'PUT') {
      const id = parseInt(endpoint.split('/')[2]);
      const body = JSON.parse(options.body || '{}');
      const idx = MOCK.nodes.findIndex(n => n.id === id);
      if (idx >= 0) {
        MOCK.nodes[idx] = { ...MOCK.nodes[idx], ...body };
        return { code: 0, message: 'updated', data: MOCK.nodes[idx] };
      }
      return { code: 404, message: 'node not found' };
    }

    // Delete node
    if (endpoint.match(/^\/nodes\/\d+$/) && method === 'DELETE') {
      return { code: 0, message: 'deleted' };
    }

    // Create node
    if (endpoint === '/nodes' && method === 'POST') {
      const body = JSON.parse(options.body || '{}');
      const newNode = { id: Date.now(), ...body, status: 'offline', uptime: '0%', lastSeen: new Date().toISOString() };
      MOCK.nodes.push(newNode);
      return { code: 0, message: 'created', data: newNode };
    }

    // Users list
    if (endpoint.startsWith('/users') && method === 'GET') {
      // filter
      let result = [...MOCK.users];
      const url = new URL(endpoint, 'http://x');
      const search = url.searchParams.get('search')?.toLowerCase();
      if (search) {
        result = result.filter(u =>
          u.username.toLowerCase().includes(search) ||
          u.email.toLowerCase().includes(search)
        );
      }
      return { code: 0, message: 'success', data: result };
    }

    // Update user (status toggle)
    if (endpoint.match(/^\/users\/\d+$/) && method === 'PUT') {
      const id = parseInt(endpoint.split('/')[2]);
      const body = JSON.parse(options.body || '{}');
      const idx = MOCK.users.findIndex(u => u.id === id);
      if (idx >= 0) {
        MOCK.users[idx] = { ...MOCK.users[idx], ...body };
        return { code: 0, message: 'updated', data: MOCK.users[idx] };
      }
      return { code: 404, message: 'user not found' };
    }

    // Orders list
    if (endpoint.startsWith('/orders') && method === 'GET') {
      let result = [...MOCK.orders];
      const url = new URL(endpoint, 'http://x');
      const status = url.searchParams.get('status');
      const search = url.searchParams.get('search')?.toLowerCase();
      if (status) result = result.filter(o => o.status === status);
      if (search) {
        result = result.filter(o =>
          o.id.toLowerCase().includes(search) ||
          o.username.toLowerCase().includes(search)
        );
      }
      return { code: 0, message: 'success', data: result };
    }

    console.warn(`[API] No mock for ${method} ${endpoint}`);
    return { code: 404, message: 'not found' };
  }

  // ─── Expose ───────────────────────────────────────────────

  return { request, getToken, MOCK };
})();
