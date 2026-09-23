package httpapi

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// staticHandler раздаёт собранный фронт (web/dist). Путь без файла → index.html (SPA-fallback).
// Если сборки нет — текстовая подсказка. Наличие index.html проверяется на каждый запрос,
// чтобы `npm run build` подхватывался без рестарта сервера.
func staticHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if _, err := os.Stat(index); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("frontend not built; see README\nAPI: /api/health, /api/tasks\n"))
			return
		}
		name := path.Clean("/" + r.URL.Path)
		if name != "/" {
			fi, err := os.Stat(filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(name, "/"))))
			if err != nil || fi.IsDir() {
				http.ServeFile(w, r, index)
				return
			}
		}
		files.ServeHTTP(w, r)
	})
}
