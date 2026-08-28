import Link from "next/link";
import { ArrowLeft, Plus } from "lucide-react";
import { api } from "@/lib/container";
import { formatPrice, watchSubtitle, watchTitle } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { DeltaBadge } from "@/components/delta-badge";
import { StalenessBadge } from "@/components/staleness-badge";
import { KindIcon } from "@/components/kind-icon";
import { DeleteTripButton } from "@/components/delete-trip-button";
import { getLocale, getTranslations } from "next-intl/server";

export default async function TripDetailPage(props: PageProps<"/trips/[id]">) {
  const { id } = await props.params;
  const [trip, locale, t, tForm] = await Promise.all([
    api.getTrip(id),
    getLocale(),
    getTranslations("trips"),
    getTranslations("watchForm"),
  ]);
  const legs = trip.legs ?? [];

  return (
    <div className="flex flex-col gap-6 p-5 md:mx-auto md:max-w-2xl md:p-8">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Link href="/trips" className="text-muted-foreground">
            <ArrowLeft className="size-4.5" />
          </Link>
          <div>
            <h1 className="text-lg font-bold tracking-tight md:text-[19px]">{trip.name}</h1>
            <p className="text-[12.5px] text-muted-foreground">{t("scheduleLabel", { Time: formatCronTime(trip.cron_expr), Tz: trip.timezone })}</p>
          </div>
        </div>
        <Button asChild size="sm">
          <Link href={`/trips/${trip.id}/new-leg`}>
            <Plus className="size-3.5" />
            {t("addLeg")}
          </Link>
        </Button>
      </div>

      {legs.length === 0 ? (
        <div className="flex flex-col items-center gap-3 rounded-xl border border-dashed border-border py-16 text-center">
          <p className="text-sm text-muted-foreground">{t("noLegsYet")}</p>
          <Button asChild>
            <Link href={`/trips/${trip.id}/new-leg`}>
              <Plus className="size-4" />
              {t("addLeg")}
            </Link>
          </Button>
        </div>
      ) : (
        <div className="flex flex-col gap-3">
          {legs.map((leg) => (
            <Link
              key={leg.id}
              href={`/watches/${leg.id}`}
              className="flex items-center justify-between gap-3 rounded-xl border border-border bg-card p-4 shadow-sm"
            >
              <div className="flex items-center gap-3">
                <KindIcon kind={leg.kind} />
                <div className="flex flex-col gap-0.5">
                  <span className="text-[14px] font-semibold">{watchTitle(leg, tForm)}</span>
                  <span className="text-[12px] text-muted-foreground">{watchSubtitle(leg, tForm, locale)}</span>
                </div>
              </div>
              <div className="flex flex-col items-end gap-1">
                {leg.price && (
                  <>
                    <span className="font-mono text-[15px] font-semibold">
                      {formatPrice(leg.price.price_minor, leg.price.currency)}
                    </span>
                    <DeltaBadge pct={leg.price.delta_pct} />
                  </>
                )}
                {leg.stale && <StalenessBadge watch={leg} />}
              </div>
            </Link>
          ))}
        </div>
      )}

      <DeleteTripButton tripId={trip.id} tripName={trip.name} />
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
