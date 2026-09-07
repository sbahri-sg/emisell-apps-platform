package httpapi

import (
	"context"
	"net/http"
	"time"
)

// overview contains aggregates only: no merchant, credential, or session identifiers.
func (s Server) overview(w http.ResponseWriter, r *http.Request) {
	if s.OverviewPool == nil {
		write(w, 503, map[string]string{"error": "overview_unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var apps, developers, installations, pending, webhookPending, webhookDead, sessions int64
	err := s.OverviewPool.QueryRow(ctx, `SELECT
 (SELECT count(DISTINCT app_id) FROM platform_app.catalog_releases WHERE status='published'),
 (SELECT count(*) FROM platform_identity.portal_accounts WHERE surface='developer' AND enabled),
 (SELECT count(*) FROM platform_installation.installations WHERE status='active'),
 (SELECT count(*) FROM platform_review.submissions WHERE status='submitted'),
 (SELECT count(*) FROM platform_webhook.deliveries WHERE status='pending'),
 (SELECT count(*) FROM platform_webhook.deliveries WHERE status='dead'),
 (SELECT count(*) FROM platform_identity.portal_sessions s JOIN platform_identity.portal_accounts a ON a.id=s.account_id WHERE s.expires_at>now() AND a.enabled)
 `).Scan(&apps, &developers, &installations, &pending, &webhookPending, &webhookDead, &sessions)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	rows, err := s.OverviewPool.Query(ctx, `WITH days AS (
 SELECT generate_series((now() AT TIME ZONE 'UTC')::date-29,(now() AT TIME ZONE 'UTC')::date,'1 day')::date AS day
 ), counts AS (
 SELECT (occurred_at AT TIME ZONE 'UTC')::date AS day,count(*) AS total
 FROM platform_installation.events WHERE envelope->>'type'='emisell.app.installed.v1'
 AND occurred_at >= ((now() AT TIME ZONE 'UTC')::date-29) AT TIME ZONE 'UTC'
 GROUP BY 1
 ) SELECT to_char(days.day,'YYYY-MM-DD'),coalesce(counts.total,0) FROM days LEFT JOIN counts USING(day) ORDER BY days.day`)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	defer rows.Close()
	type point struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	history := make([]point, 0, 30)
	for rows.Next() {
		var p point
		if err = rows.Scan(&p.Date, &p.Count); err != nil {
			s.fail(w, r, err)
			return
		}
		history = append(history, p)
	}
	if err = rows.Err(); err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 200, map[string]any{"publishedApps": apps, "developers": developers, "activeInstallations": installations, "pendingReviews": pending, "history": history, "webhookPending": webhookPending, "webhookDead": webhookDead, "portalSessions": sessions, "checkedAt": time.Now().UTC()})
}
