import { api } from "@/lib/container";
import { TripForm } from "@/components/trip-form";
import { getTranslations } from "next-intl/server";

export default async function EditTripPage(props: PageProps<"/trips/[id]/edit">) {
  const { id } = await props.params;
  const [trip, t] = await Promise.all([api.getTrip(id), getTranslations("editTrip")]);

  return (
    <div className="mx-auto flex max-w-lg flex-col gap-6 p-5 md:p-8">
      <div>
        <h1 className="text-xl font-bold tracking-tight">{t("title")}</h1>
      </div>
      <TripForm trip={trip} />
    </div>
  );
}
