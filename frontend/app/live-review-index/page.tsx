import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { getCurrentUserServer } from "@/lib/server-auth";

export const dynamic = "force-dynamic";

const PRIMARY_LINKS = [
  { href: "/app/dashboard", label: "Dashboard" },
  { href: "/app/actions", label: "Actions" },
  { href: "/app/chat", label: "AI Chat" },
  { href: "/app/account", label: "Account" },
  { href: "/app/integrations/ozon", label: "Ozon connection" },
] as const;

const LEGACY_LINKS = [
  { href: "/app/products", label: "Products" },
  { href: "/app/advertising", label: "Advertising" },
  { href: "/app/critical-skus", label: "Critical SKUs" },
  { href: "/app/stocks-replenishment", label: "Stocks replenishment" },
  { href: "/app/alerts", label: "Alerts" },
  { href: "/app/recommendations", label: "Recommendations" },
] as const;

export default async function LiveReviewIndexPage() {
  const auth = await getCurrentUserServer();
  if (!auth) {
    redirect("/login");
  }
  if (!auth.is_live_review) {
    notFound();
  }

  return (
    <main className="mx-auto max-w-2xl space-y-8 p-8 text-aivion-text">
      <header className="space-y-2">
        <h1 className="text-2xl font-semibold">Live Seller Review</h1>
        <p className="text-sm text-aivion-muted">
          Technical navigation index for the temporary live seller review session. Each link opens
          the real application screen.
        </p>
      </header>

      <section className="space-y-3">
        <h2 className="text-lg font-medium">Seller sections</h2>
        <ul className="list-disc space-y-2 pl-5">
          {PRIMARY_LINKS.map((item) => (
            <li key={item.href}>
              <Link href={item.href} className="text-aivion-blue underline-offset-2 hover:underline">
                {item.label}
              </Link>
            </li>
          ))}
        </ul>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-medium">Legacy routes, не входящие в текущее меню</h2>
        <ul className="list-disc space-y-2 pl-5">
          {LEGACY_LINKS.map((item) => (
            <li key={item.href}>
              <Link href={item.href} className="text-aivion-blue underline-offset-2 hover:underline">
                {item.label}
              </Link>
            </li>
          ))}
        </ul>
      </section>
    </main>
  );
}
