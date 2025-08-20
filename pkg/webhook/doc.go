// Package webhook provides reliable HTTP webhook delivery with retry logic and circuit breaking.
//
// This package features automatic JSON marshaling, HMAC signature generation, exponential backoff,
// and configurable resilience patterns for production webhook systems. It's designed to handle
// the complexities of webhook delivery including network failures, slow endpoints, and
// temporary service disruptions.
//
// # Features
//
// - Automatic JSON payload marshaling
// - HMAC-SHA256 signature generation
// - Exponential backoff retry strategy
// - Circuit breaker pattern for endpoint protection
// - Configurable timeouts and payload limits
// - Delivery result callbacks for monitoring
// - Production-ready connection pooling
// - Security-focused URL validation
//
// # Basic Usage
//
// Simple webhook delivery:
//
//	sender := webhook.NewSender()
//
//	// Define your event payload
//	event := map[string]any{
//		"type":      "user.created",
//		"user_id":   123,
//		"timestamp": time.Now().Unix(),
//		"data": map[string]any{
//			"email": "user@example.com",
//			"name":  "John Doe",
//		},
//	}
//
//	// Send webhook with default settings
//	err := sender.Send(ctx, "https://api.example.com/webhooks", event)
//	if err != nil {
//		log.Printf("Webhook delivery failed: %v", err)
//	}
//
// Webhook with signature:
//
//	secret := "your-webhook-secret"
//	err := sender.Send(ctx, webhookURL, event,
//		webhook.WithSignature(secret),
//	)
//	if err != nil {
//		log.Printf("Signed webhook failed: %v", err)
//	}
//
// # Configuration Options
//
// Timeout configuration:
//
//	err := sender.Send(ctx, webhookURL, event,
//		webhook.WithTimeout(10*time.Second),  // 10 second timeout
//		webhook.WithMaxRetries(5),            // Try up to 5 times
//	)
//
// Custom headers and limits:
//
//	err := sender.Send(ctx, webhookURL, event,
//		webhook.WithHeader("X-API-Version", "v1"),
//		webhook.WithMaxPayloadSize(1024*1024), // 1MB limit
//	)
//
// # Retry and Backoff
//
// Custom retry strategy:
//
//	backoff := webhook.NewExponentialBackoff(
//		500*time.Millisecond,  // Initial delay
//		30*time.Second,        // Max delay
//		2.0,                   // Multiplier
//		0.1,                   // Jitter
//	)
//
//	err := sender.Send(ctx, webhookURL, event,
//		webhook.WithMaxRetries(3),
//		webhook.WithBackoffStrategy(backoff),
//	)
//
// # Circuit Breaker
//
// Protect endpoints from overload:
//
//	cb := webhook.NewCircuitBreaker(
//		5,               // Failure threshold
//		30*time.Second,  // Reset timeout
//	)
//
//	err := sender.Send(ctx, webhookURL, event,
//		webhook.WithCircuitBreaker(cb),
//	)
//
//	// Circuit breaker will prevent requests if endpoint is failing
//	if errors.Is(err, webhook.ErrCircuitOpen) {
//		log.Println("Circuit breaker is open - endpoint unavailable")
//	}
//
// # Monitoring and Observability
//
// Delivery result callbacks:
//
//	err := sender.Send(ctx, webhookURL, event,
//		webhook.WithDeliveryCallback(func(result webhook.DeliveryResult) {
//			log.Printf("Attempt %d: success=%t, duration=%v, status=%d",
//				result.Attempt, result.Success, result.Duration, result.StatusCode)
//
//			// Send metrics to monitoring system
//			metrics.RecordWebhookDelivery(result)
//
//			if result.Error != nil {
//				log.Printf("Delivery error: %v", result.Error)
//			}
//		}),
//	)
//
// # HTTP Service Integration
//
// Webhook sending endpoint:
//
//	func handleWebhookSend(w http.ResponseWriter, r *http.Request) {
//		var req struct {
//			URL     string         `json:"url"`
//			Event   string         `json:"event"`
//			Payload map[string]any `json:"payload"`
//		}
//
//		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
//			http.Error(w, "Invalid JSON", http.StatusBadRequest)
//			return
//		}
//
//		// Create event payload
//		event := map[string]any{
//			"event":     req.Event,
//			"timestamp": time.Now().Unix(),
//			"data":      req.Payload,
//		}
//
//		// Send webhook with signature
//		sender := webhook.NewSender()
//		err := sender.Send(r.Context(), req.URL, event,
//			webhook.WithSignature(getWebhookSecret(req.URL)),
//			webhook.WithTimeout(15*time.Second),
//			webhook.WithMaxRetries(3),
//		)
//
//		if err != nil {
//			log.Printf("Webhook delivery failed: %v", err)
//			http.Error(w, "Delivery failed", http.StatusInternalServerError)
//			return
//		}
//
//		json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
//	}
//
// # Event System Integration
//
// Event-driven webhook delivery:
//
//	type WebhookService struct {
//		sender *webhook.Sender
//		config WebhookConfig
//	}
//
//	func NewWebhookService(config WebhookConfig) *WebhookService {
//		return &WebhookService{
//			sender: webhook.NewSender(),
//			config: config,
//		}
//	}
//
//	func (s *WebhookService) SendEvent(ctx context.Context, eventType string, data any) error {
//		event := map[string]any{
//			"id":        generateEventID(),
//			"type":      eventType,
//			"timestamp": time.Now().Unix(),
//			"data":      data,
//		}
//
//		var errs []error
//		for _, endpoint := range s.config.GetEndpoints(eventType) {
//			err := s.sender.Send(ctx, endpoint.URL, event,
//				webhook.WithSignature(endpoint.Secret),
//				webhook.WithTimeout(endpoint.Timeout),
//				webhook.WithMaxRetries(endpoint.MaxRetries),
//				webhook.WithHeader("X-Event-Type", eventType),
//			)
//			if err != nil {
//				errs = append(errs, fmt.Errorf("endpoint %s: %w", endpoint.URL, err))
//			}
//		}
//
//		if len(errs) > 0 {
//			return fmt.Errorf("webhook delivery errors: %v", errs)
//		}
//		return nil
//	}
//
// # Signature Verification
//
// Generate signatures for webhook payloads:
//
//	secret := "your-webhook-secret"
//	payload, _ := json.Marshal(event)
//
//	signature, err := webhook.SignPayload(secret, payload)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Use signature headers in manual HTTP requests
//	headers := signature.Headers()
//	req.Header.Set("X-Hub-Signature-256", headers["X-Hub-Signature-256"])
//	req.Header.Set("X-Webhook-Timestamp", headers["X-Webhook-Timestamp"])
//
// Verify incoming webhooks:
//
//	func handleIncomingWebhook(w http.ResponseWriter, r *http.Request) {
//		payload, err := io.ReadAll(r.Body)
//		if err != nil {
//			http.Error(w, "Cannot read body", http.StatusBadRequest)
//			return
//		}
//
//		signature := r.Header.Get("X-Hub-Signature-256")
//		timestamp := r.Header.Get("X-Webhook-Timestamp")
//
//		if !webhook.VerifySignature(webhookSecret, payload, signature, timestamp) {
//			http.Error(w, "Invalid signature", http.StatusUnauthorized)
//			return
//		}
//
//		// Process webhook payload
//		var event map[string]any
//		if err := json.Unmarshal(payload, &event); err != nil {
//			http.Error(w, "Invalid JSON", http.StatusBadRequest)
//			return
//		}
//
//		processEvent(event)
//		w.WriteHeader(http.StatusOK)
//	}
//
// # Queue Integration
//
// Async webhook delivery with queues:
//
//	type WebhookJob struct {
//		URL     string         `json:"url"`
//		Payload map[string]any `json:"payload"`
//		Secret  string         `json:"secret,omitempty"`
//		Retries int            `json:"retries"`
//	}
//
//	func processWebhookJob(ctx context.Context, job WebhookJob) error {
//		sender := webhook.NewSender()
//
//		var opts []webhook.SendOption
//		if job.Secret != "" {
//			opts = append(opts, webhook.WithSignature(job.Secret))
//		}
//		opts = append(opts, webhook.WithMaxRetries(job.Retries))
//
//		err := sender.Send(ctx, job.URL, job.Payload, opts...)
//		if err != nil {
//			// Requeue job or send to dead letter queue
//			return fmt.Errorf("webhook job failed: %w", err)
//		}
//
//		return nil
//	}
//
//	func enqueueWebhook(url string, payload map[string]any, secret string) {
//		job := WebhookJob{
//			URL:     url,
//			Payload: payload,
//			Secret:  secret,
//			Retries: 3,
//		}
//
//		queue.Enqueue("webhooks", job)
//	}
//
// # Error Handling
//
// The package defines specific error types:
//   - ErrInvalidURL: URL is malformed or uses unsupported scheme
//   - ErrInvalidPayload: Payload is empty or too large
//   - ErrTimeout: Request exceeded timeout limit
//   - ErrCircuitOpen: Circuit breaker is protecting the endpoint
//   - ErrPermanentFailure: 4xx HTTP status (won't retry)
//   - ErrTemporaryFailure: Network or 5xx error (will retry)
//   - ErrWebhookDeliveryFailed: All retry attempts exhausted
//
// Comprehensive error handling:
//
//	err := sender.Send(ctx, webhookURL, event,
//		webhook.WithMaxRetries(3),
//		webhook.WithTimeout(10*time.Second),
//	)
//
//	if err != nil {
//		switch {
//		case errors.Is(err, webhook.ErrInvalidURL):
//			log.Printf("Invalid webhook URL: %s", webhookURL)
//			// Fix configuration
//		case errors.Is(err, webhook.ErrTimeout):
//			log.Printf("Webhook timeout: %v", err)
//			// Consider increasing timeout
//		case errors.Is(err, webhook.ErrPermanentFailure):
//			log.Printf("Permanent failure (won't retry): %v", err)
//			// Disable webhook or fix endpoint
//		case errors.Is(err, webhook.ErrWebhookDeliveryFailed):
//			log.Printf("All retries exhausted: %v", err)
//			// Queue for manual review or dead letter
//		default:
//			log.Printf("Unexpected webhook error: %v", err)
//		}
//	}
//
// # Performance Considerations
//
// Connection pooling and reuse:
//   - Default HTTP client uses connection pooling
//   - 100 max idle connections total
//   - 10 max idle connections per host
//   - 90 second idle connection timeout
//
// Memory usage:
//   - ~1-3 KB per webhook delivery (temporary)
//   - JSON marshaling allocates payload copy
//   - Signature calculation uses minimal memory
//
// Concurrent delivery:
//
//	// Send multiple webhooks concurrently
//	var wg sync.WaitGroup
//	for _, endpoint := range endpoints {
//		wg.Add(1)
//		go func(url string) {
//			defer wg.Done()
//			err := sender.Send(ctx, url, event)
//			if err != nil {
//				log.Printf("Failed to deliver to %s: %v", url, err)
//			}
//		}(endpoint)
//	}
//	wg.Wait()
//
// # Security Considerations
//
// URL validation:
//   - Only HTTP and HTTPS schemes allowed
//   - Host validation prevents empty hosts
//   - Helps prevent SSRF attacks
//
// Signature security:
//   - HMAC-SHA256 provides strong authentication
//   - Timestamps prevent replay attacks
//   - Secrets should be cryptographically random
//
// Payload security:
//   - Size limits prevent memory exhaustion
//   - JSON marshaling prevents code injection
//   - HTTPS recommended for production
//
// Best practices:
//
//	// Use environment variables for secrets
//	secret := os.Getenv("WEBHOOK_SECRET")
//	if secret == "" {
//		log.Fatal("WEBHOOK_SECRET required")
//	}
//
//	// Validate URLs before use
//	if !isValidWebhookURL(webhookURL) {
//		return errors.New("invalid webhook URL")
//	}
//
//	// Set reasonable limits
//	err := sender.Send(ctx, webhookURL, event,
//		webhook.WithTimeout(30*time.Second),      // Reasonable timeout
//		webhook.WithMaxPayloadSize(10*1024*1024), // 10MB limit
//		webhook.WithMaxRetries(3),                // Limited retries
//	)
//
// # Testing Support
//
// The package provides utilities for testing:
//
//	func TestWebhookDelivery(t *testing.T) {
//		// Create test server
//		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//			// Verify request
//			assert.Equal(t, "POST", r.Method)
//			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
//
//			// Read and verify payload
//			body, _ := io.ReadAll(r.Body)
//			var event map[string]any
//			json.Unmarshal(body, &event)
//			assert.Equal(t, "test.event", event["type"])
//
//			w.WriteHeader(http.StatusOK)
//		}))
//		defer server.Close()
//
//		// Test webhook delivery
//		sender := webhook.NewSender()
//		event := map[string]any{"type": "test.event"}
//
//		err := sender.Send(context.Background(), server.URL, event)
//		assert.NoError(t, err)
//	}
//
// Mock for testing failures:
//
//	func TestWebhookRetries(t *testing.T) {
//		attempts := 0
//		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//			attempts++
//			if attempts < 3 {
//				w.WriteHeader(http.StatusInternalServerError)
//				return
//			}
//			w.WriteHeader(http.StatusOK)
//		}))
//		defer server.Close()
//
//		sender := webhook.NewSender()
//		err := sender.Send(context.Background(), server.URL, map[string]any{},
//			webhook.WithMaxRetries(3),
//		)
//
//		assert.NoError(t, err)
//		assert.Equal(t, 3, attempts)
//	}
//
// # Production Deployment
//
// Configuration management:
//
//	type WebhookConfig struct {
//		Timeout       time.Duration `json:"timeout"`
//		MaxRetries    int           `json:"max_retries"`
//		MaxPayload    int64         `json:"max_payload_size"`
//		Secret        string        `json:"-"` // Don't serialize secrets
//	}
//
//	func LoadConfig() WebhookConfig {
//		return WebhookConfig{
//			Timeout:    30 * time.Second,
//			MaxRetries: 3,
//			MaxPayload: 10 * 1024 * 1024, // 10MB
//			Secret:     os.Getenv("WEBHOOK_SECRET"),
//		}
//	}
//
// Monitoring integration:
//
//	func sendWebhookWithMetrics(ctx context.Context, url string, event any) error {
//		start := time.Now()
//		defer func() {
//			metrics.RecordWebhookLatency(time.Since(start))
//		}()
//
//		sender := webhook.NewSender()
//		err := sender.Send(ctx, url, event,
//			webhook.WithDeliveryCallback(func(result webhook.DeliveryResult) {
//				metrics.IncWebhookAttempts(result.Success)
//				if !result.Success {
//					metrics.IncWebhookFailures(result.StatusCode)
//				}
//			}),
//		)
//
//		if err != nil {
//			metrics.IncWebhookErrors()
//			return err
//		}
//
//		metrics.IncWebhookSuccess()
//		return nil
//	}
package webhook
