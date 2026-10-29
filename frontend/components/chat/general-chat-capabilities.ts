import {
  BarChart3,
  Download,
  LineChart,
  Megaphone,
  Package,
  type LucideIcon,
} from "lucide-react";

export type GeneralChatCapability = {
  label: string;
  icon: LucideIcon;
};

export const generalChatCapabilities: GeneralChatCapability[] = [
  { label: "Продажи", icon: BarChart3 },
  { label: "Остатки", icon: Package },
  { label: "Реклама", icon: Megaphone },
  { label: "Аналитика", icon: LineChart },
  { label: "Экспорт", icon: Download },
];

export const GENERAL_CHAT_EYEBROW = "AI-чат по кабинету";
export const GENERAL_CHAT_TITLE = "Анализ и отчёты по данным вашего кабинета";
export const GENERAL_CHAT_DESCRIPTION =
  "Учитываются актуальные данные кабинета и история диалога.";
