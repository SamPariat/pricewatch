import Link from "next/link";
import { Plane, BedDouble, Plus } from "lucide-react";
import { api } from "@/lib/api";
import { formatPrice, watchSubtitle, watchTitle } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { DeltaBadge } from "@/components/delta-badge";
import { RelativeTime } from "@/components/relative-time";
import { StalenessBadge } from "@/components/staleness-badge";
import type { Watch } from "@/lib/types";

function KindIcon({ kind }: { kind: Watch["kind"] }) {
  const Icon = kind === "hotel" || kind === "rental" ? BedDouble : Plane;
  return (
    <div className="flex size-9 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
      <Icon className="size-4" />
    </div>
  );
}

export default async function WatchesPage() {
  const [watches, summary] = await Promise.all([api.getWatches(), api.getAnalyticsSummary()]);

  return (
    <div className="flex flex-col gap-6 p-5 md:p-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold tracking-tight md:text-[20px]">Watches</h1>
          <p className="text-sm text-muted-foreground">
            {summary.enabled_watches} active
            {summary.stale_watches > 0 && (
              <>
                {" "}
                · <span className="font-medium text-warning">{summary.stale_watches} needs attention</span>
              </>
            )}
          </p>
        </div>
        <Button asChild className="hidden md:inline-flex">
          <Link href="/watches/new">
            <Plus className="size-4" />
            New watch
          </Link>
        </Button>
      </div>

      {watches.length === 0 ? (
        <EmptyState />
      ) : (
        <>
          {/* Desktop KPI row */}
          <div className="hidden gap-4 md:grid md:grid-cols-4">
            <Kpi label="Active watches" value={String(summary.enabled_watches)} />
            <Kpi
              label="Avg change, 7d"
              value={summary.avg_delta_7d_pct !== undefined ? `${summary.avg_delta_7d_pct.toFixed(1)}%` : "—"}
              tone={summary.avg_delta_7d_pct !== undefined && summary.avg_delta_7d_pct < 0 ? "success" : undefined}
            />
            <Kpi label="Total watches" value={String(summary.total_watches)} />
            <Kpi label="Needs attention" value={String(summary.stale_watches)} tone={summary.stale_watches > 0 ? "warning" : undefined} />
          </div>

          {/* Mobile: cards */}
          <div className="flex flex-col gap-3 md:hidden">
            {watches.map((w) => (
              <WatchCard key={w.id} watch={w} />
            ))}
          </div>

          {/* Desktop: table */}
          <div className="hidden overflow-hidden rounded-xl border border-border bg-card md:block">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border">
                  <Th>Watch</Th>
                  <Th>Price</Th>
                  <Th>Δ vs yesterday</Th>
                  <Th>Updated</Th>
                  <Th>Status</Th>
                </tr>
              </thead>
              <tbody>
                {watches.map((w) => (
                  <WatchRow key={w.id} watch={w} />
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}

function Kpi({ label, value, tone }: { label: string; value: string; tone?: "success" | "warning" }) {
  return (
    <div className="rounded-xl border border-border bg-card p-4.5 shadow-sm">
      <div className="text-[13px] text-muted-foreground">{label}</div>
      <div
        className={`mt-0.5 font-mono text-2xl font-semibold ${tone === "success" ? "text-success" : tone === "warning" ? "text-warning" : ""}`}
      >
        {value}
      </div>
    </div>
  );
}

function Th({ children }: { children: React.ReactNode }) {
  return <th className="h-10 px-5 text-left text-[12.5px] font-medium text-muted-foreground">{children}</th>;
}

function WatchCard({ watch }: { watch: Watch }) {
  return (
    <Link
      href={`/watches/${watch.id}`}
      className="flex flex-col gap-3 rounded-xl border border-border bg-card p-4 shadow-sm"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <KindIcon kind={watch.kind} />
          <div className="flex flex-col gap-0.5">
            <span className="text-[14.5px] font-semibold">{watchTitle(watch)}</span>
            <span className="text-[12.5px] text-muted-foreground">{watchSubtitle(watch)}</span>
          </div>
        </div>
        {watch.price && (
          <div className="flex flex-col items-end gap-1">
            <span className="font-mono text-[17px] font-semibold">{formatPrice(watch.price.price_minor, watch.price.currency)}</span>
            <DeltaBadge pct={watch.price.delta_pct} />
          </div>
        )}
      </div>
      {watch.stale ? (
        <StalenessBadge watch={watch} />
      ) : (
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
          Updated <RelativeTime iso={watch.last_updated_at} />
        </div>
      )}
    </Link>
  );
}

function WatchRow({ watch }: { watch: Watch }) {
  return (
    <tr className={`border-b border-border last:border-0 ${watch.stale ? "bg-warning/10" : ""}`}>
      <td className="px-5 py-3.5">
        <Link href={`/watches/${watch.id}`} className="flex items-center gap-2.5">
          <KindIcon kind={watch.kind} />
          <div className="flex flex-col">
            <span className="text-[13.5px] font-semibold">{watchTitle(watch)}</span>
            <span className="text-[11.5px] text-muted-foreground">{watchSubtitle(watch)}</span>
          </div>
        </Link>
      </td>
      <td className="px-5 py-3.5 font-mono text-sm font-semibold">
        {watch.price ? formatPrice(watch.price.price_minor, watch.price.currency) : "—"}
      </td>
      <td className="px-5 py-3.5">
        <DeltaBadge pct={watch.price?.delta_pct} />
      </td>
      <td className="px-5 py-3.5 text-[12.5px] text-muted-foreground">
        <RelativeTime iso={watch.last_updated_at} />
      </td>
      <td className="px-5 py-3.5">
        {watch.stale ? (
          <Badge className="bg-warning text-warning-foreground">Needs attention</Badge>
        ) : (
          <Badge variant="secondary" className="bg-success/15 text-success">
            OK
          </Badge>
        )}
      </td>
    </tr>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center gap-3 rounded-xl border border-dashed border-border py-16 text-center">
      <p className="text-sm text-muted-foreground">No watches yet — add one to start tracking prices.</p>
      <Button asChild>
        <Link href="/watches/new">
          <Plus className="size-4" />
          New watch
        </Link>
      </Button>
    </div>
  );
}
