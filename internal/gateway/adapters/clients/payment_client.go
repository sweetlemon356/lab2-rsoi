package clients

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/sweetlemon356/lab2-rsoi/internal/gateway/application"
)

type PaymentHTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewPaymentHTTPClient(baseURL string, client *http.Client) *PaymentHTTPClient {
	return &PaymentHTTPClient{baseURL: strings.TrimRight(baseURL, "/"), httpClient: client}
}

func (c *PaymentHTTPClient) Get(ctx context.Context, paymentUID string) (application.PaymentInfo, error) {
	var response paymentDTO
	if err := do(ctx, c.httpClient, "payment", http.MethodGet, c.baseURL+"/internal/v1/payments/"+url.PathEscape(paymentUID), "", nil, &response, false, 200); err != nil {
		return application.PaymentInfo{}, err
	}
	return paymentModel(response)
}

func (c *PaymentHTTPClient) Create(ctx context.Context, command application.CreatePaymentCommand) (application.PaymentInfo, error) {
	body := struct {
		Price int64 `json:"price"`
	}{command.Price}
	var response paymentDTO
	if err := do(ctx, c.httpClient, "payment", http.MethodPut, c.baseURL+"/internal/v1/payments/"+url.PathEscape(command.PaymentUID), "", body, &response, false, 200, 201); err != nil {
		return application.PaymentInfo{}, err
	}
	return paymentModel(response)
}

func (c *PaymentHTTPClient) Cancel(ctx context.Context, paymentUID string) error {
	return do(ctx, c.httpClient, "payment", http.MethodPut, c.baseURL+"/internal/v1/payments/"+url.PathEscape(paymentUID)+"/cancellation", "", nil, nil, false, 200, 204)
}

func paymentModel(response paymentDTO) (application.PaymentInfo, error) {
	payment, err := response.model()
	if err != nil {
		return application.PaymentInfo{}, dependencyError("payment", err)
	}
	return payment, nil
}
