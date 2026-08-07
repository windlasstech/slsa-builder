package npmprofile

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	remoteRequestTimeout = 2 * time.Minute
	remoteRequestRetries = 3
)

type boundedResponse struct {
	status int
	body   []byte
}

func hardenedHTTPClient(injected *http.Client) (*http.Client, error) {
	var sourceTransport *http.Transport
	if injected == nil {
		baseTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, errors.New("default HTTP transport has an unsupported type")
		}
		sourceTransport = baseTransport
		injected = &http.Client{}
	} else if injected.Transport == nil {
		baseTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, errors.New("default HTTP transport has an unsupported type")
		}
		sourceTransport = baseTransport
	} else {
		var ok bool
		sourceTransport, ok = injected.Transport.(*http.Transport)
		if !ok {
			return nil, errors.New("custom HTTP transport has an unsupported type")
		}
	}
	transport := sourceTransport.Clone()
	//nolint:staticcheck // Both TLS dial hooks bypass Transport TLS policy and must be rejected.
	if transport.DialTLS != nil || transport.DialTLSContext != nil {
		return nil, errors.New("custom TLS dialers are prohibited")
	}
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
		if transport.TLSClientConfig.InsecureSkipVerify {
			return nil, errors.New("TLS certificate verification cannot be disabled")
		}
		if transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
			transport.TLSClientConfig.MinVersion = tls.VersionTLS12
		}
	}
	client := *injected
	client.Transport = transport
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errors.New("authenticated endpoint redirects are prohibited")
	}
	return &client, nil
}

func performBoundedRequest(
	ctx context.Context,
	client *http.Client,
	method string,
	endpoint *url.URL,
	headers http.Header,
	maximum int64,
) (boundedResponse, error) {
	requestContext, cancel := context.WithTimeout(ctx, remoteRequestTimeout)
	defer cancel()

	var lastErr error
	for attempt := 0; attempt < remoteRequestRetries; attempt++ {
		request, err := http.NewRequestWithContext(requestContext, method, endpoint.String(), nil)
		if err != nil {
			return boundedResponse{}, errors.New("construct authenticated HTTP request")
		}
		request.Header = headers.Clone()
		if err := rejectCredentialBearingProxy(client, request); err != nil {
			return boundedResponse{}, err
		}
		response, requestErr := client.Do(request)
		if requestErr != nil {
			lastErr = errors.New("authenticated HTTP request failed")
		} else {
			body, readErr := io.ReadAll(io.LimitReader(response.Body, maximum+1))
			closeErr := response.Body.Close()
			if readErr != nil || closeErr != nil {
				lastErr = errors.New("authenticated HTTP response could not be read")
			} else if int64(len(body)) > maximum {
				return boundedResponse{}, errors.New("authenticated HTTP response exceeds size limit")
			} else {
				result := boundedResponse{status: response.StatusCode, body: body}
				if !retryableHTTPStatus(response.StatusCode) || attempt == remoteRequestRetries-1 {
					return result, nil
				}
				lastErr = fmt.Errorf("authenticated endpoint returned HTTP %d", response.StatusCode)
			}
		}
		if attempt == remoteRequestRetries-1 {
			break
		}
		delay := time.NewTimer(time.Duration(attempt+1) * 100 * time.Millisecond)
		select {
		case <-requestContext.Done():
			delay.Stop()
			return boundedResponse{}, errors.New("authenticated HTTP request timed out")
		case <-delay.C:
		}
	}
	return boundedResponse{}, lastErr
}

func rejectCredentialBearingProxy(client *http.Client, request *http.Request) error {
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	httpTransport, ok := transport.(*http.Transport)
	if !ok || httpTransport.Proxy == nil {
		return nil
	}
	proxyURL, err := httpTransport.Proxy(request)
	if err != nil {
		return errors.New("resolve HTTPS proxy")
	}
	if proxyURL != nil && proxyURL.User != nil {
		return errors.New("credential-bearing HTTPS proxies are prohibited")
	}
	return nil
}

func retryableHTTPStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}
