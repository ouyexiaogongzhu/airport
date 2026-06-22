/**
 * RFPlay Airport Manager Auth
 * Login/logout, token management, route guard
 */

const AUTH = (() => {
  const TOKEN_KEY = 'rfplay_token';
  const USER_KEY = 'rfplay_user';

  // ─── Token management ─────────────────────────────────────

  function getToken() {
    return localStorage.getItem(TOKEN_KEY);
  }

  function getUser() {
    try {
      return JSON.parse(localStorage.getItem(USER_KEY) || 'null');
    } catch {
      return null;
    }
  }

  function isAuthenticated() {
    return !!getToken();
  }

  function setSession(token, user) {
    localStorage.setItem(TOKEN_KEY, token);
    localStorage.setItem(USER_KEY, JSON.stringify(user));
  }

  function clearSession() {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
  }

  // ─── Login ────────────────────────────────────────────────

  async function login(username, password) {
    const res = await API.request('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });

    if (res.code === 0 && res.data && res.data.token) {
      setSession(res.data.token, res.data.user);
      return { ok: true, user: res.data.user };
    }

    return { ok: false, error: res.message || 'Login failed' };
  }

  function logout() {
    clearSession();
    Router.go('login');
  }

  // ─── Route guard ──────────────────────────────────────────

  function guard() {
    const publicPages = ['login'];
    const page = Router.current();

    if (!isAuthenticated() && !publicPages.includes(page)) {
      Router.go('login');
      return false;
    }

    if (isAuthenticated() && page === 'login') {
      Router.go('dashboard');
      return false;
    }

    return true;
  }

  // ─── Expose ───────────────────────────────────────────────

  return { getToken, getUser, isAuthenticated, login, logout, guard };
})();
