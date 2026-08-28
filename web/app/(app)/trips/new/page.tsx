import { TripForm } from "@/components/trip-form";
import { getTranslations } from "next-intl/server";

export default async function NewTripPage() {
  const t = await getTranslations("newTrip");
  return (
    <div className="mx-auto flex max-w-lg flex-col gap-6 p-5 md:p-8">
      <div>
        <h1 className="text-xl font-bold tracking-tight">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>
      <TripForm />
    </div>
  );
}
