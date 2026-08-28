import { api } from "@/lib/container";
import { RequestRow } from "@/components/request-row";
import { getTranslations } from "next-intl/server";

export default async function RequestsPage() {
  const [requests, t] = await Promise.all([api.getRequests("pending"), getTranslations("requests")]);

  return (
    <div className="flex flex-col gap-6 p-5 md:mx-auto md:max-w-2xl md:p-8">
      <div>
        <h1 className="text-xl font-bold tracking-tight md:text-[20px]">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>

      {requests.length === 0 ? (
        <div className="flex flex-col items-center gap-3 rounded-xl border border-dashed border-border py-16 text-center">
          <p className="text-sm text-muted-foreground">{t("empty")}</p>
        </div>
      ) : (
        <div className="flex flex-col gap-3">
          {requests.map((r) => (
            <RequestRow key={r.id} request={r} />
          ))}
        </div>
      )}
    </div>
  );
}
