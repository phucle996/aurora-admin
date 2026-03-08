import axios from "axios";

export const ADMIN_SESSION_STORAGE = "admin-ui:session-authenticated";
const ENABLED_MODULES_CACHE_KEY = "admin-ui:enabled-modules-cache:v1";
let globalUnauthorizedInterceptorAttached = false;
let redirectingToLogin = false;

type LoginResponsePayload = {
  data?: {
    authenticated?: unknown;
  };
  message?: string;
  error?: string;
};

export function resolveAdminBaseURL(): string {
  const adminBaseURL = import.meta.env.VITE_ADMIN_API_BASE_URL?.toString()?.trim();
  if (adminBaseURL) {
    return adminBaseURL.replace(/\/+$/, "");
  }
  if (typeof window !== "undefined" && window.location?.origin) {
    return window.location.origin;
  }
  return "";
}

export function hasAdminSession(): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  return localStorage.getItem(ADMIN_SESSION_STORAGE) === "1";
}

export function setAdminSession(active: boolean): void {
  if (typeof window === "undefined") {
    return;
  }
  if (active) {
    localStorage.setItem(ADMIN_SESSION_STORAGE, "1");
    return;
  }
  localStorage.removeItem(ADMIN_SESSION_STORAGE);
  localStorage.removeItem(ENABLED_MODULES_CACHE_KEY);
}

export async function loginWithAPIKey(apiKey: string): Promise<void> {
  const normalized = apiKey.trim();
  if (!normalized) {
    throw new Error("Admin key không được để trống");
  }

  const response = await axios.post<LoginResponsePayload>(
    `${resolveAdminBaseURL()}/api/v1/apikey/login`,
    {
      api_key: normalized,
    },
    {
      withCredentials: true,
      headers: {
        "Content-Type": "application/json",
      },
      timeout: 10000,
    },
  );

  const authenticated = Boolean(response.data?.data?.authenticated);
  if (!authenticated) {
    throw new Error(response.data?.message || "Xác thực thất bại");
  }
}

export function getAdminAuthErrorMessage(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const payload = error.response?.data as LoginResponsePayload | undefined;
    return payload?.message || payload?.error || error.message || "Đăng nhập thất bại";
  }
  if (error instanceof Error) {
    return error.message || "Đăng nhập thất bại";
  }
  return "Đăng nhập thất bại";
}

function shouldRedirectForUnauthorized(error: unknown): boolean {
  if (!axios.isAxiosError(error)) {
    return false;
  }

  const status = error.response?.status;
  if (status !== 401 && status !== 403) {
    return false;
  }

  const requestURL = (error.config?.url ?? "").toString();
  if (requestURL.includes("/api/v1/apikey/login")) {
    return false;
  }

  return true;
}

export function handleUnauthorizedError(error: unknown): void {
  if (typeof window === "undefined") {
    return;
  }
  if (!shouldRedirectForUnauthorized(error)) {
    return;
  }

  setAdminSession(false);

  if (window.location.pathname === "/login") {
    return;
  }
  if (redirectingToLogin) {
    return;
  }
  redirectingToLogin = true;
  window.location.replace("/login");
}

export function setupGlobalUnauthorizedInterceptor(): void {
  if (globalUnauthorizedInterceptorAttached) {
    return;
  }
  globalUnauthorizedInterceptorAttached = true;

  axios.interceptors.response.use(
    (response) => response,
    (error) => {
      handleUnauthorizedError(error);
      return Promise.reject(error);
    },
  );
}
