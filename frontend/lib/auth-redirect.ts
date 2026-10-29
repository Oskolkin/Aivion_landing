import type { AuthResponse } from "@/lib/auth-types";

export function getPostLoginPath(auth: AuthResponse): string | null {
  if (auth.seller_account) {
    return "/app";
  }
  if (auth.is_admin) {
    return "/app/admin";
  }
  return null;
}

export function isAdminOnlyUser(auth: AuthResponse): boolean {
  return auth.is_admin && auth.seller_account == null;
}
