// User-facing pricing constraints removed from MVP; redirect to Products.
import { redirect } from "next/navigation";

export default function PricingConstraintsRedirectPage() {
  redirect("/app/products");
}
