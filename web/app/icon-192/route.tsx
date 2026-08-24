import { ImageResponse } from "next/og";

// A dedicated route (rather than a second `icon.tsx`) because Next.js's
// icon file convention only generates one size per file — manifest.ts
// needs 192 and 512px variants for Android's home-screen requirements.
export async function GET() {
  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          background: "#1a1a1a",
        }}
      >
        <svg width="104" height="104" viewBox="0 0 24 24" fill="none" stroke="#fafafa" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M22 2 11 13" />
          <path d="M22 2 15 22 11 13 2 9 22 2Z" />
        </svg>
      </div>
    ),
    { width: 192, height: 192 },
  );
}
