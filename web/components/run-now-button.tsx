"use client";

import { useState, useTransition } from "react";
import { RefreshCw } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { runWatchNowAction } from "@/app/(app)/watches/actions";

export function RunNowButton({ watchId }: { watchId: string }) {
  const [pending, startTransition] = useTransition();
  const [justRan, setJustRan] = useState(false);

  const onClick = () => {
    startTransition(async () => {
      try {
        await runWatchNowAction(watchId);
        setJustRan(true);
        toast.success("Run complete", { description: "Price and chart refreshed." });
        setTimeout(() => setJustRan(false), 2000);
      } catch {
        toast.error("Run failed", { description: "Check the run log for details." });
      }
    });
  };

  return (
    <Button variant="outline" onClick={onClick} disabled={pending}>
      <RefreshCw className={pending ? "size-4 animate-spin" : "size-4"} />
      {pending ? "Running…" : justRan ? "Done" : "Run now"}
    </Button>
  );
}
