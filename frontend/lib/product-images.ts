export const DEMO_PRODUCT_IMAGES = [
  "/demo/products/product-demo-1.jpg",
  "/demo/products/product-demo-2.jpg",
  "/demo/products/product-demo-3.jpg",
  "/demo/products/product-demo-4.jpg",
  "/demo/products/product-demo-5.jpg",
  "/demo/products/product-demo-6.jpg",
  "/demo/products/product-demo-7.jpg",
  "/demo/products/product-demo-8.jpg",
  "/demo/products/product-demo-9.jpg",
  "/demo/products/product-demo-10.jpg",
  "/demo/products/product-demo-11.jpg",
  "/demo/products/product-demo-12.jpg",
  "/demo/products/product-demo-13.jpg",
  "/demo/products/product-demo-14.jpg",
  "/demo/products/product-demo-15.jpg",
  "/demo/products/product-demo-16.jpg",
  "/demo/products/product-demo-17.jpg",
  "/demo/products/product-demo-18.jpg",
  "/demo/products/product-demo-19.jpg",
  "/demo/products/product-demo-20.jpg",
] as const;

export function getDemoProductImage(seed: string | number | null | undefined): string {
  const normalized = String(seed ?? "default");
  let hash = 0;

  for (let i = 0; i < normalized.length; i += 1) {
    hash = (hash * 31 + normalized.charCodeAt(i)) >>> 0;
  }

  return DEMO_PRODUCT_IMAGES[hash % DEMO_PRODUCT_IMAGES.length];
}

export function resolveProductImageUrl(
  primaryImageUrl: string | null | undefined,
  seed: string | number | null | undefined,
): string {
  if (primaryImageUrl && primaryImageUrl.trim().length > 0) {
    return primaryImageUrl;
  }

  return getDemoProductImage(seed);
}

/** Stable seed: SKU → offer_id → ozon_product_id → fallback label. */
export function productImageSeed(item: {
  sku?: number | null;
  offer_id?: string | null;
  ozon_product_id?: number | null;
  product_name?: string | null;
}): string | number {
  if (item.sku != null) return item.sku;
  if (item.offer_id && item.offer_id.trim().length > 0) return item.offer_id;
  if (item.ozon_product_id != null) return item.ozon_product_id;
  if (item.product_name && item.product_name.trim().length > 0) return item.product_name;
  return "default";
}
