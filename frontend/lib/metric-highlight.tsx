import type { ReactNode } from "react";

export type MetricTextSegment = {
  text: string;
  bold: boolean;
};

type MetricMatch = {
  start: number;
  end: number;
  text: string;
};

const METRIC_PATTERN_SOURCES = [
  String.raw`[+\-−]?\d{1,3}(?:\s\d{3})+(?:[.,]\d+)?\s*₽`,
  String.raw`[+\-−]?\d+(?:[.,]\d+)?\s*₽`,
  String.raw`[+\-−]?\d+(?:[.,]\d+)?\s*п\.п\.`,
  String.raw`[+\-−]?\d+(?:[.,]\d+)?\s*%`,
  String.raw`\d+\s+рекламн(?:ых|ая|ые)\s+кампани(?:й|я|и|ям)`,
  String.raw`\d+\s+кампани(?:й|я|и|ям)`,
  String.raw`\d+(?:[.,]\d+)?\s*%?\s*отмен(?:а|ы|ах|е)?`,
  String.raw`\d+\s+отмен(?:а|ы|ах|е)?`,
  String.raw`\d+\s+заказ(?:ов|а|е|ах|у|ом)?`,
  String.raw`\d+\s+(?:критичн(?:ых|ый|ые)|высок(?:их|ий|ие)|коротк(?:их|ий|ое))`,
  String.raw`\d+\s+риск(?:ов|а|е)?`,
  String.raw`покрытие\s+\d+(?:[.,]\d+)?\s+дн(?:я|ей|ь)`,
  String.raw`\d+(?:[.,]\d+)?\s+дн(?:я|ей|ь)`,
  String.raw`\d+\s+SKU`,
] as const;

const METRIC_PATTERNS = METRIC_PATTERN_SOURCES.map((source) => new RegExp(source, "giu"));

function collectMetricMatches(text: string): MetricMatch[] {
  const matches: MetricMatch[] = [];

  for (const pattern of METRIC_PATTERNS) {
    pattern.lastIndex = 0;
    for (const match of text.matchAll(pattern)) {
      const start = match.index ?? -1;
      if (start < 0) {
        continue;
      }
      const value = match[0];
      matches.push({
        start,
        end: start + value.length,
        text: value,
      });
    }
  }

  matches.sort((left, right) => {
    if (left.start !== right.start) {
      return left.start - right.start;
    }
    return right.end - right.start - (left.end - left.start);
  });

  const selected: MetricMatch[] = [];
  let cursor = 0;
  for (const match of matches) {
    if (match.start < cursor) {
      continue;
    }
    selected.push(match);
    cursor = match.end;
  }

  return selected;
}

export function splitMetricHighlightText(text: string): MetricTextSegment[] {
  const trimmed = (text ?? "").trim();
  if (!trimmed) {
    return [];
  }

  const matches = collectMetricMatches(trimmed);
  if (matches.length === 0) {
    return [{ text: trimmed, bold: false }];
  }

  const segments: MetricTextSegment[] = [];
  let cursor = 0;
  for (const match of matches) {
    if (match.start > cursor) {
      segments.push({ text: trimmed.slice(cursor, match.start), bold: false });
    }
    segments.push({ text: match.text, bold: true });
    cursor = match.end;
  }

  if (cursor < trimmed.length) {
    segments.push({ text: trimmed.slice(cursor), bold: false });
  }

  return segments.filter((segment) => segment.text.length > 0);
}

export function renderMetricHighlightText(text: string, className?: string): ReactNode {
  const segments = splitMetricHighlightText(text);
  return (
    <span className={className}>
      {segments.map((segment, index) =>
        segment.bold ? (
          <strong key={`${segment.text}-${index}`} className="font-semibold text-aivion-text">
            {segment.text}
          </strong>
        ) : (
          <span key={`${segment.text}-${index}`}>{segment.text}</span>
        ),
      )}
    </span>
  );
}
