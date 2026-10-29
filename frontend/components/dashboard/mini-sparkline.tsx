import type { SparklinePoint } from "@/lib/dashboard-home-types";
import { sparklineNumericValue } from "@/lib/sparkline-value";

type MiniSparklineProps = {
  points: SparklinePoint[];
  width?: number;
  height?: number;
  stroke?: string;
  strokeWidth?: number;
  fill?: string;
  showBaseline?: boolean;
  className?: string;
};

export function getValidSparklineValues(points: SparklinePoint[]): number[] {
  return points
    .map((point) => sparklineNumericValue(point.value))
    .filter((value): value is number => value != null);
}

export function canRenderMiniSparkline(points: SparklinePoint[]): boolean {
  let segmentLength = 0;
  let maxSegment = 0;
  for (const point of points) {
    const value = sparklineNumericValue(point.value);
    if (value == null) {
      maxSegment = Math.max(maxSegment, segmentLength);
      segmentLength = 0;
      continue;
    }
    segmentLength += 1;
  }
  maxSegment = Math.max(maxSegment, segmentLength);
  return maxSegment >= 2;
}

export function MiniSparkline({
  points,
  width = 72,
  height = 28,
  stroke = "#2563eb",
  strokeWidth = 1.5,
  fill,
  showBaseline = false,
  className,
}: MiniSparklineProps) {
  const paddingX = 4;
  const paddingY = 4;
  const innerW = width - paddingX * 2;
  const innerH = height - paddingY * 2;
  const step = points.length > 1 ? innerW / (points.length - 1) : 0;

  const definedValues = getValidSparklineValues(points);
  if (!canRenderMiniSparkline(points)) {
    return <svg width={width} height={height} className={className} aria-hidden />;
  }

  const min = Math.min(...definedValues);
  const max = Math.max(...definedValues);
  const range = max - min;
  const flatLine = range === 0;

  const yForValue = (value: number) => {
    if (flatLine) {
      return paddingY + innerH / 2;
    }
    return paddingY + innerH - ((value - min) / range) * innerH;
  };

  type Coord = { x: number; y: number };
  const segments: Coord[][] = [];
  let current: Coord[] = [];

  points.forEach((point, index) => {
    const value = sparklineNumericValue(point.value);
    if (value == null) {
      if (current.length > 0) {
        segments.push(current);
        current = [];
      }
      return;
    }
    const x = paddingX + index * step;
    const y = yForValue(value);
    current.push({ x, y });
  });
  if (current.length > 0) {
    segments.push(current);
  }

  const baselineY = paddingY + innerH;

  return (
    <svg width={width} height={height} className={className} aria-hidden>
      {showBaseline ? (
        <line
          x1={paddingX}
          y1={baselineY}
          x2={width - paddingX}
          y2={baselineY}
          stroke="#E2E8F0"
          strokeWidth={1}
        />
      ) : null}
      {segments.map((segment, index) => {
        if (segment.length < 2) return null;
        const linePoints = segment.map((coord) => `${coord.x},${coord.y}`).join(" ");
        if (!fill) {
          return (
            <polyline
              key={index}
              fill="none"
              stroke={stroke}
              strokeWidth={strokeWidth}
              strokeLinejoin="round"
              strokeLinecap="round"
              points={linePoints}
            />
          );
        }
        const areaPoints = [
          `${segment[0].x},${baselineY}`,
          ...segment.map((coord) => `${coord.x},${coord.y}`),
          `${segment[segment.length - 1].x},${baselineY}`,
        ].join(" ");
        return (
          <g key={index}>
            <polygon points={areaPoints} fill={fill} />
            <polyline
              fill="none"
              stroke={stroke}
              strokeWidth={strokeWidth}
              strokeLinejoin="round"
              strokeLinecap="round"
              points={linePoints}
            />
          </g>
        );
      })}
    </svg>
  );
}
