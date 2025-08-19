package smtp

// Config holds SMTP server configuration.
// All fields are required for runtime operation to ensure explicit configuration
// and avoid silent failures in production.
type Config struct {
	Host         string `env:"SMTP_HOST,required"`
	Port         int    `env:"SMTP_PORT" envDefault:"587"`
	Username     string `env:"SMTP_USERNAME,required"`
	Password     string `env:"SMTP_PASSWORD,required"`
	TLSMode      string `env:"SMTP_TLS_MODE" envDefault:"starttls"` // starttls, tls, or plain
	SenderEmail  string `env:"SENDER_EMAIL,required"`
	SupportEmail string `env:"SUPPORT_EMAIL,required"`
}
