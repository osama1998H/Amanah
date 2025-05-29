package paypal

// Client is a minimal placeholder for interacting with the PayPal API.
type Client struct {
	ClientID     string
	ClientSecret string
}

// NewClient returns a new PayPal client using the provided credentials.
func NewClient(id, secret string) *Client {
	return &Client{ClientID: id, ClientSecret: secret}
}

// Charge is a stub for creating a payment using PayPal.
func (c *Client) Charge(amount float64, currency, source string) error {
	// TODO: implement PayPal payment logic
	return nil
}
