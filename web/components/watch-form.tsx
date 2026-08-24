"use client";

import { useActionState, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { isFlightParams, type AssetKind, type Watch } from "@/lib/types";
import { createWatchAction, updateWatchAction, type FormState } from "@/app/(app)/watches/actions";

const KIND_LABEL: Record<AssetKind, string> = {
  flight_one_way: "One-way",
  flight_return: "Return",
  hotel: "Hotel",
  rental: "Rental",
};

export function WatchForm({ watch }: { watch?: Watch }) {
  const [kind, setKind] = useState<AssetKind>(watch?.kind ?? "flight_return");
  const isFlight = kind === "flight_one_way" || kind === "flight_return";
  const isReturn = kind === "flight_return";

  const flightParams = watch && isFlightParams(watch.params) ? watch.params : undefined;
  const hotelParams = watch && !isFlightParams(watch.params) ? watch.params : undefined;

  const [hour, minute] = watch ? cronToTime(watch.cron_expr) : ["7", "0"];

  const action = watch ? updateWatchAction.bind(null, watch.id) : createWatchAction;
  const initialState: FormState = { error: null };
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="flex flex-col gap-6">
      <div className="flex flex-col gap-2">
        <Label>Type</Label>
        <Tabs value={kind} onValueChange={(v) => setKind(v as AssetKind)}>
          <TabsList className="grid w-full grid-cols-4">
            {(Object.keys(KIND_LABEL) as AssetKind[]).map((k) => (
              <TabsTrigger key={k} value={k}>
                {KIND_LABEL[k]}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        <input type="hidden" name="kind" value={kind} />
      </div>

      {isFlight ? (
        <div className="grid grid-cols-2 gap-4">
          <Field label="Origin (IATA)">
            <Input name="origin" defaultValue={flightParams?.origin} placeholder="BLR" required maxLength={3} className="uppercase" />
          </Field>
          <Field label="Destination (IATA)">
            <Input name="destination" defaultValue={flightParams?.destination} placeholder="GOI" required maxLength={3} className="uppercase" />
          </Field>
          <Field label="Depart date">
            <Input type="date" name="depart_date" defaultValue={flightParams?.depart_date} required />
          </Field>
          {isReturn && (
            <Field label="Return date">
              <Input type="date" name="return_date" defaultValue={flightParams?.return_date} required={isReturn} />
            </Field>
          )}
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-4">
          <Field label="Location" className="col-span-2">
            <Input name="location" defaultValue={hotelParams?.location} placeholder="Goa" required />
          </Field>
          <Field label="Check-in">
            <Input type="date" name="check_in" defaultValue={hotelParams?.check_in} required />
          </Field>
          <Field label="Check-out">
            <Input type="date" name="check_out" defaultValue={hotelParams?.check_out} required />
          </Field>
        </div>
      )}

      <div className="grid grid-cols-2 gap-4">
        <Field label="Check time (hour)">
          <Input type="number" name="hour" min={0} max={23} defaultValue={hour} required />
        </Field>
        <Field label="Check time (minute)">
          <Input type="number" name="minute" min={0} max={59} defaultValue={minute} required />
        </Field>
        <Field label="Timezone" className="col-span-2">
          <Input name="timezone" defaultValue={watch?.timezone ?? "Asia/Kolkata"} required />
        </Field>
        <Field label="Alert threshold (% drop)" className="col-span-2">
          <Input type="number" name="threshold_pct" min={1} max={90} defaultValue={watch?.threshold_pct ?? 15} required />
        </Field>
        <Field label="Name (optional)" className="col-span-2">
          <Input name="name" defaultValue={watch?.name} placeholder="Goa return in December" />
        </Field>
      </div>

      {state.error && <p className="text-sm text-destructive">{state.error}</p>}

      <Button type="submit" disabled={pending} size="lg">
        {pending ? "Saving…" : watch ? "Save changes" : "Save watch"}
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

function cronToTime(cronExpr: string): [string, string] {
  const parts = cronExpr.trim().split(/\s+/);
  if (parts.length >= 2) return [parts[1], parts[0]];
  return ["7", "0"];
}
