package server

import (
	"html/template"
	"strings"
)

// Precompiled templates for status pages.
// These are parsed once at package initialization for efficiency.
var (
	provisioningTemplate = template.Must(template.New("provisioning").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta http-equiv="refresh" content="10">
    <title>Securing Connection</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
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
        h1 { color: #333; margin: 0 0 20px; font-size: 28px; }
        .lock-icon { font-size: 48px; margin-bottom: 20px; }
        .domain { color: #667eea; font-weight: 600; font-size: 18px; }
        p { color: #666; line-height: 1.6; margin: 15px 0; }
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
            animation: progress 2s ease-in-out infinite;
        }
        @keyframes progress {
            0% { transform: translateX(-100%); }
            50% { transform: translateX(0); }
            100% { transform: translateX(100%); }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="lock-icon">🔒</div>
        <h1>Setting up secure connection</h1>
        <p>We're configuring SSL/TLS for</p>
        <p class="domain">{{.Domain}}</p>
        <div class="progress"><div class="progress-bar"></div></div>
        <p>This typically takes 30-60 seconds.</p>
        <p style="font-size: 14px; color: #999;">This page will refresh automatically</p>
    </div>
</body>
</html>`))

	failedTemplate = template.Must(template.New("failed").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Configuration Required</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
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
        h1 { color: #d93025; margin: 0 0 20px; font-size: 28px; }
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
        h3 { color: #333; margin: 30px 0 15px; }
        ul { color: #666; line-height: 1.8; }
        li { margin: 8px 0; }
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
            <li>DNS records not properly configured</li>
            <li>Domain not pointing to our servers</li>
            <li>CAA records blocking Let's Encrypt</li>
            <li>Rate limits exceeded</li>
        </ul>
        <p>Please verify your DNS settings and contact support if the issue persists.</p>
    </div>
</body>
</html>`))
)

const defaultNotFoundHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>404 - Domain Not Found</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: #f5f5f5;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            margin: 0;
        }
        .container { text-align: center; }
        h1 { font-size: 120px; color: #e0e0e0; margin: 0; font-weight: 700; }
        h2 { color: #333; margin: 20px 0; font-size: 28px; }
        p { color: #666; font-size: 18px; }
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

// buildProvisioningHTML generates the provisioning status page using precompiled template.
func buildProvisioningHTML(info *DomainInfo) string {
	var buf strings.Builder
	if err := provisioningTemplate.Execute(&buf, info); err != nil {
		// Fallback to a simple error page if template execution fails
		return "<!DOCTYPE html><html><body><h1>Error rendering page</h1></body></html>"
	}
	return buf.String()
}

// buildFailedHTML generates the failed status page using precompiled template.
func buildFailedHTML(info *DomainInfo) string {
	var buf strings.Builder
	if err := failedTemplate.Execute(&buf, info); err != nil {
		// Fallback to a simple error page if template execution fails
		return "<!DOCTYPE html><html><body><h1>Error rendering page</h1></body></html>"
	}
	return buf.String()
}
