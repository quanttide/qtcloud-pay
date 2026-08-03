package channel

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/httpapi"
)

// Server 支付 API HTTP 服务。
type Server struct {
	provider Provider
	mux      *http.ServeMux
	srv      *http.Server
}

// NewServer 创建 API 服务。
func NewServer(addr string, p Provider) *Server {
	s := &Server{provider: p}
	mux := http.NewServeMux()
	RegisterRoutes(mux, p)
	s.mux = mux
	s.srv = &http.Server{Addr: addr, Handler: mux}
	return s
}

// RegisterRoutes 在给定 mux 上注册渠道 API 路由（供服务端统一挂载）。
func RegisterRoutes(mux *http.ServeMux, p Provider) {
	s := &Server{provider: p}
	mux.HandleFunc("POST /pay", s.handlePay)
	mux.HandleFunc("GET /query/{order_id}", s.handleQuery)
	mux.HandleFunc("POST /refund", s.handleRefund)
}

// Handler 返回 HTTP handler（用于测试）。
func (s *Server) Handler() http.Handler { return s.mux }

// SetHandler 替换 HTTP handler（用于挂载中间件）。
func (s *Server) SetHandler(h http.Handler) { s.srv.Handler = h }

// Start 启动服务。
func (s *Server) Start() error {
	log.Printf("API server listening on %s", s.srv.Addr)
	return s.srv.ListenAndServe()
}

// Close 关闭服务。
func (s *Server) Close() error { return s.srv.Close() }

// Shutdown 优雅关闭。
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (s *Server) handlePay(w http.ResponseWriter, r *http.Request) {
	var req PayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := s.provider.Pay(r.Context(), &req)
	if err != nil {
		log.Printf("pay: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(r.PathValue("order_id"))
	if orderID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "missing order_id")
		return
	}
	status, err := s.provider.Query(r.Context(), orderID)
	if err != nil {
		log.Printf("query: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, status)
}

func (s *Server) handleRefund(w http.ResponseWriter, r *http.Request) {
	var req RefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := s.provider.Refund(r.Context(), &req)
	if err != nil {
		log.Printf("refund: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}
