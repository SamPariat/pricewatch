"use client";

import { useEffect, useState } from "react";

// Computed client-side from an absolute ISO string, never baked into a
// server-rendered payload — an RSC-computed "6h ago" would be wrong the
// moment the response (or Fiber's cache layer) serves the same bytes
// again later. See PLAN.md's Traps list.
//
// The relative label itself is computed inside an effect, not directly
// during render: Date.now() is an impure call, and calling it in the
// render body would also mean the server-rendered markup and the
// client's first paint compute two different "now" values, producing a
// hydration mismatch. Rendering a stable placeholder until the effect
// fires avoids both problems.
export function RelativeTime({ iso }: { iso?: string }) {
  const [label, setLabel] = useState<string | null>(null);

  useEffect(() => {
    if (!iso) return;
    const date = new Date(iso);
    if (Number.isNaN(date.getTime())) return;
    // A single synchronous setState here is the standard, accepted shape
    // for "client-only value computed once after mount" (the same
    // pattern every relative-time library uses) — it costs one extra
    // render of a small <time> label, not a cascade worth restructuring
    // around.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setLabel(formatRelative(Date.now() - date.getTime()));
  }, [iso]);

  if (!iso) {
    return <span className="text-muted-foreground">Never updated</span>;
  }
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return <span className="text-muted-foreground">Unknown</span>;
  }

  return (
    <time dateTime={iso} title={date.toLocaleString()}>
      {label ?? "…"}
    </time>
  );
}

function formatRelative(ms: number): string {
  const minutes = Math.round(ms / 60_000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.round(hours / 24);
  return `${days}d ago`;
}
