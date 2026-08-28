import { Home, Plane } from "lucide-react";
import type { AssetKind } from "@/lib/types";

export function KindIcon({ kind }: { kind: AssetKind }) {
  const Icon = kind === "lodging_airbnb" ? Home : Plane;
  return (
    <div className="flex size-9 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
      <Icon className="size-4" />
    </div>
  );
}
