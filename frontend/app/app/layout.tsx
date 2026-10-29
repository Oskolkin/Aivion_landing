import { redirect } from "next/navigation";
import AppAuthRedirect from "@/components/app-auth-redirect";
import AppShell from "@/components/app-shell";
import ConnectionGuard from "@/components/connection-guard";
import { isAdminOnlyUser } from "@/lib/auth-redirect";
import { getCurrentUserServer } from "@/lib/server-auth";

export default async function AppSectionLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const auth = await getCurrentUserServer();
  if (!auth) {
    redirect("/login");
  }

  const isLiveReview = Boolean(auth.is_live_review);
  const isClassicReviewMode =
    Boolean(auth.is_review_mode || auth.read_only) && !isLiveReview;
  const visualDemoSeller = Boolean(auth.is_demo_seller) && !isLiveReview;
  const adminOnly = isAdminOnlyUser(auth) && !isClassicReviewMode && !isLiveReview;

  return (
    <>
      <AppAuthRedirect
        isAdminOnly={adminOnly}
        isReviewMode={isClassicReviewMode || isLiveReview}
      />
      <AppShell
        userEmail={auth.user.email}
        sellerAccountName={auth.seller_account?.name ?? null}
        isAdminOnly={adminOnly}
        isAdmin={Boolean(auth.is_admin) && !isClassicReviewMode && !isLiveReview}
        isDemoSeller={visualDemoSeller}
        isReviewMode={isClassicReviewMode}
      >
        <ConnectionGuard
          isAdminOnly={adminOnly}
          isDemoSeller={
            Boolean(auth.is_demo_seller) || isClassicReviewMode || isLiveReview
          }
        >
          {children}
        </ConnectionGuard>
      </AppShell>
    </>
  );
}
