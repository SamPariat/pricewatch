import type { NextConfig } from "next";
import path from "node:path";
import createNextIntlPlugin from "next-intl/plugin";

const withNextIntl = createNextIntlPlugin("./i18n/request.ts");

const nextConfig: NextConfig = {
  // Slim, self-contained production output for the Docker image — see
  // Dockerfile, which copies only .next/standalone rather than the full
  // node_modules tree.
  output: "standalone",
  // This app isn't the git root — locales/ (read by i18n/request.ts,
  // shared with the Go backend) lives one level up at the monorepo
  // root. Without this, Next's file tracing for the standalone output
  // can misdetect the workspace root and fail to bundle that import.
  outputFileTracingRoot: path.join(__dirname, ".."),
};

export default withNextIntl(nextConfig);
