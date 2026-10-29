type SparklineInputPoint = {
  orders_count?: number | null;
};

type ProductSparklineProps = {
  points: SparklineInputPoint[];
  width?: number;
  height?: number;
  className?: string;
};

function definedOrdersCount(p: SparklineInputPoint): number | null {
  if (p.orders_count == null || Number.isNaN(p.orders_count)) {
    return null;
  }
  return Math.max(0, p.orders_count);
}

export function ProductSparkline({
  points,
  width = 120,
  height = 36,
  className = "",
}: ProductSparklineProps) {
  const padding = 2;
  const innerW = width - padding * 2;
  const innerH = height - padding * 2;
  const step = points.length > 1 ? innerW / (points.length - 1) : 0;

  const definedValues = points
    .map(definedOrdersCount)
    .filter((v): v is number => v != null);
  const max = Math.max(...definedValues, 1);

  type Coord = { x: number; y: number };
  const segments: Coord[][] = [];
  let current: Coord[] = [];

  points.forEach((p, i) => {
    const v = definedOrdersCount(p);
    if (v == null) {
      if (current.length > 0) {
        segments.push(current);
        current = [];
      }
      return;
    }
    const x = padding + i * step;
    const y = padding + innerH - (v / max) * innerH;
    current.push({ x, y });
  });
  if (current.length > 0) {
    segments.push(current);
  }

  const hasAnyData = definedValues.length > 0;
  const allZero = hasAnyData && definedValues.every((v) => v === 0);

  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      className={className}
      aria-hidden
    >
      {!hasAnyData || allZero ? (
        <line
          x1={padding}
          y1={height / 2}
          x2={width - padding}
          y2={height / 2}
          stroke="currentColor"
          strokeWidth={1}
          className="text-aivion-border"
        />
      ) : (
        segments.map((seg, idx) => (
          <polyline
            key={idx}
            fill="none"
            stroke="currentColor"
            strokeWidth={1.5}
            strokeLinejoin="round"
            strokeLinecap="round"
            points={seg.map((c) => `${c.x},${c.y}`).join(" ")}
            className="text-aivion-blue"
          />
        ))
      )}
    </svg>
  );
}
