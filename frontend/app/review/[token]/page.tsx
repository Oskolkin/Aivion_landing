import { redirect } from "next/navigation";

type Props = {
  params: Promise<{ token: string }>;
};

/**
 * One-time review entry: hand off to backend redeem (sets HttpOnly cookie, redirects to /app/dashboard).
 * The browser URL after exchange must not retain the invite token.
 */
export default async function ReviewRedeemPage({ params }: Props) {
  const { token } = await params;
  const cleaned = (token || "").trim().toLowerCase();
  if (!/^[0-9a-f]{64}$/.test(cleaned)) {
    redirect("/login");
  }
  redirect(`/api/v1/auth/review/redeem/${cleaned}`);
}
