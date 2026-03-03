import axios from "axios";
import type { AxiosInstance } from "axios";

const baseURL =
  import.meta.env.VITE_VM_SERVICE_BASE_URL?.toString() ??
  import.meta.env.VITE_API_URL?.toString() ??
  "";

const api: AxiosInstance = axios.create({
  baseURL,
  headers: {
    "Content-Type": "application/json",
  },
  timeout: 10000,
});

export type ApiError = {
  message?: string;
  error?: string;
};

export function isRequestCanceled(error: unknown): boolean {
  if (axios.isCancel(error)) {
    return true;
  }
  if (axios.isAxiosError(error) && error.code === "ERR_CANCELED") {
    return true;
  }
  return false;
}

export function getErrorMessage(error: unknown, fallback = "Request failed") {
  if (isRequestCanceled(error)) {
    return "";
  }
  if (axios.isAxiosError(error)) {
    const data = error.response?.data as ApiError | undefined;
    return data?.error ?? data?.message ?? error.message ?? fallback;
  }
  if (error instanceof Error) {
    return error.message || fallback;
  }
  return fallback;
}

export default api;
