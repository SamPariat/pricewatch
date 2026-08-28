import { WatchForm } from "@/components/watch-form";
import { api } from "@/lib/container";
import { getTranslations } from "next-intl/server";

export default async function NewLegPage(props: PageProps<"/trips/[id]/new-leg">) {
  const { id } = await props.params;
  const [trip, t] = await Promise.all([api.getTrip(id), getTranslations("newWatch")]);
  return (
    <div className="mx-auto flex max-w-lg flex-col gap-6 p-5 md:p-8">
      <div>
        <h1 className="text-xl font-bold tracking-tight">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle", { Name: trip.name })}</p>
      </div>
      <WatchForm tripId={trip.id} />
    </div>
  );
}
