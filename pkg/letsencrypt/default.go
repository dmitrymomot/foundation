package letsencrypt

import (
	"fmt"
	"html/template"
	"net/http"
	"time"
)

// DefaultStatusPages provides default HTML status page implementations.
type DefaultStatusPages struct {
	// Optional custom HTML templates.
	// If empty, uses built-in templates.
	ProvisioningHTML string
	FailedHTML       string
	NotFoundHTML     string
}

// NewDefaultStatusPages creates a new status page handler with default templates.
func NewDefaultStatusPages() *DefaultStatusPages {
	return &DefaultStatusPages{}
}

// ServeProvisioning shows a page indicating certificate generation is in progress.
func (d *DefaultStatusPages) ServeProvisioning(w http.ResponseWriter, r *http.Request, info *DomainInfo) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Retry-After", "10")
	w.WriteHeader(http.StatusAccepted)

	html := d.ProvisioningHTML
	if html == "" {
		html = defaultProvisioningTemplate
	}

	// Simple template replacement
	htmlStr := replaceTemplate(html, map[string]string{
		"{{.Domain}}":    template.HTMLEscapeString(info.Domain),
		"{{.StartTime}}": info.CreatedAt.Format(time.RFC3339),
		"{{.Elapsed}}":   time.Since(info.CreatedAt).Round(time.Second).String(),
	})

	fmt.Fprint(w, htmlStr)
}

// ServeFailed shows a page explaining why certificate generation failed.
func (d *DefaultStatusPages) ServeFailed(w http.ResponseWriter, r *http.Request, info *DomainInfo) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusServiceUnavailable)

	html := d.FailedHTML
	if html == "" {
		html = defaultFailedTemplate
	}

	// Simple template replacement
	htmlStr := replaceTemplate(html, map[string]string{
		"{{.Domain}}": template.HTMLEscapeString(info.Domain),
		"{{.Error}}":  template.HTMLEscapeString(info.Error),
	})

	fmt.Fprint(w, htmlStr)
}

// ServeNotFound shows a 404 page for unregistered domains.
func (d *DefaultStatusPages) ServeNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)

	html := d.NotFoundHTML
	if html == "" {
		html = defaultNotFoundTemplate
	}

	fmt.Fprint(w, html)
}

// replaceTemplate performs simple string replacement for template variables.
func replaceTemplate(template string, vars map[string]string) string {
	result := template
	for key, value := range vars {
		result = replaceAll(result, key, value)
	}
	return result
}

// replaceAll replaces all occurrences of old with new in s.
func replaceAll(s, old, new string) string {
	if old == "" {
		return s
	}
	result := ""
	for {
		i := indexString(s, old)
		if i == -1 {
			return result + s
		}
		result += s[:i] + new
		s = s[i+len(old):]
	}
}

// indexString returns the index of substr in s, or -1 if not found.
func indexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

const defaultProvisioningTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta http-equiv="refresh" content="10">
    <title>Securing Connection</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            margin: 0;
            padding: 20px;
        }
        .container {
            background: white;
            border-radius: 12px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.15);
            padding: 40px;
            max-width: 500px;
            width: 100%;
            text-align: center;
        }
        h1 {
            color: #333;
            margin: 0 0 20px;
            font-size: 28px;
        }
        .lock-icon {
            font-size: 48px;
            margin-bottom: 20px;
        }
        .domain {
            color: #667eea;
            font-weight: 600;
            font-size: 18px;
        }
        p {
            color: #666;
            line-height: 1.6;
            margin: 15px 0;
        }
        .progress {
            width: 100%;
            height: 6px;
            background: #f0f0f0;
            border-radius: 3px;
            overflow: hidden;
            margin: 30px 0;
        }
        .progress-bar {
            height: 100%;
            background: linear-gradient(90deg, #667eea, #764ba2);
            border-radius: 3px;
            animation: progress 2s ease-in-out infinite;
        }
        @keyframes progress {
            0% { transform: translateX(-100%); }
            50% { transform: translateX(0); }
            100% { transform: translateX(100%); }
        }
        .info {
            font-size: 14px;
            color: #999;
            margin-top: 20px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="lock-icon">🔒</div>
        <h1>Setting up secure connection</h1>
        <p>We're configuring SSL/TLS for</p>
        <p class="domain">{{.Domain}}</p>
        <div class="progress">
            <div class="progress-bar"></div>
        </div>
        <p>This typically takes 30-60 seconds. Please wait while we secure your connection.</p>
        <p class="info">This page will refresh automatically</p>
    </div>
</body>
</html>`

const defaultFailedTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Configuration Required</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: #f5f5f5;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            margin: 0;
            padding: 20px;
        }
        .container {
            background: white;
            border-radius: 12px;
            box-shadow: 0 4px 20px rgba(0,0,0,0.1);
            padding: 40px;
            max-width: 600px;
            width: 100%;
        }
        h1 {
            color: #d93025;
            margin: 0 0 20px;
            font-size: 28px;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .domain {
            color: #333;
            font-weight: 600;
            background: #f8f9fa;
            padding: 8px 12px;
            border-radius: 6px;
            display: inline-block;
            margin: 10px 0;
        }
        .error-box {
            background: #fef2f2;
            border: 1px solid #fecaca;
            border-radius: 8px;
            padding: 16px;
            margin: 20px 0;
        }
        .error-message {
            color: #b91c1c;
            font-family: 'Courier New', monospace;
            font-size: 14px;
            word-break: break-all;
        }
        h3 {
            color: #333;
            margin: 30px 0 15px;
        }
        ul {
            color: #666;
            line-height: 1.8;
        }
        li {
            margin: 8px 0;
        }
        .support {
            background: #f8f9fa;
            border-radius: 8px;
            padding: 20px;
            margin-top: 30px;
            text-align: center;
        }
        .support p {
            color: #666;
            margin: 0;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>⚠️ Domain Configuration Required</h1>
        <p>Unable to set up SSL/TLS certificate for:</p>
        <div class="domain">{{.Domain}}</div>

        <div class="error-box">
            <strong>Error Details:</strong>
            <div class="error-message">{{.Error}}</div>
        </div>

        <h3>Common causes:</h3>
        <ul>
            <li>DNS records not properly configured or propagated</li>
            <li>Domain not pointing to our servers</li>
            <li>CAA records blocking Let's Encrypt</li>
            <li>Let's Encrypt rate limits exceeded</li>
            <li>Firewall blocking ACME validation</li>
        </ul>

        <div class="support">
            <p>Please verify your DNS settings and try again.</p>
            <p>If the problem persists, contact your administrator.</p>
        </div>
    </div>
</body>
</html>`

const defaultNotFoundTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>404 - Domain Not Found</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: #f5f5f5;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            margin: 0;
            padding: 20px;
        }
        .container {
            text-align: center;
            max-width: 500px;
        }
        h1 {
            font-size: 120px;
            color: #e0e0e0;
            margin: 0;
            font-weight: 700;
        }
        h2 {
            color: #333;
            margin: 20px 0;
            font-size: 28px;
        }
        p {
            color: #666;
            font-size: 18px;
            line-height: 1.6;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>404</h1>
        <h2>Domain Not Found</h2>
        <p>The requested domain is not configured on this server.</p>
    </div>
</body>
</html>`
