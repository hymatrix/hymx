package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"time"

	nodeSchema "github.com/hymatrix/hymx/node/schema"
	serverSchema "github.com/hymatrix/hymx/server/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(hmxURL string) *Client {
	return &Client{
		baseURL: hmxURL,
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 1000,
				IdleConnTimeout:     90 * time.Second,
				MaxConnsPerHost:     2000,
			},
			Timeout: 30 * time.Second,
			// Disable automatic redirect handling to support 308 redirect
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}

// handleRedirect handles 308 redirect responses by trying alternative nodes
func (c *Client) handleRedirect(resp *http.Response, originalReq *http.Request, originalBody []byte) (*http.Response, error) {
	if resp.StatusCode != 308 {
		return resp, nil
	}

	// Read redirect response body
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read redirect response: %w", err)
	}

	// Parse nodes from response
	var redirectResp nodeSchema.RedirectError
	if err := json.Unmarshal(body, &redirectResp); err != nil {
		return nil, fmt.Errorf("failed to parse redirect response: %w", err)
	}

	// Try each node in the redirect response
	for _, node := range redirectResp.Nodes {
		// Create new request with the same method, path, and body as original
		newURL, err := url.Parse(node.URL)
		if err != nil {
			continue
		}
		if newURL.Path == "" || newURL.Path == "/" {
			newURL.Path = originalReq.URL.Path
			newURL.RawQuery = originalReq.URL.RawQuery
		}

		// Create new request with original body
		newReq, err := http.NewRequest(originalReq.Method, newURL.String(), bytes.NewReader(originalBody))
		if err != nil {
			continue // Skip invalid URLs
		}

		// Copy headers from original request
		for key, values := range originalReq.Header {
			for _, value := range values {
				newReq.Header.Add(key, value)
			}
		}

		// Copy other relevant fields from the original request
		newReq.Host = newURL.Host
		newReq.URL = newURL
		newReq.Close = originalReq.Close

		// Execute request to alternative node
		newResp, err := c.httpClient.Do(newReq)
		if err != nil {
			continue // Try next node
		}

		// If successful (2xx status), return this response
		if newResp.StatusCode >= 200 && newResp.StatusCode < 300 {
			return newResp, nil
		}

		// If still 308, try next node
		newResp.Body.Close()
	}

	// If all nodes failed, return original redirect response
	return &http.Response{
		StatusCode: 308,
		Status:     "308 Permanent Redirect",
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     resp.Header,
	}, nil
}

func (c *Client) Send(itemBin []byte) (res *serverSchema.Response, err error) {
	url := fmt.Sprintf("%s/", c.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(itemBin))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Handle 308 redirect
	resp, err = c.handleRedirect(resp, req, itemBin)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// If it's a 308 redirect that couldn't be resolved, return nil response
		if resp.StatusCode == 308 {
			return nil, nil
		}
		return nil, fmt.Errorf("invalid server response: %d", resp.StatusCode)
	}

	res = &serverSchema.Response{}
	err = json.NewDecoder(resp.Body).Decode(res)
	return res, err
}

func (c *Client) Info() (info nodeSchema.Info, err error) {
	url := fmt.Sprintf("%s/info", c.baseURL)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("invalid server response: %d", resp.StatusCode)
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&info)
	return
}

func (c *Client) Callback(targetURL string) (res string, err error) {
	encoded := url.QueryEscape(targetURL)
	fullURL := fmt.Sprintf("%s/callback?url=%s", c.baseURL, encoded)

	resp, err := c.httpClient.Get(fullURL)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("callback request error: status %d", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body failed: %w", err)
	}

	return string(body), nil
}

func (c *Client) GetResult(msgid string) (result vmmSchema.Result, err error) {
	url := fmt.Sprintf("%s/result/%s", c.baseURL, msgid)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("invalid server response: %d", resp.StatusCode)
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	return
}

func (c *Client) GetResults(pid string, limit int64) (results []vmmSchema.Result, err error) {
	url := fmt.Sprintf("%s/results/%s?sort=DESC&limit=%d", c.baseURL, pid, limit)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("invalid server response: %d", resp.StatusCode)
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&results)
	return
}

func (c *Client) GetMessage(msgid string) (item goarSchema.BundleItem, err error) {
	url := fmt.Sprintf("%s/message/%s", c.baseURL, msgid)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("invalid server response: %d", resp.StatusCode)
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&item)
	return
}

func (c *Client) GetMessageByNonce(pid string, nonce int64) (item goarSchema.BundleItem, err error) {
	url := fmt.Sprintf("%s/messageByNonce/%s/%d", c.baseURL, pid, nonce)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("invalid server response: %d", resp.StatusCode)
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&item)
	return
}

func (c *Client) GetAssignByNonce(pid string, nonce int64) (item goarSchema.BundleItem, err error) {
	url := fmt.Sprintf("%s/assignmentByNonce/%s/%d", c.baseURL, pid, nonce)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("invalid server response: %d", resp.StatusCode)
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&item)
	return
}

func (c *Client) GetAssignByMessage(msgid string) (item goarSchema.BundleItem, err error) {
	url := fmt.Sprintf("%s/assignmentByMessage/%s", c.baseURL, msgid)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("invalid server response: %d", resp.StatusCode)
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&item)
	return
}

func (c *Client) BalanceOf(accid string) (amt *big.Int, err error) {
	url := fmt.Sprintf("%s/balanceof/%s", c.baseURL, accid)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("invalid server response: %d", resp.StatusCode)
		return
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	ok := false
	amt = new(big.Int)
	if amt, ok = amt.SetString(string(bodyBytes), 10); !ok {
		err = fmt.Errorf("invalid server response: %s", string(bodyBytes))
		return
	}

	return
}

func (c *Client) StakeOf(accid string) (amt *big.Int, err error) {
	url := fmt.Sprintf("%s/stakeof/%s", c.baseURL, accid)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("invalid server response: %d", resp.StatusCode)
		return
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	ok := false
	amt = new(big.Int)
	if amt, ok = amt.SetString(string(bodyBytes), 10); !ok {
		err = fmt.Errorf("invalid server response: %s", string(bodyBytes))
		return
	}

	return
}
