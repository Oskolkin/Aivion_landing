import type { LucideIcon } from "lucide-react";
import {
  BadgePercent,
  Boxes,
  CircleDollarSign,
  Filter,
  HeartPulse,
  Megaphone,
  Package,
  Search,
  ShoppingCart,
  Tag,
  Truck,
  Warehouse,
} from "lucide-react";

export const PRIMARY_DOMAINS = [
  "SALES",
  "STOCK",
  "PRICE",
  "ADVERTISING",
  "FUNNEL",
  "SEARCH",
  "FINANCE",
  "PROMOTION",
  "ACCOUNT_HEALTH",
  "SUPPLY",
  "CATALOG",
] as const;

export type PrimaryDomain = (typeof PRIMARY_DOMAINS)[number];

const ICONS: Record<string, LucideIcon> = {
  SALES: ShoppingCart,
  STOCK: Warehouse,
  PRICE: Tag,
  ADVERTISING: Megaphone,
  FUNNEL: Filter,
  SEARCH: Search,
  FINANCE: CircleDollarSign,
  PROMOTION: BadgePercent,
  ACCOUNT_HEALTH: HeartPulse,
  SUPPLY: Truck,
  CATALOG: Package,
};

export function domainIcon(domain?: string): LucideIcon {
  return ICONS[domain ?? ""] ?? Boxes;
}
