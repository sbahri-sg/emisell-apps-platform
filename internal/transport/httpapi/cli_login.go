package httpapi

import (
	"emisell.app/platform/internal/platform/fault"
	"github.com/go-chi/chi/v5"
	"html/template"
	"net/http"
)

var cliConfirmPage = template.Must(template.New("cli-login").Parse(`<!doctype html><html lang="id"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Login Emisell CLI</title><style>body{margin:0;background:#0d1518;color:#e8eeee;font:16px/1.6 system-ui;min-height:100vh;display:grid;place-items:center}main{box-sizing:border-box;width:min(92vw,480px);padding:32px;background:#152326;border:1px solid #304448;border-radius:16px}h1{font-size:24px}button{font:inherit;padding:10px 20px;border:0;border-radius:8px;background:#8cf2d2;color:#0d1518;cursor:pointer}p{overflow-wrap:anywhere}a{color:#8cf2d2}</style><main><small>emisell · developer</small><h1>{{if .Done}}CLI terhubung{{else}}Izinkan login Emisell CLI?{{end}}</h1>{{if .Done}}<p>Kembali ke terminal untuk melanjutkan. Anda dapat menutup halaman ini.</p>{{else}}<p>Akun: <strong>{{.Email}}</strong></p><p>CLI dapat mengelola aplikasi developer Anda. Ini tidak memasang aplikasi atau memberi akses data toko.</p><p>Lanjutkan hanya jika Anda sendiri menjalankan login dari terminal.</p><form method="post"><input type="hidden" name="request" value="{{.Request}}"><input type="hidden" name="code" value="{{.Code}}"><button type="submit">Izinkan login CLI</button></form><p><a href="/development">Batal</a></p>{{end}}</main></html>`))

func (s Server) cliLoginRoutes(r chi.Router) {
	r.Post("/cli/start", func(w http.ResponseWriter, r *http.Request) {
		origin := s.developerBrowserOrigin(r)
		if origin == "" || r.Header.Get("Origin") != origin || r.URL.RawQuery != "" {
			s.fail(w, r, fault.Forbidden)
			return
		}
		seller := s.sellerOrigin()
		if seller == "" {
			s.fail(w, r, fault.Unavailable)
			return
		}
		var body struct{}
		if err := decode(w, r, &body); err != nil {
			s.fail(w, r, err)
			return
		}
		request, proof, err := s.DeveloperLogin.StartCLI(r.Context(), origin)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		write(w, 200, map[string]any{"request": request, "verifier": proof, "authorizeUrl": merchantLoginURL(seller, request), "expiresIn": 300, "interval": 3})
	})
	r.Post("/cli/poll", func(w http.ResponseWriter, r *http.Request) {
		origin := s.developerBrowserOrigin(r)
		if origin == "" || r.Header.Get("Origin") != origin || r.URL.RawQuery != "" {
			s.fail(w, r, fault.Forbidden)
			return
		}
		var in struct {
			Request  string `json:"request"`
			Verifier string `json:"verifier"`
		}
		if err := decode(w, r, &in); err != nil {
			s.fail(w, r, err)
			return
		}
		token, err := s.DeveloperLogin.PollCLI(r.Context(), in.Request, in.Verifier, origin)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if token == "" {
			write(w, 200, map[string]string{"status": "pending"})
			return
		}
		// CLI uses the unified cookie name; it never reads or imports browser cookies.
		s.unifiedCookie(w, token, 3600)
		write(w, 200, map[string]string{"status": "authorized"})
	})
	confirm := func(w http.ResponseWriter, r *http.Request) {
		origin := s.developerBrowserOrigin(r)
		if origin == "" {
			s.fail(w, r, fault.Forbidden)
			return
		}
		request, code := r.URL.Query().Get("request"), r.URL.Query().Get("code")
		post := r.Method == "POST"
		if post {
			if r.Header.Get("Origin") != origin {
				s.fail(w, r, fault.Forbidden)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 4096)
			if err := r.ParseForm(); err != nil {
				s.fail(w, r, fault.Invalid)
				return
			}
			request, code = r.PostForm.Get("request"), r.PostForm.Get("code")
		}
		email, err := s.DeveloperLogin.CLIConfirmation(r.Context(), request, code, origin, post)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
		_ = cliConfirmPage.Execute(w, map[string]any{"Done": post, "Request": request, "Code": code, "Email": email})
	}
	r.Get("/cli/confirm", confirm)
	r.Post("/cli/confirm", confirm)
}
