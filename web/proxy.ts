import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const SESSION_COOKIE = "pw_session";
const PUBLIC_PATHS = ["/login"];

// Cheap, presence-only gate: redirects to /login when the session cookie
// is entirely absent, before the route even renders. This does NOT
// verify the cookie's signature or expiry — Fiber is the sole authority
// on that (see lib/api.ts's redirect-on-401), since verifying an HMAC
// here would mean duplicating SESSION_SECRET into the web container,
// which the whole point of this design is to avoid.
export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  if (PUBLIC_PATHS.some((p) => pathname.startsWith(p)) || pathname.startsWith("/_next")) {
    return NextResponse.next();
  }

  if (!request.cookies.get(SESSION_COOKIE)) {
    const url = new URL("/login", request.url);
    return NextResponse.redirect(url);
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico|manifest.json|icons).*)"],
};
