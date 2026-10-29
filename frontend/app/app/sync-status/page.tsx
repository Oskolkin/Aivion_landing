import { redirect } from "next/navigation";

/** Technical sync diagnostics live in admin support, not in seller UI. */
export default function SyncStatusPage() {
  redirect("/app/dashboard");
}
