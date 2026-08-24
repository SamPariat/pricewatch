"use client";

import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart";
import type { PriceSample } from "@/lib/types";

const chartConfig = {
  median_minor: { label: "Median price", color: "var(--success)" },
} satisfies ChartConfig;

export function PriceChart({ samples, currency }: { samples: PriceSample[]; currency: string }) {
  if (samples.length === 0) {
    return (
      <div className="flex h-56 items-center justify-center text-sm text-muted-foreground">
        No price history yet
      </div>
    );
  }

  const data = samples.map((s) => ({ ...s, price: s.median_minor / 100 }));

  return (
    <ChartContainer config={chartConfig} className="aspect-auto h-56 w-full">
      <AreaChart data={data} margin={{ left: 4, right: 4, top: 8, bottom: 0 }}>
        <defs>
          <linearGradient id="priceFill" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--success)" stopOpacity={0.18} />
            <stop offset="100%" stopColor="var(--success)" stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid vertical={false} strokeDasharray="3 4" />
        <XAxis
          dataKey="date"
          tickLine={false}
          axisLine={false}
          tickMargin={8}
          minTickGap={32}
          tickFormatter={(v: string) => new Date(v + "T00:00:00Z").toLocaleDateString("en-US", { month: "short", day: "numeric", timeZone: "UTC" })}
        />
        <YAxis
          tickLine={false}
          axisLine={false}
          tickMargin={8}
          width={56}
          tickFormatter={(v: number) => `${currency} ${Math.round(v).toLocaleString("en-US")}`}
        />
        <ChartTooltip
          content={
            <ChartTooltipContent
              labelFormatter={(v) => new Date(String(v) + "T00:00:00Z").toLocaleDateString("en-US", { month: "short", day: "numeric", timeZone: "UTC" })}
              formatter={(value) => `${currency} ${Number(value).toLocaleString("en-US")}`}
            />
          }
        />
        <Area dataKey="price" type="monotone" stroke="var(--success)" strokeWidth={2} fill="url(#priceFill)" />
      </AreaChart>
    </ChartContainer>
  );
}
