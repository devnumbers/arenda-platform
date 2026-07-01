package mailer

// Message is a single email message.
type Message struct {
	To       []string
	Subject  string
	TextBody string
	HTMLBody string
}
