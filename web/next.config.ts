import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Slim, self-contained production output for the Docker image — see
  // Dockerfile, which copies only .next/standalone rather than the full
  // node_modules tree.
  output: "standalone",
};

export default nextConfig;
