package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetDeviceType(t *testing.T) {
	tests := []struct {
		name           string
		userAgent      string
		cloudFrontMobile string
		want           string
	}{
		{
			name:             "CloudFront mobile header takes precedence",
			userAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
			cloudFrontMobile: "true",
			want:             "mobile",
		},
		{
			name:      "iPhone detected as mobile",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X)",
			want:      "mobile",
		},
		{
			name:      "Android mobile detected as mobile",
			userAgent: "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
			want:      "mobile",
		},
		{
			name:      "iPad detected as tablet",
			userAgent: "Mozilla/5.0 (iPad; CPU OS 16_0 like Mac OS X)",
			want:      "tablet",
		},
		{
			name:      "Android tablet detected as tablet",
			userAgent: "Mozilla/5.0 (Linux; Android 13; SM-X710) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			want:      "tablet",
		},
		{
			name:      "Samsung Smart TV detected",
			userAgent: "Mozilla/5.0 (SMART-TV; Linux; Tizen 6.0)",
			want:      "smarttv",
		},
		{
			name:      "Roku detected as smarttv",
			userAgent: "Roku/DVP-9.10 (519.10E04111A)",
			want:      "smarttv",
		},
		{
			name:      "WebOS TV detected",
			userAgent: "Mozilla/5.0 (Web0S; Linux/SmartTV) AppleWebKit/537.36",
			want:      "smarttv",
		},
		{
			name:      "Windows desktop",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			want:      "desktop",
		},
		{
			name:      "Mac desktop",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			want:      "desktop",
		},
		{
			name:      "Empty user agent returns unknown",
			userAgent: "",
			want:      "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			c.Request.Header.Set("User-Agent", tt.userAgent)
			if tt.cloudFrontMobile != "" {
				c.Request.Header.Set("CloudFront-Is-Mobile-Viewer", tt.cloudFrontMobile)
			}

			got := GetDeviceType(c)
			if got != tt.want {
				t.Errorf("GetDeviceType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetDeviceOS(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		want      string
	}{
		{
			name:      "Android phone",
			userAgent: "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36",
			want:      "android",
		},
		{
			name:      "Android tablet",
			userAgent: "Mozilla/5.0 (Linux; Android 13; SM-X710) AppleWebKit/537.36",
			want:      "android",
		},
		{
			name:      "iPhone",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X)",
			want:      "ios",
		},
		{
			name:      "iPad",
			userAgent: "Mozilla/5.0 (iPad; CPU OS 16_0 like Mac OS X)",
			want:      "ios",
		},
		{
			name:      "iPod",
			userAgent: "Mozilla/5.0 (iPod touch; CPU iPhone OS 15_0 like Mac OS X)",
			want:      "ios",
		},
		{
			name:      "Windows desktop",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			want:      "other",
		},
		{
			name:      "Mac desktop",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
			want:      "other",
		},
		{
			name:      "Linux desktop",
			userAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
			want:      "other",
		},
		{
			name:      "Empty user agent",
			userAgent: "",
			want:      "other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			c.Request.Header.Set("User-Agent", tt.userAgent)

			got := GetDeviceOS(c)
			if got != tt.want {
				t.Errorf("GetDeviceOS() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetRealClientIP(t *testing.T) {
	tests := []struct {
		name               string
		cloudFrontAddress  string
		fallbackIP         string
		want               string
	}{
		{
			name:              "CloudFront header with port",
			cloudFrontAddress: "203.0.113.50:12345",
			want:              "203.0.113.50",
		},
		{
			name:              "CloudFront header IPv6 with port",
			cloudFrontAddress: "2001:db8::1:54321",
			want:              "2001:db8::1",
		},
		{
			name:              "No CloudFront header uses fallback",
			cloudFrontAddress: "",
			fallbackIP:        "192.168.1.1",
			want:              "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.cloudFrontAddress != "" {
				c.Request.Header.Set("CloudFront-Viewer-Address", tt.cloudFrontAddress)
			}
			if tt.fallbackIP != "" {
				c.Request.RemoteAddr = tt.fallbackIP + ":8080"
			}

			got := GetRealClientIP(c)
			if got != tt.want {
				t.Errorf("GetRealClientIP() = %v, want %v", got, tt.want)
			}
		})
	}
}
