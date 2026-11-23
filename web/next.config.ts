import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // This app lives inside a Go monorepo, so pin the tracing root to web/.
  // Without it Next walks up, finds other lockfiles, and warns about an
  // ambiguous workspace root.
  outputFileTracingRoot: __dirname,
};

export default nextConfig;
