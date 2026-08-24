import Link from "next/link";
import { ArrowLeft, Pencil } from "lucide-react";
import { api } from "@/lib/api";
import { formatPrice, watchSubtitle, watchTitle } from "@/lib/format";
import { allTimeLow, rollingLow, rollingMedian } from "@/lib/stats";
import { Button } from "@/components/ui/button";
import { DeltaBadge } from "@/components/delta-badge";
import { RelativeTime } from "@/components/relative-time";
import { StalenessBadge } from "@/components/staleness-badge";
import { PriceChart } from "@/components/price-chart";
import { RunNowButton } from "@/components/run-now-button";
import { WatchToggle } from "@/components/watch-toggle";
import { DeleteWatchButton } from "@/components/delete-watch-button";

export default async function WatchDetailPage(props: PageProps<"/watches/[id]">) {
  const { id } = await props.params;
  const [watch, history] = await Promise.all([api.getWatch(id), api.getHistory(id, "90d")]);

  const currency = watch.price?.currency ?? "INR";
  const low7d = rollingLow(history.samples, 7);
  const median30d = rollingMedian(history.samples, 30);
  const atl = allTimeLow(history.samples);

  return (
    <div className="flex flex-col gap-6 p-5 md:mx-auto md:max-w-3xl md:p-8">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Link href="/watches" className="text-muted-foreground">
            <ArrowLeft className="size-4.5" />
          </Link>
          <div>
            <h1 className="text-lg font-bold tracking-tight md:text-[19px]">{watchTitle(watch)}</h1>
            <p className="text-[12.5px] text-muted-foreground">{watchSubtitle(watch)}</p>
          </div>
        </div>
        <div className="hidden items-center gap-2 md:flex">
          <RunNowButton watchId={watch.id} />
          <Button variant="outline" asChild>
            <Link href={`/watches/${watch.id}/edit`}>
              <Pencil className="size-3.5" />
              Edit
            </Link>
          </Button>
        </div>
      </div>

      <div className="flex items-center gap-2 rounded-md bg-muted px-3 py-2 text-[12.5px]">
        <span>
          Updated <RelativeTime iso={watch.last_updated_at} /> · next check <RelativeTime iso={watch.next_run} />
        </span>
      </div>
      <p className="-mt-4 text-[11px] text-muted-foreground">Prices are indicative, not bookable — confirm on the provider before booking.</p>

      <StalenessBadge watch={watch} />

      {watch.price && (
        <div className="flex items-end justify-between">
          <div>
            <div className="text-[13px] text-muted-foreground">Cheapest right now</div>
            <div className="font-mono text-3xl font-semibold tracking-tight">
              {formatPrice(watch.price.price_minor, watch.price.currency)}
            </div>
          </div>
          <DeltaBadge pct={watch.price.delta_pct} className="mb-1 px-2.5 py-1.5 text-[12.5px]" />
        </div>
      )}

      {watch.price?.percentile !== undefined && (
        <div className="rounded-lg bg-success/10 px-4 py-3.5">
          <p className="text-[13.5px] font-semibold text-success">
            Cheaper than {watch.price.percentile.toFixed(0)}% of the last 90 days
          </p>
        </div>
      )}

      <div className="rounded-xl border border-border bg-card p-5 shadow-sm">
        <PriceChart samples={history.samples} currency={currency} />
      </div>

      <div className="grid grid-cols-3 gap-3">
        <StatTile label="7D low" value={low7d !== undefined ? formatPrice(low7d, currency) : "—"} />
        <StatTile label="30D median" value={median30d !== undefined ? formatPrice(median30d, currency) : "—"} />
        <StatTile label="All-time low" value={atl ? formatPrice(atl.minor, currency) : "—"} tone="warning" />
      </div>

      <div className="flex items-center justify-between rounded-xl border border-border bg-card p-4">
        <div>
          <div className="text-[13.5px] font-medium">Watch enabled</div>
          <div className="text-[12px] text-muted-foreground">Daily at {formatCronTime(watch.cron_expr)} ({watch.timezone})</div>
        </div>
        <WatchToggle watch={watch} />
      </div>

      {/* No "view deal" link here yet — the deep link lives on individual
          Quote rows, not the aggregated PriceSummary this page reads, so
          there's no single canonical URL to point to without threading
          deep_link through the DTOs. A dead "#" link would look
          actionable and do nothing, which is worse than omitting it. */}
      <DeleteWatchButton watchId={watch.id} watchName={watch.name} />

      {/* Mobile action bar */}
      <div className="fixed inset-x-0 bottom-16 flex gap-2 border-t border-border bg-card p-3 md:hidden">
        <div className="flex-1">
          <RunNowButton watchId={watch.id} />
        </div>
        <Button variant="outline" asChild>
          <Link href={`/watches/${watch.id}/edit`}>
            <Pencil className="size-3.5" />
          </Link>
        </Button>
      </div>
    </div>
  );
}

function StatTile({ label, value, tone }: { label: string; value: string; tone?: "warning" }) {
  return (
    <div className="rounded-lg border border-border bg-card p-3.5">
      <div className="text-[11.5px] text-muted-foreground">{label}</div>
      <div className={`mt-0.5 font-mono text-[15px] font-semibold ${tone === "warning" ? "text-warning" : ""}`}>{value}</div>
    </div>
  );
}

function formatCronTime(cronExpr: string): string {
  const [minute, hour] = cronExpr.trim().split(/\s+/);
  const h = Number(hour);
  const m = Number(minute);
  if (Number.isNaN(h) || Number.isNaN(m)) return cronExpr;
  const period = h >= 12 ? "PM" : "AM";
  const h12 = h % 12 === 0 ? 12 : h % 12;
  return `${h12}:${String(m).padStart(2, "0")} ${period}`;
}
