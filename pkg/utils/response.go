package utils

import (
	"encoding/json"
	"net/http"
)

// APIResponse 标准API响应结构
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// APIError 错误信息结构
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Meta 元数据结构（用于分页等）
type Meta struct {
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"per_page,omitempty"`
	Total      int `json:"total,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
}

// WriteJSONResponse 写入JSON响应
func WriteJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := APIResponse{
		Success: statusCode >= 200 && statusCode < 300,
		Data:    data,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		// 如果编码失败，写入简单的错误响应
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// WriteSuccessResponse 写入成功响应
func WriteSuccessResponse(w http.ResponseWriter, data interface{}) {
	WriteJSONResponse(w, http.StatusOK, data)
}

// WriteCreatedResponse 写入创建成功响应
func WriteCreatedResponse(w http.ResponseWriter, data interface{}) {
	WriteJSONResponse(w, http.StatusCreated, data)
}

// WriteErrorResponse 写入错误响应
func WriteErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	WriteErrorResponseWithCode(w, statusCode, "ERROR", message, "")
}

// WriteErrorResponseWithCode 写入带错误代码的错误响应
func WriteErrorResponseWithCode(w http.ResponseWriter, statusCode int, code, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		// 如果编码失败，写入简单的错误响应
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// WriteBadRequestResponse 写入400错误响应
func WriteBadRequestResponse(w http.ResponseWriter, message string) {
	WriteErrorResponseWithCode(w, http.StatusBadRequest, "BAD_REQUEST", message, "")
}

// WriteUnauthorizedResponse 写入401错误响应
func WriteUnauthorizedResponse(w http.ResponseWriter, message string) {
	WriteErrorResponseWithCode(w, http.StatusUnauthorized, "UNAUTHORIZED", message, "")
}

// WriteForbiddenResponse 写入403错误响应
func WriteForbiddenResponse(w http.ResponseWriter, message string) {
	WriteErrorResponseWithCode(w, http.StatusForbidden, "FORBIDDEN", message, "")
}

// WriteNotFoundResponse 写入404错误响应
func WriteNotFoundResponse(w http.ResponseWriter, message string) {
	WriteErrorResponseWithCode(w, http.StatusNotFound, "NOT_FOUND", message, "")
}

// WriteConflictResponse 写入409错误响应
func WriteConflictResponse(w http.ResponseWriter, message string) {
	WriteErrorResponseWithCode(w, http.StatusConflict, "CONFLICT", message, "")
}

// WriteInternalServerErrorResponse 写入500错误响应
func WriteInternalServerErrorResponse(w http.ResponseWriter, message string) {
	WriteErrorResponseWithCode(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", message, "")
}

// WriteValidationErrorResponse 写入验证错误响应
func WriteValidationErrorResponse(w http.ResponseWriter, message string, details string) {
	WriteErrorResponseWithCode(w, http.StatusBadRequest, "VALIDATION_ERROR", message, details)
}

// WritePaginatedResponse 写入分页响应
func WritePaginatedResponse(w http.ResponseWriter, data interface{}, page, perPage, total int) {
	totalPages := (total + perPage - 1) / perPage // 向上取整

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := APIResponse{
		Success: true,
		Data:    data,
		Meta: &Meta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// ParseJSONBody 解析JSON请求体
func ParseJSONBody(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// GetQueryParam 获取查询参数，如果不存在则返回默认值
func GetQueryParam(r *http.Request, key, defaultValue string) string {
	if value := r.URL.Query().Get(key); value != "" {
		return value
	}
	return defaultValue
}
