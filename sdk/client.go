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
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
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

func (c *Client) buildURL(path string) (string, error) {
	baseURL, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	relativePath, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(relativePath).String(), nil
}

// handleRedirect handles 308 redirect responses by trying alternative nodes
func (c *Client) handleRedirect(resp *http.Response, originalReq *http.Request, originalBody []byte) (*http.Response, string, error) {
	if resp.StatusCode != 308 {
		return resp, "", nil
	}

	// Read redirect response body
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, "", fmt.Errorf("failed to read redirect response: %w", err)
	}

	// Parse nodes from response
	// Try to parse as RedirectError first (standard format)
	var nodes []registrySchema.Node
	if err := json.Unmarshal(body, &nodes); err != nil {
		log.Error("unmarshal redirect response failed", "body", string(body), "err", err)
		return nil, "", fmt.Errorf("failed to parse redirect response: %w", err)
	}

	// Try each node in the redirect response
	for _, node := range nodes {
		fmt.Println("get node url", node.URL)
		// Create new request with the same method, path, and body as original
		newURL, err := url.Parse(node.URL)
		if err != nil {
			log.Error("parse node url failed", "url", node.URL, "err", err)
			continue
		}
		if newURL.Path == "" || newURL.Path == "/" {
			newURL.Path = originalReq.URL.Path
			newURL.RawQuery = originalReq.URL.RawQuery
		}

		// Create new request with original body
		newReq, err := http.NewRequest(originalReq.Method, newURL.String(), bytes.NewReader(originalBody))
		if err != nil {
			log.Error("create new request failed", "url", newURL.String(), "err", err)
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
		log.Debug("send redirect msg", "new url", newURL, "ori url", originalReq.URL.String())
		newResp, err := c.httpClient.Do(newReq)
		if err != nil {
			log.Error("send redirect msg failed", "new url", newURL, "ori url", originalReq.URL.String(), "err", err)
			continue // Try next node
		}

		// If successful (2xx status), return this response with the redirected URL
		if newResp.StatusCode >= 200 && newResp.StatusCode < 300 {
			log.Debug("send redirect msg success", "new url", newURL, "ori url", originalReq.URL.String(), "statusCode", newResp.StatusCode)
			return newResp, newURL.String(), nil
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
	}, "", nil
}

func (c *Client) Send(itemBin []byte) (res *serverSchema.Response, redirectedURL string, err error) {
	endpointURL, err := c.buildURL("/")
	if err != nil {
		return nil, "", err
	}

	req, err := http.NewRequest("POST", endpointURL, bytes.NewBuffer(itemBin))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}

	// Handle 308 redirect
	resp, redirectedURL, err = c.handleRedirect(resp, req, itemBin)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("invalid server response: %d", resp.StatusCode)
	}

	res = &serverSchema.Response{}
	err = json.NewDecoder(resp.Body).Decode(res)
	return res, redirectedURL, err
}

func (c *Client) Info() (info nodeSchema.Info, err error) {
	url, err := c.buildURL("/info")
	if err != nil {
		return
	}
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
	fullURL, err := c.buildURL("/callback?url=" + encoded)
	if err != nil {
		return
	}

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

func (c *Client) GetResult(pid, msgid string) (result vmmSchema.Result, err error) {
	url, err := c.buildURL(fmt.Sprintf("/result/%s/%s", pid, msgid))
	if err != nil {
		return
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Handle redirect response
	resp, _, err = c.handleRedirect(resp, req, nil)
	if err != nil {
		return result, err
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
	path := fmt.Sprintf("/results/%s?sort=DESC&limit=%d", pid, limit)
	url, err := c.buildURL(path)
	if err != nil {
		return
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}

	resp, err := c.httpClient.Do(req)
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
	url, err := c.buildURL("/message/" + msgid)
	if err != nil {
		return
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}

	resp, err := c.httpClient.Do(req)
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
	path := fmt.Sprintf("/messageByNonce/%s/%d", pid, nonce)
	url, err := c.buildURL(path)
	if err != nil {
		return
	}
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
	path := fmt.Sprintf("/assignmentByNonce/%s/%d", pid, nonce)
	url, err := c.buildURL(path)
	if err != nil {
		return
	}
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
	url, err := c.buildURL("/assignmentByMessage/" + msgid)
	if err != nil {
		return
	}
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
	url, err := c.buildURL("/balanceof/" + accid)
	if err != nil {
		return
	}
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
	url, err := c.buildURL("/stakeof/" + accid)
	if err != nil {
		return
	}
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
