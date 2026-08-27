"use client";

import { useState, useTransition } from "react";
import { RefreshCw } from "lucide-react";
import { toast } from "sonner";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { runWatchNowAction } from "@/app/(app)/watches/actions";

export function RunNowButton({ watchId }: { watchId: string }) {
  const t = useTranslations("runNow");
  const [pending, startTransition] = useTransition();
  const [justRan, setJustRan] = useState(false);

  const onClick = () => {
    startTransition(async () => {
      try {
        await runWatchNowAction(watchId);
        setJustRan(true);
        toast.success(t("successTitle"), { description: t("successDesc") });
        setTimeout(() => setJustRan(false), 2000);
      } catch {
        toast.error(t("failTitle"), { description: t("failDesc") });
      }
    });
  };

  return (
    <Button variant="outline" onClick={onClick} disabled={pending}>
      <RefreshCw className={pending ? "size-4 animate-spin" : "size-4"} />
      {pending ? t("running") : justRan ? t("done") : t("idle")}
    </Button>
  );
}
