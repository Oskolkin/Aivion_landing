import { Suspense } from "react";
import ProductsScreen from "@/components/products-screen";

export default function ProductsPage() {
  return (
    <Suspense fallback={null}>
      <ProductsScreen />
    </Suspense>
  );
}
