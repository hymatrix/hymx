package chainkit

import goarSchema "github.com/permadao/goar/schema"

func (c *Chainkit) download(parentTxID string, itemsIds []string) (items []*goarSchema.BundleItem, err error) {
	return c.operator.Download(parentTxID, itemsIds)
}
