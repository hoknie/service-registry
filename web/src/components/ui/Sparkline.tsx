import type { Point } from "@/lib/metrics";
import { cn } from "@/lib/cn";

export function Sparkline({ points, className }: { points: Point[]; className?: string }) {
  if (points.length < 2) return null;
  const t0 = points[0]!.t;
  const t1 = points[points.length - 1]!.t;
  let lo = Infinity;
  let hi = -Infinity;
  for (const p of points) {
    lo = Math.min(lo, p.v);
    hi = Math.max(hi, p.v);
  }
  const w = 100;
  const h = 24;
  const span = hi - lo || 1;
  const coords = points
    .map((p) => `${(((p.t - t0) / (t1 - t0 || 1)) * w).toFixed(2)},${(h - 1 - ((p.v - lo) / span) * (h - 2)).toFixed(2)}`)
    .join(" ");
  return (
    <svg aria-hidden="true" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" className={cn("h-6 w-full text-signal", className)}>
      <polyline points={coords} fill="none" stroke="currentColor" strokeWidth="1.5" vectorEffect="non-scaling-stroke" strokeLinejoin="round" />
    </svg>
  );
}
