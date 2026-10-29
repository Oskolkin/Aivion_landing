import { redirect } from "next/navigation";

/** MVP technical home is not user-facing; sellers land on dashboard. */
export default function AppPage() {
  redirect("/app/dashboard");
}
