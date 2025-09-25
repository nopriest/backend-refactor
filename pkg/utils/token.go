package utils

import (
    "crypto/rand"
    "encoding/base64"
)

// GenerateURLToken 生成 URL-safe 的随机 token，长度约为 4/3*n 字符
// n 为原始随机字节数，推荐 24 或 32
func GenerateURLToken(n int) (string, error) {
    if n <= 0 {
        n = 24
    }
    b := make([]byte, n)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    // 使用 RawURLEncoding，避免出现 '=' 填充与 '+' '/' 字符
    return base64.RawURLEncoding.EncodeToString(b), nil
}

