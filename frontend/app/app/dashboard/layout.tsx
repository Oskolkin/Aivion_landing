import { DashboardShell } from "@/components/dashboard/shell/dashboard-shell";

export default function DashboardSectionLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return <DashboardShell>{children}</DashboardShell>;
}
