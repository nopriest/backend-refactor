package middleware

import (
    "net/http"
    "strings"
)

// Normalize standardizes request fields coming through proxies (Vercel/Cloudflare)
// - Trims whitespace around URL.Path to avoid paths like "/api/oauth/callback%20%20"
// - Restores scheme/host from forwarding headers for logs and absolute-URL construction
func Normalize() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Trim any leading/trailing whitespace in path
            if p := r.URL.Path; strings.TrimSpace(p) != p {
                r.URL.Path = strings.TrimSpace(p)
            }

            // Restore scheme/host for downstream consumers
            if xfproto := r.Header.Get("X-Forwarded-Proto"); xfproto != "" {
                r.URL.Scheme = xfproto
            }
            if xfhost := r.Header.Get("X-Forwarded-Host"); xfhost != "" {
                r.Host = xfhost
            }
            next.ServeHTTP(w, r)
        })
    }
}

