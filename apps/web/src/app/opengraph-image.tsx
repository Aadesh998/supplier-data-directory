import { ImageResponse } from "next/og";
import { SITE_DOMAIN, SITE_NAME, SITE_TAGLINE } from "@/lib/constants";

export const alt = `${SITE_NAME}: ${SITE_TAGLINE}`;
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default function OpengraphImage() {
  return new ImageResponse(
    (
      <div style={{ width: "100%", height: "100%", display: "flex", flexDirection: "column", justifyContent: "space-between",
        background: "#fafafa", color: "#09090b", padding: "72px", fontFamily: "sans-serif" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 20 }}>
          <div style={{ width: 72, height: 72, borderRadius: 16, background: "#1d4ed8", color: "#eff6ff", display: "flex",
            alignItems: "center", justifyContent: "center", fontSize: 44, fontWeight: 700 }}>P</div>
          <div style={{ fontSize: 44, fontWeight: 700 }}>{SITE_NAME}</div>
        </div>
        <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
          <div style={{ display: "flex", flexDirection: "column", fontSize: 68, fontWeight: 800, lineHeight: 1.05, maxWidth: 1000 }}>
            <span>Manufacturers &amp; suppliers across India,</span>
            <span style={{ color: "#1d4ed8" }}>reviewed by people.</span>
          </div>
          <div style={{ fontSize: 30, color: "#3f3f46" }}>Search by product, industry cluster and state.</div>
        </div>
        <div style={{ display: "flex", justifyContent: "space-between", fontSize: 26, color: "#52525b", borderTop: "2px solid #e4e4e7", paddingTop: 24 }}>
          <span>{SITE_DOMAIN}</span>
          <span>Manufacturers · Contract manufacturing · Job work · Exporters</span>
        </div>
      </div>
    ),
    size,
  );
}
