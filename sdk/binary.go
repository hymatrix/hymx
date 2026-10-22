package sdk

import (
	"fmt"
	"io"
	"mime"
	"net/http"

	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
)

func (c *Client) GetMessageBin(msgid string) (item goarSchema.BundleItem, err error) {
	item, err = c.getItemBinary("/messageBin/" + msgid)
	if err == nil && item.Id != msgid {
		return goarSchema.BundleItem{}, nodeSchema.ErrUnauthorizedNode
	}
	return
}

func (c *Client) GetMessageBinByNonce(pid string, nonce int64) (goarSchema.BundleItem, error) {
	return c.getItemBinary(fmt.Sprintf("/messageBinByNonce/%s/%d", pid, nonce))
}

func (c *Client) GetAssignBinByNonce(pid string, nonce int64) (goarSchema.BundleItem, error) {
	return c.getItemBinary(fmt.Sprintf("/assignmentBinByNonce/%s/%d", pid, nonce))
}

func (c *Client) GetAssignBinByMessage(msgid string) (item goarSchema.BundleItem, err error) {
	item, err = c.getItemBinary("/assignmentBinByMessage/" + msgid)
	if err == nil && (utils.GetType(item) != schema.TypeAssignment || utils.GetTagsValue("Message", item.Tags) != msgid) {
		return goarSchema.BundleItem{}, nodeSchema.ErrUnauthorizedNode
	}
	return
}

// getItemBinary never falls back to JSON: all returned fields come from signed bytes.
func (c *Client) getItemBinary(path string) (item goarSchema.BundleItem, err error) {
	url, err := c.buildURL(path)
	if err != nil {
		return
	}
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return item, fmt.Errorf("invalid server response: %d", resp.StatusCode)
	}
	contentType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || contentType != "application/octet-stream" {
		return item, fmt.Errorf("invalid bundle item content type: %q", resp.Header.Get("Content-Type"))
	}
	binary, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	return decodeItemBinary(binary)
}

// Decode and verify at the network boundary; goar can panic on truncated headers.
func decodeItemBinary(binary []byte) (item goarSchema.BundleItem, err error) {
	defer func() {
		if r := recover(); r != nil {
			item = goarSchema.BundleItem{}
			err = fmt.Errorf("invalid bundle item: %v", r)
		}
	}()
	item, err = goarUtils.DecodeBundleItem(binary)
	if err == nil {
		err = goarUtils.VerifyBundleItem(item)
	}
	if err != nil {
		item = goarSchema.BundleItem{}
	}
	return
}
