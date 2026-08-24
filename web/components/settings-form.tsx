"use client";

import { useActionState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { updateSettingsAction } from "@/app/(app)/settings/actions";
import type { Settings } from "@/lib/types";

export function SettingsForm({ settings }: { settings: Settings }) {
  const [state, formAction, pending] = useActionState(updateSettingsAction, { error: null });

  return (
    <form action={formAction} className="flex flex-col gap-6">
      <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5 shadow-sm">
        <h2 className="text-sm font-semibold">Telegram</h2>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="telegram_chat_id">Chat ID</Label>
          <Input id="telegram_chat_id" name="telegram_chat_id" defaultValue={settings.telegram_chat_id} placeholder="-100482910" />
          <p className="text-[11.5px] text-muted-foreground">
            Add the bot to your group, then find its chat ID via the Bot API&apos;s getUpdates endpoint.
          </p>
        </div>
      </section>

      <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5 shadow-sm">
        <h2 className="text-sm font-semibold">Notifications</h2>
        <div className="grid grid-cols-2 gap-4">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="quiet_hours_start">Quiet hours start</Label>
            <Input id="quiet_hours_start" name="quiet_hours_start" type="time" defaultValue={settings.quiet_hours_start} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="quiet_hours_end">Quiet hours end</Label>
            <Input id="quiet_hours_end" name="quiet_hours_end" type="time" defaultValue={settings.quiet_hours_end} />
          </div>
        </div>
      </section>

      <section className="flex flex-col gap-4 rounded-xl border border-border bg-card p-5 shadow-sm">
        <h2 className="text-sm font-semibold">Data</h2>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="currency">Default currency</Label>
          <Input id="currency" name="currency" defaultValue={settings.currency} className="w-24 font-mono uppercase" maxLength={3} />
        </div>
        <div className="flex items-center justify-between">
          <div>
            <div className="text-[13px] font-medium">Dry-run mode</div>
            <p className="text-[11.5px] text-muted-foreground">Runs fetch and render but never sends to Telegram.</p>
          </div>
          <Switch name="dry_run" defaultChecked={settings.dry_run} />
        </div>
      </section>

      {state.error && <p className="text-sm text-destructive">{state.error}</p>}

      <Button type="submit" disabled={pending} className="self-end">
        {pending ? "Saving…" : "Save changes"}
      </Button>
    </form>
  );
}
