import type { NextConfig } from "next";

// A static export (`web/out`) that the Go binary serves from disk on the same origin as the
// API (ADR-0022): no Node at runtime. The UI never calls the API by absolute URL, only relative
// `/api/...`; locale redirects and the `NEXT_LOCALE` cookie are done by the Go server
// (`internal/application/web/`), so no middleware/proxy here (ARCHITECTURE.md §9).
const config: NextConfig = {
  output: "export",
  // `ru/catalog.html`, addresses without a trailing slash; the server 308s `/ru/catalog/`.
  trailingSlash: false,
  poweredByHeader: false,
  reactStrictMode: true,
  // No image optimization: it needs a server (not available in a static export).
  images: { unoptimized: true },
};

export default config;
