import type { NextConfig } from "next";

const backendProxyUrl = process.env.BACKEND_PROXY_URL || "http://localhost:8081";

const nextConfig: NextConfig = {
  async rewrites() {
    return [
      {
        source: "/api/v1/:path*",
        destination: `${backendProxyUrl}/api/v1/:path*`,
      },
      {
        source: "/health/:path*",
        destination: `${backendProxyUrl}/health/:path*`,
      },
    ];
  },
  async headers() {
    return [
      {
        source: "/review/:path*",
        headers: [
          { key: "Cache-Control", value: "no-store" },
          { key: "X-Robots-Tag", value: "noindex, nofollow, noarchive" },
          { key: "Referrer-Policy", value: "no-referrer" },
        ],
      },
      {
        source: "/live-review-index",
        headers: [
          { key: "Cache-Control", value: "no-store, private, max-age=0" },
          { key: "X-Robots-Tag", value: "noindex, nofollow, noarchive, nosnippet" },
          { key: "Referrer-Policy", value: "no-referrer" },
        ],
      },
    ];
  },
};

export default nextConfig;
