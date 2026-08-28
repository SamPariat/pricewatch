"use client";

import { useTransition } from "react";
import { toast } from "sonner";
import { Switch } from "@/components/ui/switch";
import { toggleEnabledAction } from "@/app/(app)/watches/actions";
import { useTranslations } from "next-intl";
import type { Watch } from "@/lib/types";

export function WatchToggle({ watch }: { watch: Watch }) {
  const t = useTranslations("runNow");
  const [pending, startTransition] = useTransition();

  const onCheckedChange = (checked: boolean) => {
    startTransition(async () => {
      try {
        await toggleEnabledAction(watch.id, {
          name: watch.name,
          kind: watch.kind,
          params: watch.params,
          threshold_pct: watch.threshold_pct,
          trip_id: watch.trip_id,
        }, checked);
      } catch {
        toast.error(t("toggleFailed"));
      }
    });
  };

  return <Switch checked={watch.enabled} disabled={pending} onCheckedChange={onCheckedChange} />;
}
