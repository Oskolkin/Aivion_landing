import { renderToStaticMarkup } from "react-dom/server";
import { WidgetState, isWidgetRenderable } from "@/components/dashboard/widget-state";
import type { WidgetBase, WidgetStatus } from "@/lib/dashboard-home-types";

type WidgetStateTone = "neutral" | "info" | "warning" | "danger";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function render(status: WidgetStatus, message?: string): string {
  return renderToStaticMarkup(
    <WidgetState widget={{ status, message } as WidgetBase}>
      <div>WIDGET_CONTENT</div>
    </WidgetState>,
  );
}

function run() {
  const okHtml = render("available");
  assert(okHtml.includes("WIDGET_CONTENT"), "available renders children");
  assert(!okHtml.includes("data-widget-state-tone"), "available has no placeholder");

  const partialHtml = render("partial");
  assert(partialHtml.includes("WIDGET_CONTENT"), "partial renders children");
  assert(partialHtml.includes('data-widget-state-tone="info"'), "partial uses info tone");
  assert(partialHtml.includes("Данные частично доступны"), "partial default copy");
  assert(!partialHtml.includes("partial"), "partial raw status not shown");
  assert(!partialHtml.includes("Не удалось загрузить блок"), "partial is not error");

  const partialCustom = render("partial", "Часть рекламных метрик за выбранный период недоступна.");
  assert(partialCustom.includes("Часть рекламных метрик"), "partial uses backend message");

  const toneCases: Array<[WidgetStatus, WidgetStateTone]> = [
    ["not_enough_data", "neutral"],
    ["source_pending", "info"],
    ["source_access_denied", "warning"],
    ["premium_required", "warning"],
    ["source_not_available", "warning"],
    ["error", "danger"],
  ];
  for (const [status, tone] of toneCases) {
    const html = render(status);
    assert(html.includes(`data-widget-state-tone="${tone}"`), `${status} -> tone ${tone}`);
    assert(!html.includes("WIDGET_CONTENT"), `${status} hides children`);
    assert(!html.includes(status), `raw status key not shown for ${status}`);
    assert(/<svg/.test(html), `${status} renders an svg icon`);
    assert(!/[ⓘ⚠✓✔✕]/.test(html), `${status} no unicode glyph icons`);
  }

  assert(render("error").includes('role="status"'), "error has role=status");
  assert(!render("not_enough_data").includes('role="status"'), "neutral has no role=status");
  assert(partialHtml.includes('role="status"'), "partial notice is announced");

  assert(render("not_enough_data").includes("Данных пока недостаточно."), "not_enough_data copy");
  assert(render("source_pending").includes("Данные скоро появятся."), "pending copy");
  assert(render("source_access_denied").includes("Доступ к данным ограничен."), "access copy");
  assert(render("premium_required").includes("Требуется расширенный тариф Ozon."), "premium copy");
  assert(render("source_not_available").includes("Источник данных не подключён."), "source copy");
  assert(render("error").includes("Не удалось загрузить блок."), "error copy");
  assert(render("error", "Своя ошибка интеграции").includes("Своя ошибка интеграции"), "custom message preserved");

  assert(isWidgetRenderable("partial"), "partial is renderable");
  assert(isWidgetRenderable("available"), "available is renderable");
  assert(!isWidgetRenderable("error"), "error is not renderable");

  console.log("widget-state.test.tsx: ok");
}

run();
