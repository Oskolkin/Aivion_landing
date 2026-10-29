import { notFound } from "next/navigation";
import AdminClientDetailScreen from "@/components/admin-client-detail-screen";

export default async function AdminClientPage({ params }: { params: { id: string } }) {
  const sellerAccountId = Number(params.id);
  if (!Number.isFinite(sellerAccountId) || sellerAccountId <= 0) {
    notFound();
  }
  return <AdminClientDetailScreen sellerAccountId={sellerAccountId} />;
}
