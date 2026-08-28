"use client";

import { useActionState, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { AssetKind, FlightParams, LodgingParams, Watch } from "@/lib/types";
import { createWatchAction, updateWatchAction, type FormState } from "@/app/(app)/watches/actions";
import { useTranslations } from "next-intl";

// tripId is required for a new leg (the trip it's being added to);
// ignored for an edit, since watch.trip_id already carries it and legs
// aren't reassignable to a different trip from this form.
export function WatchForm({ watch, tripId }: { watch?: Watch; tripId?: string }) {
  const t = useTranslations("watchForm");
  const kindLabel: Record<AssetKind, string> = {
    flight_one_way: t("kindOneWay"),
    flight_return: t("kindReturn"),
    lodging_airbnb: t("kindAirbnb"),
  };

  const [kind, setKind] = useState<AssetKind>(watch?.kind ?? "flight_return");
  const isReturn = kind === "flight_return";
  const isLodging = kind === "lodging_airbnb";
  const flightParams = watch?.kind !== "lodging_airbnb" ? (watch?.params as FlightParams | undefined) : undefined;
  const lodgingParams = watch?.kind === "lodging_airbnb" ? (watch.params as LodgingParams) : undefined;

  const action = watch ? updateWatchAction.bind(null, watch.id) : createWatchAction;
  const initialState: FormState = { error: null };
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="flex flex-col gap-6">
      <input type="hidden" name="trip_id" value={watch?.trip_id ?? tripId ?? ""} />

      <div className="flex flex-col gap-2">
        <Label>{t("typeLabel")}</Label>
        <Tabs value={kind} onValueChange={(v) => setKind(v as AssetKind)}>
          <TabsList className="grid w-full grid-cols-3">
            {(Object.keys(kindLabel) as AssetKind[]).map((k) => (
              <TabsTrigger key={k} value={k}>
                {kindLabel[k]}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        <input type="hidden" name="kind" value={kind} />
      </div>

      {isLodging ? (
        <div className="grid grid-cols-2 gap-4">
          <Field label={t("urlLabel")} className="col-span-2">
            <Input type="url" name="url" defaultValue={lodgingParams?.url} placeholder="https://www.airbnb.com/rooms/..." required />
          </Field>
          <Field label={t("checkInLabel")}>
            <Input type="date" name="check_in" defaultValue={lodgingParams?.check_in} required />
          </Field>
          <Field label={t("checkOutLabel")}>
            <Input type="date" name="check_out" defaultValue={lodgingParams?.check_out} required />
          </Field>
          <Field label={t("guestsLabel")} className="col-span-2">
            <Input type="number" name="guests" min={1} defaultValue={lodgingParams?.guests} />
          </Field>
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-4">
          <Field label={t("originLabel")}>
            <Input name="origin" defaultValue={flightParams?.origin} placeholder="BLR" required maxLength={3} className="uppercase" />
          </Field>
          <Field label={t("destinationLabel")}>
            <Input name="destination" defaultValue={flightParams?.destination} placeholder="GOI" required maxLength={3} className="uppercase" />
          </Field>
          <Field label={t("departDateLabel")}>
            <Input type="date" name="depart_date" defaultValue={flightParams?.depart_date} required />
          </Field>
          {isReturn && (
            <Field label={t("returnDateLabel")}>
              <Input type="date" name="return_date" defaultValue={flightParams?.return_date} required={isReturn} />
            </Field>
          )}
        </div>
      )}

      <div className="grid grid-cols-2 gap-4">
        <Field label={t("thresholdLabel")} className="col-span-2">
          <Input type="number" name="threshold_pct" min={1} max={90} defaultValue={watch?.threshold_pct ?? 15} required />
        </Field>
        <Field label={t("nameLabel")} className="col-span-2">
          <Input name="name" defaultValue={watch?.name} placeholder={t("namePlaceholder")} />
        </Field>
      </div>

      {state.error && <p className="text-sm text-destructive">{state.error}</p>}

      <Button type="submit" disabled={pending} size="lg">
        {pending ? t("saving") : watch ? t("saveChanges") : t("saveWatch")}
      </Button>
    </form>
  );
}

function Field({ label, className, children }: { label: string; className?: string; children: React.ReactNode }) {
  return (
    <div className={`flex flex-col gap-1.5 ${className ?? ""}`}>
      <Label>{label}</Label>
      {children}
    </div>
  );
}
