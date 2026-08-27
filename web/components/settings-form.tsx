"use client";

import { useActionState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { updateSettingsAction } from "@/app/(app)/settings/actions";
import { useTranslations } from "next-intl";
import type { Locale } from "@/i18n/request";
import type { Settings } from "@/lib/types";

export function SettingsForm({ settings }: { settings: Settings }) {
  const t = useTranslations("settings");
  const [state, formAction, pending] = useActionState(updateSettingsAction, { error: null });

  return (
    <form action={formAction} className="flex flex-col gap-6">
      <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5 shadow-sm">
        <h2 className="text-sm font-semibold">{t("telegramSection")}</h2>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="telegram_chat_id">{t("chatIdLabel")}</Label>
          <Input id="telegram_chat_id" name="telegram_chat_id" defaultValue={settings.telegram_chat_id} placeholder="-100482910" />
          <p className="text-[11.5px] text-muted-foreground">{t("chatIdHelp")}</p>
        </div>
      </section>

      <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5 shadow-sm">
        <h2 className="text-sm font-semibold">{t("notificationsSection")}</h2>
        <div className="grid grid-cols-2 gap-4">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="quiet_hours_start">{t("quietStartLabel")}</Label>
            <Input id="quiet_hours_start" name="quiet_hours_start" type="time" defaultValue={settings.quiet_hours_start} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="quiet_hours_end">{t("quietEndLabel")}</Label>
            <Input id="quiet_hours_end" name="quiet_hours_end" type="time" defaultValue={settings.quiet_hours_end} />
          </div>
        </div>
      </section>

      <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5 shadow-sm">
        <h2 className="text-sm font-semibold">{t("dataSection")}</h2>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="currency">{t("currencyLabel")}</Label>
          <Input id="currency" name="currency" defaultValue={settings.currency} className="w-24 font-mono uppercase" maxLength={3} />
        </div>
        <div className="flex items-center justify-between">
          <div>
            <div className="text-[13px] font-medium">{t("dryRunLabel")}</div>
            <p className="text-[11.5px] text-muted-foreground">{t("dryRunHelp")}</p>
          </div>
          <Switch name="dry_run" defaultChecked={settings.dry_run} />
        </div>
      </section>

      <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5 shadow-sm">
        <h2 className="text-sm font-semibold">{t("languageSection")}</h2>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="language">{t("languageLabel")}</Label>
          <select
            id="language"
            name="language"
            defaultValue={(settings.language as Locale) || "en"}
            className="h-9 rounded-md border border-input bg-transparent px-3 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
          >
            <option value="en">{t("languageEnglish")}</option>
            <option value="hi">{t("languageHindi")}</option>
          </select>
          <p className="text-[11.5px] text-muted-foreground">{t("languageHelp")}</p>
        </div>
      </section>

      {state.error && <p className="text-sm text-destructive">{state.error}</p>}

      <Button type="submit" disabled={pending} className="self-end">
        {pending ? t("saving") : t("saveChanges")}
      </Button>
    </form>
  );
}
