// Package main 服务端入口：加载配置、组装依赖、启动服务。
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/app"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/middleware"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
	channelName := flag.String("channel", "", "支付渠道：wechat 或 alipay（可为空，仅启动账本 API）")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, *addr, *channelName); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// run 打开数据库、组装路由并启动服务（含优雅关闭）。
func run(ctx context.Context, addr, channelName string) error {
	db, err := app.OpenDB()
	if err != nil {
		return err
	}
	mux, err := app.BuildMux(db, channelName, os.Getenv("ADMIN_TOKEN"))
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	log.Printf("API server listening on %s", ln.Addr())

	srv := &http.Server{Handler: middleware.Logging(mux)}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
