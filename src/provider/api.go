package provider

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
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
	mux.HandleFunc("POST /pay", s.handlePay)
	mux.HandleFunc("GET /query/{order_id}", s.handleQuery)
	mux.HandleFunc("POST /refund", s.handleRefund)
	s.mux = mux
	s.srv = &http.Server{Addr: addr, Handler: mux}
	return s
}

// Handler 返回 HTTP handler（用于测试）。
func (s *Server) Handler() http.Handler { return s.mux }

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

// writeJSON 以 JSON 格式写入响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json response: %v", err)
	}
}

// writeError 写入错误响应。内部错误细节只记录日志，不返回给客户端。
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handlePay(w http.ResponseWriter, r *http.Request) {
	var req PayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := s.provider.Pay(r.Context(), &req)
	if err != nil {
		log.Printf("pay: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(r.PathValue("order_id"))
	if orderID == "" {
		writeError(w, http.StatusBadRequest, "missing order_id")
		return
	}
	status, err := s.provider.Query(r.Context(), orderID)
	if err != nil {
		log.Printf("query: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleRefund(w http.ResponseWriter, r *http.Request) {
	var req RefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := s.provider.Refund(r.Context(), &req)
	if err != nil {
		log.Printf("refund: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
