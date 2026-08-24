"use client";

import { useTransition } from "react";
import { toast } from "sonner";
import { Switch } from "@/components/ui/switch";
import { toggleEnabledAction } from "@/app/(app)/watches/actions";
import type { Watch } from "@/lib/types";

export function WatchToggle({ watch }: { watch: Watch }) {
  const [pending, startTransition] = useTransition();

  const onCheckedChange = (checked: boolean) => {
    startTransition(async () => {
      try {
        await toggleEnabledAction(watch.id, {
          name: watch.name,
          kind: watch.kind,
          cron_expr: watch.cron_expr,
          timezone: watch.timezone,
          params: watch.params,
          threshold_pct: watch.threshold_pct,
        }, checked);
      } catch {
        toast.error("Failed to update watch");
      }
    });
  };

  return <Switch checked={watch.enabled} disabled={pending} onCheckedChange={onCheckedChange} />;
}
