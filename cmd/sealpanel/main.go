package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"web-sealdice/internal/bootstrap"
	"web-sealdice/internal/config"
	"web-sealdice/internal/server"
	buildver "web-sealdice/internal/version"
)

// main 为程序入口：加载配置、准备日志与运行环境、启动 HTTP 服务并处理优雅退出。
func main() {
	configPath := flag.String("config", "config.yaml", "Path to config file")
	printVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()
	if *printVersion {
		fmt.Println(buildver.Display())
		return
	}

	cfg, created, err := config.LoadOrCreate(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	runLogs, err := bootstrap.PrepareRunLogs(cfg)
	if err != nil {
		log.Fatalf("failed to prepare run logs: %v", err)
	}
	runtimeFile, err := os.OpenFile(runLogs.RuntimeLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatalf("failed to open runtime log: %v", err)
	}
	defer func() { _ = runtimeFile.Close() }()
	accessFile, err := os.OpenFile(runLogs.AccessLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatalf("failed to open access log: %v", err)
	}
	defer func() { _ = accessFile.Close() }()
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(io.MultiWriter(os.Stdout, runtimeFile))
	accessLogger := log.New(accessFile, "", 0)

	if created {
		log.Printf("created default config at %s", *configPath)
	}

	if err := bootstrap.EnsureLayout(cfg); err != nil {
		log.Fatalf("failed to initialize directories: %v", err)
	}
	if os.Geteuid() != 0 {
		log.Fatalf("web-sealdice must run as root (required for LuckyLilliaBot/pmhq permission setup)")
	}
	if err := bootstrap.EnsureDotnet9(); err != nil {
		log.Fatalf("failed to ensure .NET 9: %v", err)
	}
	if err := bootstrap.EnsureLLBotDeps(); err != nil {
		log.Fatalf("failed to ensure LLBot dependencies: %v", err)
	}

	app, err := server.New(cfg, *configPath)
	if err != nil {
		log.Fatalf("failed to build server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr(),
		Handler:           accessLogMiddleware(app.Handler(), accessLogger),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("web ui listening on http://%s%s", cfg.ListenAddr(), cfg.BasePath)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server crashed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Printf("received shutdown signal, stopping managed services")
	app.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
}

// accessRecorder 包装 ResponseWriter，用于记录状态码与响应字节数。
type accessRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

// WriteHeader 记录状态码并向下游写回响应头。
func (r *accessRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// Write 记录响应体字节数；若上层未显式写状态码，则按 200 处理。
func (r *accessRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(p)
	r.bytes += n
	return n, err
}

// accessLogMiddleware 为每个请求输出访问日志（方法、路径、状态码、耗时、UA 等）。
func accessLogMiddleware(next http.Handler, logger *log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &accessRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logger.Printf(
			`time=%s remote=%q method=%s host=%q uri=%q status=%d bytes=%d dur_ms=%d ua=%q`,
			start.Format(time.RFC3339),
			r.RemoteAddr,
			r.Method,
			r.Host,
			r.RequestURI,
			rec.status,
			rec.bytes,
			time.Since(start).Milliseconds(),
			r.UserAgent(),
		)
	})
}
