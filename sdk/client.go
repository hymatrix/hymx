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

	"github.com/hymatrix/hymx/node/schema"
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
		},
	}
}

func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
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
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("invalid server response: %d", resp.StatusCode)
	}

	res = &serverSchema.Response{}
	err = json.NewDecoder(resp.Body).Decode(res)
	return res, err
}

func (c *Client) Info() (info schema.Info, err error) {
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
