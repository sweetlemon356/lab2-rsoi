package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sweetlemon356/lab2-rsoi/internal/gateway/application"
)

const maxResponseBytes = 1 << 20

func do(ctx context.Context, client *http.Client, service, method, address, username string, body, result any, allowNotFound bool, statuses ...int) error {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return dependencyError(service, err)
		}
		payload = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, address, payload)
	if err != nil {
		return dependencyError(service, err)
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if username != "" {
		request.Header.Set("X-User-Name", username)
	}
	response, err := client.Do(request)
	if err != nil {
		return dependencyError(service, err)
	}
	defer response.Body.Close()
	if allowNotFound && response.StatusCode == http.StatusNotFound {
		return application.ErrNotFound
	}
	accepted := false
	for _, status := range statuses {
		if response.StatusCode == status {
			accepted = true
			break
		}
	}
	if !accepted {
		return dependencyError(service, fmt.Errorf("unexpected HTTP status %d", response.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return dependencyError(service, err)
	}
	if len(data) > maxResponseBytes {
		return dependencyError(service, errors.New("response exceeds size limit"))
	}
	if result == nil {
		return nil
	}
	if strings.TrimSpace(string(data)) == "null" {
		return dependencyError(service, errors.New("null response"))
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(result); err != nil {
		return dependencyError(service, fmt.Errorf("decode response: %w", err))
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return dependencyError(service, errors.New("response contains trailing data"))
	}
	return nil
}

func dependencyError(service string, err error) error {
	return &application.DependencyError{Service: service, Err: err}
}
