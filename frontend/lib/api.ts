import { PUBLIC_API_BASE_URL } from "@/lib/env/api-base-url";

export type ApiError = {
  error: string;
};

async function parseResponse<T>(res: Response): Promise<T> {
  const contentType = res.headers.get("content-type") || "";
  const isJson = contentType.includes("application/json");

  if (!res.ok) {
    if (isJson) {
      const data = (await res.json()) as ApiError;
      throw new Error(data.error || "Request failed");
    }
    throw new Error(`Request failed with status ${res.status}`);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  if (!isJson) {
    throw new Error("Expected JSON response");
  }

  return (await res.json()) as T;
}

export async function apiGet<T>(path: string, options?: { signal?: AbortSignal }): Promise<T> {
  const res = await fetch(`${PUBLIC_API_BASE_URL}${path}`, {
    method: "GET",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
    cache: "no-store",
    signal: options?.signal,
  });

  return parseResponse<T>(res);
}

export async function apiPost<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${PUBLIC_API_BASE_URL}${path}`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
    body: body ? JSON.stringify(body) : undefined,
  });

  return parseResponse<T>(res);
}

export async function apiPut<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${PUBLIC_API_BASE_URL}${path}`, {
    method: "PUT",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
  });

  return parseResponse<T>(res);
}

export async function apiPatch<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${PUBLIC_API_BASE_URL}${path}`, {
    method: "PATCH",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
  });

  return parseResponse<T>(res);
}

export async function apiDelete(path: string, options?: { signal?: AbortSignal }): Promise<void> {
  const res = await fetch(`${PUBLIC_API_BASE_URL}${path}`, {
    method: "DELETE",
    credentials: "include",
    signal: options?.signal,
  });
  await parseResponse<void>(res);
}

export async function apiUploadForm<T>(
  path: string,
  formData: FormData,
  options?: { signal?: AbortSignal },
): Promise<T> {
  const res = await fetch(`${PUBLIC_API_BASE_URL}${path}`, {
    method: "POST",
    credentials: "include",
    body: formData,
    signal: options?.signal,
  });
  return parseResponse<T>(res);
}
