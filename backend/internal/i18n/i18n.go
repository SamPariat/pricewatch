// Package i18n translates the app's own user-facing strings — API
// response messages, validation errors, and the Telegram digest/alert
// text — using go-i18n (github.com/nicksnyder/go-i18n/v2) for message
// storage and retrieval. It deliberately does not attempt to translate
// text this app doesn't author: wrapped errors from third-party
// libraries (cron parsing, timezone loading) and JSON field/enum
// identifiers stay in English regardless of locale, the same way a
// stack trace or a raw field name would in any localized system.
//
// Translations live in one shared source of truth: /locales/en.json and
// /locales/hi.json at the monorepo root, under this app's own "backend"
// key — the same two files web/lib/i18n reads directly via next-intl.
// Placeholders use ICU-style {Name} markers (next-intl's native
// MessageFormat syntax) rather than go-i18n's default Go-template
// {{.Name}} syntax, since the file has to parse identically for both
// libraries. That means go-i18n here is deliberately used only for
// bundling and message lookup, not its own template execution —
// substitute (below) does the {Name} replacement itself, after
// go-i18n hands back the raw, unexecuted string.
package i18n

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

type Locale string

const (
	EN Locale = "en"
	HI Locale = "hi"
)

var (
	bundle     = i18n.NewBundle(language.English)
	localizers = map[Locale]*i18n.Localizer{}
)

func init() {
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	dir := localesDir()
	loadBackendMessages(bundle, filepath.Join(dir, "en.json"), "en")
	loadBackendMessages(bundle, filepath.Join(dir, "hi.json"), "hi")
	localizers[EN] = i18n.NewLocalizer(bundle, "en")
	localizers[HI] = i18n.NewLocalizer(bundle, "hi")
}

// localesDir finds the shared /locales directory. The container image
// (see backend/Dockerfile) copies it to /locales at the filesystem root,
// alongside /server — checked first since it's the real deployed shape.
// Local `go run`/`go test` have no such absolute path, so this falls
// back to walking up from the working directory looking for a locales/
// dir, which finds the monorepo root's copy regardless of which
// package under backend/ the walk started from.
func localesDir() string {
	if d := os.Getenv("LOCALES_DIR"); d != "" {
		return d
	}
	if info, err := os.Stat("/locales"); err == nil && info.IsDir() {
		return "/locales"
	}
	dir, err := os.Getwd()
	if err != nil {
		return "locales"
	}
	for range 8 {
		candidate := filepath.Join(dir, "locales")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "locales"
}

// loadBackendMessages reads one shared locale file, pulls out only this
// app's own "backend" subtree (the file also carries web/lib/i18n's
// frontend namespaces at the root, which this package has no use for),
// flattens it to dotted message IDs matching this package's existing key
// names (e.g. "validation.invalid_kind"), and registers each with
// go-i18n. A missing or unparsable file panics at startup rather than
// serving silently-empty translations — the same "fail loud, not
// quiet" choice this app already makes for a failed DB migration.
func loadBackendMessages(b *i18n.Bundle, path, lang string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("i18n: read %s: %v", path, err))
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		panic(fmt.Sprintf("i18n: parse %s: %v", path, err))
	}
	backend, ok := doc["backend"].(map[string]any)
	if !ok {
		panic(fmt.Sprintf("i18n: %s has no \"backend\" section", path))
	}
	flat := map[string]string{}
	flatten("", backend, flat)
	for id, value := range flat {
		if err := b.AddMessages(language.MustParse(lang), &i18n.Message{ID: id, Other: value}); err != nil {
			panic(fmt.Sprintf("i18n: register %s/%s: %v", lang, id, err))
		}
	}
}

func flatten(prefix string, node map[string]any, out map[string]string) {
	for k, v := range node {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case string:
			out[key] = val
		case map[string]any:
			flatten(key, val, out)
		default:
			panic(fmt.Sprintf("i18n: unexpected value type at %q", key))
		}
	}
}

var placeholderRe = regexp.MustCompile(`\{([A-Za-z0-9_]+)\}`)

// substitute replaces {Name} markers with values from kv, an alternating
// name/value list (mirrors internal/logging.With's own variadic
// key/value convention). Unmatched placeholders are left as-is rather
// than blanked — a missing arg should be obviously wrong in the
// rendered text, not silently disappear.
func substitute(s string, kv []any) string {
	if len(kv) == 0 {
		return s
	}
	data := make(map[string]string, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		name, ok := kv[i].(string)
		if !ok {
			continue
		}
		data[name] = fmt.Sprint(kv[i+1])
	}
	return placeholderRe.ReplaceAllStringFunc(s, func(match string) string {
		if v, ok := data[match[1:len(match)-1]]; ok {
			return v
		}
		return match
	})
}

// Parse extracts a supported locale from a raw Accept-Language header
// value (e.g. "hi-IN,hi;q=0.9,en;q=0.8"), taking the first supported tag
// in preference order. Falls back to EN for anything empty, malformed,
// or unsupported — a bad header must never break a response, only pick
// the app's original language.
func Parse(acceptLanguage string) Locale {
	for _, part := range strings.Split(acceptLanguage, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		switch {
		case tag == "hi" || strings.HasPrefix(tag, "hi-"):
			return HI
		case tag == "en" || strings.HasPrefix(tag, "en-"):
			return EN
		}
	}
	return EN
}

// Valid reports whether s is a locale this app actually has a catalog
// for — used to validate Settings.Language on the way in, so a typo
// doesn't silently persist as a locale that always falls back to EN.
func Valid(s string) bool {
	_, ok := localizers[Locale(s)]
	return ok
}

type contextKey struct{}

// CtxKey is exported so internal/httpapi/middleware's locale middleware
// (a separate package, to keep this one framework-agnostic) can store a
// request's resolved Locale directly into fiber.Ctx's Locals — which,
// since fiber.Ctx satisfies context.Context by reading that same Locals
// store in its Value method, makes it visible to From(ctx) anywhere a
// handler passes ctx (== the fiber.Ctx itself) straight into a service
// call, with no per-handler wiring required.
var CtxKey = contextKey{}

// With attaches loc to ctx — for the paths that don't go through an HTTP
// request at all (the scheduler's cron fires, the Telegram bot's own
// long-poll loop), where there's no Accept-Language header to read and
// the locale instead comes from the persisted Settings.Language.
func With(ctx context.Context, loc Locale) context.Context {
	return context.WithValue(ctx, CtxKey, loc)
}

// From returns the locale carried on ctx, or EN if none was ever set.
func From(ctx context.Context) Locale {
	if loc, ok := ctx.Value(CtxKey).(Locale); ok && loc != "" {
		return loc
	}
	return EN
}

// T looks up key's message for loc, falling back to EN and then to the
// bare key — a missing translation must still render something legible,
// never an empty string, since these strings land directly in API
// responses and Telegram messages. kv is an alternating name/value list
// substituted into any {Name} placeholders the message contains.
func T(loc Locale, key string, kv ...any) string {
	l, ok := localizers[loc]
	if !ok {
		l = localizers[EN]
	}
	if msg, err := l.Localize(&i18n.LocalizeConfig{MessageID: key}); err == nil {
		return substitute(msg, kv)
	}
	if loc != EN {
		if msg, err := localizers[EN].Localize(&i18n.LocalizeConfig{MessageID: key}); err == nil {
			return substitute(msg, kv)
		}
	}
	return key
}
