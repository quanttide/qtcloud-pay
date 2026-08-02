package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestRun_InvalidAddr(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_SQLITE_DSN", filepath.Join(t.TempDir(), "run.db"))
	err := run(context.Background(), "bad addr", "")
	if err == nil {
		t.Fatal("expected error for invalid addr")
	}
}

func TestRun_OpenDBError(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", "://bad")
	if err := run(context.Background(), "127.0.0.1:0", ""); err == nil {
		t.Fatal("expected openDB error")
	}
}

func TestRun_BuildMuxError(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_SQLITE_DSN", filepath.Join(t.TempDir(), "run.db"))
	if err := run(context.Background(), "127.0.0.1:0", "unionpay"); err == nil {
		t.Fatal("expected buildMux error")
	}
}

func TestRun_ServeAndShutdown(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_SQLITE_DSN", filepath.Join(t.TempDir(), "run.db"))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, "127.0.0.1:0", "")
	}()

	// 等待服务启动（run 内部监听 127.0.0.1:0，端口未知，仅验证启动后随取消正常退出）
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not exit after cancel")
	}
}
