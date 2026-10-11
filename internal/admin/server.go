package admin

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// HTTPHandler 组装全部路由：/api/{uri} 开放接口 + 管理台 REST + 静态前端。
func (s *Service) HTTPHandler(staticDir string) http.Handler {
	mux := http.NewServeMux()

	// 开放 API（执行器调用，token 鉴权）
	mux.HandleFunc("POST /api/{uri}", func(w http.ResponseWriter, r *http.Request) {
		s.handleOpenAPI(w, r)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		s.handleOpenAPI(w, r)
	})

	// 登录相关（无需登录态）
	mux.HandleFunc("POST /auth/login", s.handleLogin)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	mux.HandleFunc("GET /auth/user", s.handleUserInfo)

	// 管理台 REST（需登录态）
	auth := func(h func(w http.ResponseWriter, r *http.Request, u *JobUser)) http.HandlerFunc {
		return s.requireAuth(h)
	}
	mux.HandleFunc("POST /jobinfo/pageList", auth(s.handleJobPageList))
	mux.HandleFunc("POST /jobinfo/add", auth(s.handleJobSave))
	mux.HandleFunc("POST /jobinfo/update", auth(s.handleJobUpdate))
	mux.HandleFunc("POST /jobinfo/remove", auth(s.handleJobRemove))
	mux.HandleFunc("POST /jobinfo/start", auth(func(w http.ResponseWriter, r *http.Request, u *JobUser) {
		s.handleJobStartStop(w, r, u, true)
	}))
	mux.HandleFunc("POST /jobinfo/stop", auth(func(w http.ResponseWriter, r *http.Request, u *JobUser) {
		s.handleJobStartStop(w, r, u, false)
	}))
	mux.HandleFunc("POST /jobinfo/trigger", auth(s.handleJobTrigger))
	mux.HandleFunc("GET /jobinfo/nextTriggerTime", s.handleNextTriggerTime)

	mux.HandleFunc("POST /jobgroup/pageList", auth(s.handleGroupPageList))
	mux.HandleFunc("POST /jobgroup/save", auth(s.handleGroupSave))
	mux.HandleFunc("POST /jobgroup/remove", auth(s.handleGroupRemove))

	mux.HandleFunc("POST /joblog/pageList", auth(s.handleLogPageList))
	mux.HandleFunc("POST /joblog/logDetailCat", auth(s.handleLogDetailCat))
	mux.HandleFunc("POST /joblog/logKill", auth(s.handleLogKill))
	mux.HandleFunc("POST /joblog/clearLog", auth(s.handleLogClear))

	mux.HandleFunc("POST /user/pageList", auth(s.handleUserPageList))
	mux.HandleFunc("POST /user/save", auth(s.handleUserSave))
	mux.HandleFunc("POST /user/remove", auth(s.handleUserRemove))
	mux.HandleFunc("POST /auth/updatePwd", auth(s.handleUpdatePwd))

	mux.HandleFunc("GET /index/chartInfo", auth(s.handleChartInfo))
	mux.HandleFunc("POST /jobcode/save", auth(s.handleGlueSave))

	// 静态前端（SPA fallback 到 index.html）
	if staticDir != "" {
		fileServer := http.FileServer(http.Dir(staticDir))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Clean(r.URL.Path)
			full := filepath.Join(staticDir, path)
			if st, err := os.Stat(full); err != nil || st.IsDir() {
				if !strings.HasPrefix(path, "/assets") {
					// SPA 路由回退
					data, err := os.ReadFile(filepath.Join(staticDir, "index.html"))
					if err == nil {
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						_, _ = w.Write(data)
						return
					}
				}
				http.NotFound(w, r)
				return
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	return mux
}

var _ = fs.ValidPath
