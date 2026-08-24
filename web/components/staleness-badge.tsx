import { TriangleAlert } from "lucide-react";
import type { Watch } from "@/lib/types";

// The staleness signal from PLAN.md § Freshness: a silently broken watch
// must never look identical to one whose price simply hasn't moved.
// watch.stale is computed server-side (backend/internal/httpapi/
// presenter/v1.IsStale) against 2x the watch's own schedule interval —
// this component only renders what the API already decided.
export function StalenessBadge({ watch }: { watch: Pick<Watch, "stale" | "last_error"> }) {
  if (!watch.stale) return null;
  return (
    <div className="flex items-start gap-2 rounded-md bg-warning/15 px-2.5 py-2">
      <TriangleAlert className="mt-0.5 size-3.5 shrink-0 text-warning" />
      <div className="flex flex-col gap-0.5">
        <span className="text-xs font-semibold text-warning">Needs attention</span>
        {watch.last_error && <span className="text-[11px] text-muted-foreground">{watch.last_error}</span>}
      </div>
    </div>
  );
}
