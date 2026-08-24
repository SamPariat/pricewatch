import { ImageResponse } from "next/og";

export const size = { width: 32, height: 32 };
export const contentType = "image/png";

// Generated via next/og's ImageResponse (Satori) — no external design
// tool needed, and it can't read CSS custom properties from globals.css
// (Satori renders in an isolated context), so the mark color is
// hardcoded to match --primary's light-mode value rather than referenced.
export default function Icon() {
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
          borderRadius: 7,
        }}
      >
        <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="#fafafa" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M22 2 11 13" />
          <path d="M22 2 15 22 11 13 2 9 22 2Z" />
        </svg>
      </div>
    ),
    { ...size },
  );
}
