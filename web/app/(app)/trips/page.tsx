import Link from "next/link";
import { Plus } from "lucide-react";
import { api } from "@/lib/container";
import { formatPrice, watchSubtitle, watchTitle } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { DeltaBadge } from "@/components/delta-badge";
import { StalenessBadge } from "@/components/staleness-badge";
import { KindIcon } from "@/components/kind-icon";
import { getLocale, getTranslations } from "next-intl/server";
import type { Trip } from "@/lib/types";

type TripsT = Awaited<ReturnType<typeof getTranslations<"trips">>>;
type WatchFormT = Awaited<ReturnType<typeof getTranslations<"watchForm">>>;

export default async function TripsPage() {
  const [trips, locale, t, tForm] = await Promise.all([
    api.getTrips(),
    getLocale(),
    getTranslations("trips"),
    getTranslations("watchForm"),
  ]);

  return (
    <div className="flex flex-col gap-6 p-5 md:p-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold tracking-tight md:text-[20px]">{t("title")}</h1>
          <p className="text-sm text-muted-foreground">
            {trips.length} {t("count")}
          </p>
        </div>
        <Button asChild className="hidden md:inline-flex">
          <Link href="/trips/new">
            <Plus className="size-4" />
            {t("newTrip")}
          </Link>
        </Button>
      </div>

      {trips.length === 0 ? (
        <div className="flex flex-col items-center gap-3 rounded-xl border border-dashed border-border py-16 text-center">
          <p className="text-sm text-muted-foreground">{t("emptyText")}</p>
          <Button asChild>
            <Link href="/trips/new">
              <Plus className="size-4" />
              {t("newTrip")}
            </Link>
          </Button>
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {trips.map((trip) => (
            <TripCard key={trip.id} trip={trip} locale={locale} t={t} tForm={tForm} />
          ))}
        </div>
      )}
    </div>
  );
}

function TripCard({ trip, locale, t, tForm }: { trip: Trip; locale: string; t: TripsT; tForm: WatchFormT }) {
  const legs = trip.legs ?? [];
  return (
    <div className="flex flex-col gap-3 rounded-xl border border-border bg-card p-4.5 shadow-sm">
      <div className="flex items-center justify-between gap-3">
        <div>
          <Link href={`/trips/${trip.id}`} className="text-[15px] font-semibold hover:underline">
            {trip.name}
          </Link>
          <p className="text-[12px] text-muted-foreground">
            {legs.length} {t("legCount")}
            {!trip.enabled && <span className="text-warning"> · {t("paused")}</span>}
          </p>
        </div>
        <Button variant="outline" size="sm" asChild>
          <Link href={`/trips/${trip.id}/new-leg`}>
            <Plus className="size-3.5" />
            {t("addLeg")}
          </Link>
        </Button>
      </div>

      {legs.length === 0 ? (
        <p className="text-[12.5px] text-muted-foreground">{t("noLegsYet")}</p>
      ) : (
        <div className="flex flex-col gap-2">
          {legs.map((leg) => (
            <Link
              key={leg.id}
              href={`/watches/${leg.id}`}
              className="flex items-center justify-between gap-3 rounded-lg border border-border/60 px-3 py-2"
            >
              <div className="flex items-center gap-2.5">
                <KindIcon kind={leg.kind} />
                <div className="flex flex-col">
                  <span className="text-[13px] font-medium">{watchTitle(leg, tForm)}</span>
                  <span className="text-[11.5px] text-muted-foreground">{watchSubtitle(leg, tForm, locale)}</span>
                </div>
              </div>
              <div className="flex flex-col items-end gap-1">
                {leg.price && (
                  <>
                    <span className="font-mono text-[13px] font-semibold">
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
    </div>
  );
}
