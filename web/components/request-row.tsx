"use client";

import { useState, useTransition } from "react";
import { toast } from "sonner";
import { Check, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { approveRequestAction, rejectRequestAction } from "@/app/(app)/requests/actions";
import { useTranslations } from "next-intl";
import type { Request } from "@/lib/types";

export function RequestRow({ request }: { request: Request }) {
  const t = useTranslations("requests");
  const [pending, startTransition] = useTransition();
  const [resolved, setResolved] = useState(false);

  const approve = () =>
    startTransition(async () => {
      const result = await approveRequestAction(request.id);
      if (result.error) toast.error(result.error);
      else setResolved(true);
    });

  const reject = () =>
    startTransition(async () => {
      await rejectRequestAction(request.id);
      setResolved(true);
    });

  if (resolved) return null;

  return (
    <div className="flex flex-col gap-3 rounded-xl border border-border bg-card p-4 shadow-sm sm:flex-row sm:items-center sm:justify-between">
      <div className="flex min-w-0 flex-col gap-1">
        <Badge variant="secondary" className="w-fit text-[11px]">
          {t(`kind_${request.kind}` as "kind_add_leg" | "kind_remove_leg" | "kind_create_trip")}
        </Badge>
        <p className="text-[14.5px] font-semibold">{request.title}</p>
        {request.note && <p className="break-all text-[12px] text-muted-foreground">{request.note}</p>}
      </div>
      <div className="flex shrink-0 gap-2">
        <Button size="sm" variant="outline" disabled={pending} onClick={reject}>
          <X className="size-3.5" />
          {t("reject")}
        </Button>
        <Button size="sm" disabled={pending} onClick={approve}>
          <Check className="size-3.5" />
          {t("approve")}
        </Button>
      </div>
    </div>
  );
}
