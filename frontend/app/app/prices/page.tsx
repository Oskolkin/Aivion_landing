import { redirect } from "next/navigation";

export default function PricesRedirectPage() {
  redirect("/app/products");
}
