"use client";

import { ActionAskAIButton } from "@/components/actions/action-ask-ai-button";

export function ActionAskAIButtonWithRouter({
  actionViewId,
  disabled,
  variant = "secondary",
}: {
  actionViewId: string;
  disabled?: boolean;
  variant?: "primary" | "secondary";
}) {
  return (
    <ActionAskAIButton
      actionViewId={actionViewId}
      disabled={disabled}
      variant={variant}
      label="Спросить AI"
      hint="Получить уточнения и рекомендации"
    />
  );
}
