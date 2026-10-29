import assert from "node:assert/strict";
import {
  buildYTicks,
  chooseNiceStep,
  computeDRRChartYDomain,
  dedupeYTicksByRoundedLabel,
  getAverageLabelPlacement,
  getDRRChartDomain,
  getPointLabelPlacement,
  plotHeightPx,
  shouldShowPointLabel,
  Y_TICK_MIN_GAP_PX,
} from "@/lib/ad-efficiency-chart-layout";
import { formatDrrPercent } from "@/lib/ad-efficiency-display";

function assertNoOverlappingYLabels(
  ticks: number[],
  yMin: number,
  yMax: number,
  plotHeight: number,
) {
  const span = yMax - yMin;
  for (let i = 1; i < ticks.length; i++) {
    const gapPx = span > 0 ? (Math.abs(ticks[i] - ticks[i - 1]) / span) * plotHeight : Y_TICK_MIN_GAP_PX;
    assert.ok(
      gapPx >= Y_TICK_MIN_GAP_PX - 0.5,
      `ticks ${ticks[i - 1]} and ${ticks[i]} gap ${gapPx}px < ${Y_TICK_MIN_GAP_PX}`,
    );
    assert.notEqual(
      formatDrrPercent(ticks[i - 1]),
      formatDrrPercent(ticks[i]),
      `duplicate labels for ${ticks[i - 1]} and ${ticks[i]}`,
    );
  }
}

function run() {
  const plotHeight = plotHeightPx();

  // Typical seller-1 range: must NOT be static 0–100%.
  const typicalSeries = [12.2, 14.9, 17, 18.5, 18.8, 19.4, 20.1];
  const typicalAvg = 16.8;
  const typicalDomain = computeDRRChartYDomain(typicalSeries, typicalAvg, plotHeight);
  assert.ok(typicalDomain.yMin >= 8 && typicalDomain.yMin <= 12, `yMin=${typicalDomain.yMin}`);
  assert.ok(typicalDomain.yMax >= 20 && typicalDomain.yMax <= 28, `yMax=${typicalDomain.yMax}`);
  assert.notEqual(typicalDomain.yMax, 100);
  const typicalTicks = buildYTicks(typicalDomain.yMin, typicalDomain.yMax, plotHeight);
  assert.ok(typicalTicks.length >= 4 && typicalTicks.length <= 6, `ticks=${typicalTicks}`);
  assertNoOverlappingYLabels(typicalTicks, typicalDomain.yMin, typicalDomain.yMax, plotHeight);

  // getDRRChartDomain is yMax only — should not force 100.
  assert.ok(getDRRChartDomain(typicalSeries, typicalAvg, plotHeight) < 40);

  // Above 100% expands cleanly.
  const overDomain = computeDRRChartYDomain([118, 130], 125, plotHeight);
  assert.ok(overDomain.yMax >= 135, `over yMax=${overDomain.yMax}`);
  assertNoOverlappingYLabels(
    buildYTicks(overDomain.yMin, overDomain.yMax, plotHeight),
    overDomain.yMin,
    overDomain.yMax,
    plotHeight,
  );

  // Flat series: visible centered band.
  const flatDomain = computeDRRChartYDomain([19.4, 19.4, 19.4], 19.4, plotHeight);
  assert.ok(flatDomain.yMin < 19.4 && flatDomain.yMax > 19.4, `flat=${JSON.stringify(flatDomain)}`);
  assert.ok(flatDomain.yMax - flatDomain.yMin >= 8);

  // Average outside raw min/max must still fit.
  const avgOutside = computeDRRChartYDomain([15, 16, 17], 25, plotHeight);
  assert.ok(avgOutside.yMax >= 25, `avgOutside=${JSON.stringify(avgOutside)}`);

  // Null/missing excluded — domain from valid points only.
  const withNulls = computeDRRChartYDomain([12.2, NaN, 20.1], null, plotHeight);
  assert.ok(withNulls.yMin <= 12.2 && withNulls.yMax >= 20.1);

  assert.ok(chooseNiceStep(12, 5) > 0);

  const first = getPointLabelPlacement({
    x: 58,
    y: 180,
    index: 0,
    pointsCount: 7,
    averageY: 120,
    chartTop: 36,
    chartRight: 672,
    chartLeft: 58,
    plotBottom: 208,
    isCurrent: false,
  });
  assert.equal(first.textAnchor, "start");
  assert.ok(first.labelX > 58);
  assert.ok(first.labelY < 180);

  const nearAverage = getPointLabelPlacement({
    x: 560,
    y: 122,
    index: 5,
    pointsCount: 7,
    averageY: 120,
    chartTop: 36,
    chartRight: 672,
    chartLeft: 58,
    plotBottom: 208,
    isCurrent: false,
  });
  assert.equal(nearAverage.placeBelow, true);
  assert.ok(nearAverage.labelY > 122);

  const lastPoint = getPointLabelPlacement({
    x: 650,
    y: 100,
    index: 6,
    pointsCount: 7,
    averageY: 120,
    chartTop: 36,
    chartRight: 672,
    chartLeft: 58,
    plotBottom: 208,
    isCurrent: true,
  });
  assert.equal(lastPoint.textAnchor, "middle");
  assert.equal(lastPoint.placeBelow, false);
  assert.equal(lastPoint.labelX, 650);
  assert.ok(lastPoint.labelY < 100);
  assert.ok(100 - lastPoint.labelY >= 10 && 100 - lastPoint.labelY <= 16);

  const averageLabel = getAverageLabelPlacement({
    averageY: 120,
    chartRight: 632,
    chartTop: 36,
    chartWidth: 760,
    containerWidth: 1280,
    lastPointX: 620,
    lastPointLabelY: 104,
  });
  assert.equal(averageLabel.textAnchor, "start");
  assert.equal(averageLabel.legendZone, false);
  assert.ok(averageLabel.x > 632, "weekly label must sit outside plot area");
  assert.ok(Math.abs(averageLabel.y - 124) <= 8, "weekly label aligns with dashed line");

  // When current value equals the weekly value, labels must not share the same x band.
  const lastAtAvg = getPointLabelPlacement({
    x: 620,
    y: 120,
    index: 6,
    pointsCount: 7,
    averageY: 120,
    chartTop: 36,
    chartRight: 632,
    chartLeft: 58,
    plotBottom: 208,
    isCurrent: true,
  });
  const weeklyAtSame = getAverageLabelPlacement({
    averageY: 120,
    chartRight: 632,
    chartTop: 36,
    chartWidth: 760,
    containerWidth: 1280,
    lastPointX: 620,
    lastPointLabelY: lastAtAvg.labelY,
  });
  assert.ok(weeklyAtSame.x > lastAtAvg.labelX + 20);
  assert.ok(
    Math.abs(weeklyAtSame.y - lastAtAvg.labelY) >= 12 || weeklyAtSame.x - lastAtAvg.labelX >= 24,
    "weekly and current labels must not overlap",
  );

  // Geometric bounding-box check: current value above last point vs weekly gutter label.
  const plotRight = 632;
  const currentBox = {
    left: lastAtAvg.labelX - 28,
    right: lastAtAvg.labelX + 28,
    top: lastAtAvg.labelY - 10,
    bottom: lastAtAvg.labelY + 10,
  };
  const weeklyBox = {
    left: weeklyAtSame.x,
    right: weeklyAtSame.x + 110,
    top: weeklyAtSame.y - 18,
    bottom: weeklyAtSame.y + 18,
  };
  assert.equal(lastAtAvg.textAnchor, "middle");
  assert.equal(lastAtAvg.labelX, 620, "current label centered on last point x");
  assert.ok(weeklyBox.left >= plotRight, "weekly label starts outside plot area");
  const overlapX = currentBox.left < weeklyBox.right && currentBox.right > weeklyBox.left;
  const overlapY = currentBox.top < weeklyBox.bottom && currentBox.bottom > weeklyBox.top;
  assert.ok(!(overlapX && overlapY), "current and weekly label bounding boxes must not overlap");

  const narrowLegend = getAverageLabelPlacement({
    averageY: 120,
    chartRight: 632,
    chartTop: 36,
    chartWidth: 760,
    containerWidth: 640,
  });
  assert.equal(narrowLegend.legendZone, true);
  assert.equal(narrowLegend.textAnchor, "end");

  assert.equal(
    shouldShowPointLabel({ index: 6, pointsCount: 7, width: 360, isMaximum: false, isCurrent: true }),
    true,
  );

  const deduped = dedupeYTicksByRoundedLabel([10, 10.1, 15, 20], 10, 20, plotHeight);
  assert.ok(deduped.length < 4);

  console.log("ad-efficiency-chart-layout.test.ts: ok");
}

run();
