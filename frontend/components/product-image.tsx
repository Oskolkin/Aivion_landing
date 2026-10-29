"use client";

import { useState, type ReactNode } from "react";
import { ImageIcon } from "lucide-react";
import { cn } from "@/components/ui/cn";
import { productImageSeed, resolveProductImageUrl } from "@/lib/product-images";

type ProductImageVariant = "card" | "thumb" | "product-card";

type ProductImageProps = {
  imageUrl?: string | null;
  sku?: number | null;
  offerId?: string | null;
  ozonProductId?: number | null;
  productName?: string | null;
  alt?: string;
  imgClassName?: string;
  placeholder?: "card" | "thumb";
  variant?: ProductImageVariant;
  /** When false, show placeholder instead of demo product photos. */
  useDemoFallback?: boolean;
};

function ProductImagePlaceholder({
  variant,
  placeholder,
}: {
  variant: ProductImageVariant;
  placeholder: "card" | "thumb";
}) {
  const compact = variant === "product-card" || placeholder === "thumb";
  return (
    <div
      className={cn(
        "flex h-full w-full items-center justify-center text-[#94A3B8]",
        variant === "product-card" ? "bg-[#F8FAFC]" : "flex-col gap-1 text-aivion-muted-soft",
      )}
      aria-hidden
    >
      <ImageIcon className={compact ? "h-5 w-5" : "h-10 w-10"} strokeWidth={1.5} aria-hidden="true" />
      {!compact ? <span className="text-xs">Нет фото</span> : null}
    </div>
  );
}

function ProductImageFrame({
  variant,
  children,
}: {
  variant: ProductImageVariant;
  children: ReactNode;
}) {
  if (variant !== "product-card") {
    return <>{children}</>;
  }

  return (
    <div
      className="h-16 w-16 shrink-0 overflow-hidden rounded-xl border border-[#E2E8F0] bg-[#F8FAFC]"
      data-testid="product-image-product-card"
    >
      {children}
    </div>
  );
}

export function ProductImage({
  imageUrl,
  sku,
  offerId,
  ozonProductId,
  productName,
  alt = "",
  imgClassName = "h-full w-full object-cover",
  placeholder = "card",
  variant = "card",
  useDemoFallback = true,
}: ProductImageProps) {
  const seed = productImageSeed({
    sku,
    offer_id: offerId,
    ozon_product_id: ozonProductId,
    product_name: productName,
  });
  const hasPrimaryImage = Boolean(imageUrl?.trim());
  const resolvedSrc = useDemoFallback
    ? resolveProductImageUrl(imageUrl, seed)
    : imageUrl?.trim() ?? "";
  const [broken, setBroken] = useState(false);

  const content =
    !hasPrimaryImage && !useDemoFallback ? (
      <ProductImagePlaceholder variant={variant} placeholder={placeholder} />
    ) : broken ? (
      <ProductImagePlaceholder variant={variant} placeholder={placeholder} />
    ) : (
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={resolvedSrc}
        alt={alt}
        className={imgClassName}
        loading="lazy"
        decoding="async"
        referrerPolicy={/^https?:\/\//i.test(resolvedSrc) ? "no-referrer" : undefined}
        onError={() => setBroken(true)}
      />
    );

  return <ProductImageFrame variant={variant}>{content}</ProductImageFrame>;
}
