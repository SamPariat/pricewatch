import { ArrowDown, ArrowUp } from "lucide-react";
import { cn } from "@/lib/utils";

export function DeltaBadge({ pct, className }: { pct?: number; className?: string }) {
  if (pct === undefined) return null;
  const down = pct < 0;
  return (
    <span
      className={cn(
        "inline-flex items-center gap-0.5 rounded-md px-1.5 py-0.5 font-mono text-[11.5px] font-semibold",
        down ? "bg-success/15 text-success" : "bg-destructive/15 text-destructive",
        className,
      )}
    >
      {down ? <ArrowDown className="size-2.5" /> : <ArrowUp className="size-2.5" />}
      {Math.abs(pct).toFixed(1)}%
    </span>
  );
}
