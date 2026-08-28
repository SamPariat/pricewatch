"use client";

import { useActionState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { Trip } from "@/lib/types";
import { createTripAction, updateTripAction, type FormState } from "@/app/(app)/trips/actions";
import { useTranslations } from "next-intl";

export function TripForm({ trip }: { trip?: Trip }) {
  const t = useTranslations("tripForm");
  const [hour, minute] = trip ? cronToTime(trip.cron_expr) : ["7", "0"];

  const action = trip ? updateTripAction.bind(null, trip.id) : createTripAction;
  const initialState: FormState = { error: null };
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="flex flex-col gap-6">
      <Field label={t("nameLabel")}>
        <Input name="name" defaultValue={trip?.name} placeholder={t("namePlaceholder")} required />
      </Field>

      <div className="grid grid-cols-2 gap-4">
        <Field label={t("hourLabel")}>
          <Input type="number" name="hour" min={0} max={23} defaultValue={hour} required />
        </Field>
        <Field label={t("minuteLabel")}>
          <Input type="number" name="minute" min={0} max={59} defaultValue={minute} required />
        </Field>
        <Field label={t("timezoneLabel")} className="col-span-2">
          <Input name="timezone" defaultValue={trip?.timezone ?? "Asia/Kolkata"} required />
        </Field>
      </div>

      {state.error && <p className="text-sm text-destructive">{state.error}</p>}

      <Button type="submit" disabled={pending} size="lg">
        {pending ? t("saving") : trip ? t("saveChanges") : t("saveTrip")}
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
