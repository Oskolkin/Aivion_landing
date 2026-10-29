/**
 * DOM structure checks for diverging comparison bars.
 */
import { renderToStaticMarkup } from "react-dom/server";
import { FunnelComparisonBars } from "@/components/dashboard/funnel-comparison-bars";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

const html = renderToStaticMarkup(
  <FunnelComparisonBars
    title="К прошлой сводке"
    items={[
      { key: "impressions", label: "Показы", delta: "-4,24%", tone: "negative" },
      { key: "add_to_cart_to_orders", label: "Корзина → Заказанные единицы", delta: "+9,64 п.п.", tone: "positive" },
    ]}
    groups={[
      {
        heading: "Этапы",
        items: [{ key: "impressions", label: "Показы", delta: "-4,24%", tone: "negative" }],
      },
      {
        heading: "Конверсии",
        items: [
          {
            key: "add_to_cart_to_orders",
            label: "Корзина → Заказанные единицы",
            delta: "+9,64 п.п.",
            tone: "positive",
          },
        ],
      },
    ]}
  />,
);

assert(html.includes('data-testid="funnel-comparison-bar-grid"'), "shared bar grid");
assert(html.includes('data-testid="funnel-zero-axis"'), "zero axis present");
assert(html.includes('data-testid="funnel-bar-negative"'), "negative bar");
assert(html.includes('data-testid="funnel-bar-positive"'), "positive bar");
assert(html.includes('data-testid="funnel-comparison-value"'), "value column");
assert(html.includes("+9,64 п.п."), "pp value rendered");

const trackIdx = html.indexOf('data-testid="funnel-chart-area"');
const valueIdx = html.indexOf('data-testid="funnel-comparison-value"');
assert(trackIdx > 0 && valueIdx > trackIdx, "value column follows bar track, not inside it");
