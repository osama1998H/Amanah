package stripe

// Client is a minimal placeholder for interacting with the Stripe API.
type Client struct {
	APIKey string
}

// NewClient returns a new Stripe client with the provided API key.
func NewClient(apiKey string) *Client {
	return &Client{APIKey: apiKey}
}

// Charge is a stub for creating a charge using Stripe.
func (c *Client) Charge(amount int64, currency, source string) error {
	// TODO: implement Stripe charge logic
	return nil
}
